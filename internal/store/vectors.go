package store

import (
	"cmp"
	"context"
	"encoding/binary"
	"maps"
	"math"
	"slices"
	"strings"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
	bolt "go.etcd.io/bbolt"
)

// Image embeddings live in their own bucket, tagged with the model that
// produced them; a different model invalidates them all. They are kept in
// memory for brute-force cosine search, which for a personal library is a
// few milliseconds.

var (
	bucketVectors = []byte("vectors")
	bucketMeta    = []byte("meta")
	keyVecModel   = []byte("vector_model")
)

func encodeVec(v []float32) []byte {
	b := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[4*i:], math.Float32bits(x))
	}
	return b
}

func decodeVec(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
	}
	return v
}

func (s *Store) loadVectors(tx *bolt.Tx) error {
	for _, b := range [][]byte{bucketVectors, bucketMeta} {
		if _, err := tx.CreateBucketIfNotExists(b); err != nil {
			return err
		}
	}
	s.vecModel = string(tx.Bucket(bucketMeta).Get(keyVecModel))
	return tx.Bucket(bucketVectors).ForEach(func(k, v []byte) error {
		id := int64(binary.BigEndian.Uint64(k))
		s.vectors[id] = decodeVec(v)
		// Vectors from before per-component versions are version 1.
		if im := s.images[id]; im != nil && im.Analysis[analysis.CEmbed] == 0 {
			im.Analysis[analysis.CEmbed] = 1
		}
		return nil
	})
}

// SetVectorModel records which model produces vectors. If it differs from
// the stored one, all existing vectors are discarded so images re-index.
func (s *Store) SetVectorModel(ctx context.Context, model string) (reset bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if model == s.vecModel {
		return false, nil
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketVectors); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		if _, err := tx.CreateBucket(bucketVectors); err != nil {
			return err
		}
		return tx.Bucket(bucketMeta).Put(keyVecModel, []byte(model))
	})
	if err != nil {
		return false, err
	}
	reset = s.vecModel != "" || len(s.vectors) > 0
	s.vecModel = model
	clear(s.vectors)
	return reset, nil
}

// SetVector stores an image's embedding. The hash guards against the image
// having changed while it was being embedded.
func (s *Store) SetVector(ctx context.Context, id int64, hash string, v []float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	im, ok := s.images[id]
	if !ok || im.Hash != hash {
		return nil // deleted or changed meanwhile; it will be queued again
	}
	c := *im
	c.Analysis = maps.Clone(im.Analysis)
	if c.Analysis == nil {
		c.Analysis = analysis.Versions{}
	}
	c.Analysis[analysis.CEmbed] = analysis.Embed
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketVectors).Put(key(id), encodeVec(v)); err != nil {
			return err
		}
		return put(tx, bucketImages, id, &c)
	}); err != nil {
		return err
	}
	s.vectors[id] = v
	im.Analysis = c.Analysis
	return nil
}

// dropVector forgets an image's embedding; call with s.mu held, inside or
// after a successful transaction.
func (s *Store) dropVector(tx *bolt.Tx, id int64) error {
	delete(s.vectors, id)
	return tx.Bucket(bucketVectors).Delete(key(id))
}

// VectorJob is an image waiting to be embedded.
type VectorJob struct {
	ID   int64
	Hash string
}

// NeedVectors returns up to n present images with a thumbnail but no
// embedding, newest first so recent photos become searchable soonest.
func (s *Store) NeedVectors(ctx context.Context, n int, skip map[int64]bool) []VectorJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*imageRec
	for id, im := range s.images {
		_, has := s.vectors[id]
		if (!has || im.Analysis.Behind(analysis.CEmbed, analysis.Embed)) && im.HasThumb && !skip[id] && s.present(id) {
			list = append(list, im)
		}
	}
	slices.SortFunc(list, func(a, b *imageRec) int { return cmp.Compare(takenOr(b, 0), takenOr(a, 0)) })
	out := make([]VectorJob, 0, min(n, len(list)))
	for _, im := range list[:min(n, len(list))] {
		out = append(out, VectorJob{im.ID, im.Hash})
	}
	return out
}

// VectorStats reports indexing progress.
func (s *Store) VectorStats() (done, total int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, im := range s.images {
		if im.HasThumb && s.present(id) {
			total++
			if _, ok := s.vectors[id]; ok {
				done++
			}
		}
	}
	return done, total
}

// Search ranks images matching f (ignoring f.Text) by similarity to the
// query vector. Images whose title, notes or path contain the literal text
// get a boost, so searching for a filename still works. Every indexed image
// is returned, most relevant first; the UI pages through them as you scroll.
func (s *Store) Search(ctx context.Context, q []float32, f Filter) ([]Image, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	text := strings.ToLower(strings.TrimSpace(f.Text))
	words := strings.Fields(text)
	f.Text = ""
	type hit struct {
		im    *imageRec
		score float32
	}
	var hits []hit
	for _, im := range s.selectImages(f) {
		// Photos not embedded yet still match, after every embedded one.
		var score float32 = -2
		if v, ok := s.vectors[im.ID]; ok {
			score = dot(q, v)
		}
		if len(words) > 0 && s.match(im, &Filter{Text: text}, words, nil) {
			score += 0.25
		}
		hits = append(hits, hit{im, score})
	}
	// Ties broken by id so pages stay stable while scrolling.
	slices.SortFunc(hits, func(a, b hit) int { return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(b.im.ID, a.im.ID)) })
	// In another order (newest first, say): the best matches, so ordered.
	// Every photo has a score, so all of them by date would bury the
	// relevant ones.
	if f.Sort != "" && f.Sort != SortRelevance {
		hits = hits[:min(len(hits), searchTop)]
		list := make([]*imageRec, len(hits))
		score := make(map[*imageRec]float32, len(hits))
		for i, h := range hits {
			list[i], score[h.im] = h.im, h.score
		}
		sortImages(list, f.Sort)
		for i, im := range list {
			hits[i] = hit{im, score[im]}
		}
	}
	if f.Limit <= 0 || f.Limit > 2000 {
		f.Limit = 200
	}
	start := min(max(f.Offset, 0), len(hits))
	end := min(start+f.Limit, len(hits))
	out := make([]Image, 0, end-start)
	for _, h := range hits[start:end] {
		v := s.view(h.im)
		v.Score = h.score
		out = append(out, v)
	}
	return out, len(hits), nil
}

// SortRelevance orders a search by how well photos match it, best first
// (the default with search text).
const SortRelevance = "relevance"

// searchTop is how many of the best matches a search sorted another way
// keeps.
const searchTop = 300

func dot(a, b []float32) float32 {
	n := min(len(a), len(b))
	a, b = a[:n], b[:n]
	// Four sums at a time: the products don't wait on each other.
	var s0, s1, s2, s3 float32
	i := 0
	for ; i+4 <= n; i += 4 {
		s0 += a[i] * b[i]
		s1 += a[i+1] * b[i+1]
		s2 += a[i+2] * b[i+2]
		s3 += a[i+3] * b[i+3]
	}
	for ; i < n; i++ {
		s0 += a[i] * b[i]
	}
	return s0 + s1 + s2 + s3
}
