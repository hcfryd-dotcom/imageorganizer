package store

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
	bolt "go.etcd.io/bbolt"
)

// nsfwMin is the classifier's score from which a photo counts as not safe
// for work.
const nsfwMin = 0.8

// blurMax is the sharpness (media.Sharpness) below which a photo looks
// blurry.
const blurMax = 50

// isNsfw reports whether a photo is not safe for work: the user's say, or
// the classifier's.
func (s *Store) isNsfw(im *imageRec) bool {
	if im.UserNsfw != nil {
		return *im.UserNsfw
	}
	return im.Nsfw >= nsfwMin
}

// isBlurry reports whether a photo looks blurry, once measured.
func isBlurry(im *imageRec) bool {
	return !im.Analysis.Behind(analysis.CBlur, analysis.Blur) && im.Sharp >= 0 && im.Sharp < blurMax
}

// lookBehind reports whether the look stage still has to look at an image:
// its sharpness, or (with a classifier) its NSFW score.
func (s *Store) lookBehind(im *imageRec) bool {
	return im.Analysis.Behind(analysis.CBlur, analysis.Blur) || s.NsfwModel && im.Analysis.Behind(analysis.CNsfw, analysis.Nsfw)
}

// NeedLook returns up to n present images with a thumbnail the look stage
// hasn't seen yet, newest first.
func (s *Store) NeedLook(ctx context.Context, n int, skip map[int64]bool) []VectorJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*imageRec
	for id, im := range s.images {
		if im.HasThumb && s.lookBehind(im) && !skip[id] && s.present(id) {
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

// Look is what the look stage found in a thumbnail: its sharpness, and its
// NSFW score (-1: couldn't judge).
type Look struct {
	Sharp, Nsfw float32
}

// SetLook records what the look stage found, unless the image changed
// meanwhile. Without a classifier the NSFW score is left as it was.
func (s *Store) SetLook(ctx context.Context, id int64, hash string, l Look) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	im, ok := s.images[id]
	if !ok || im.Hash != hash {
		return nil
	}
	c := *im
	c.Analysis = maps.Clone(im.Analysis)
	if c.Analysis == nil {
		c.Analysis = analysis.Versions{}
	}
	c.Sharp = l.Sharp
	c.Analysis[analysis.CBlur] = analysis.Blur
	if s.NsfwModel {
		c.Nsfw = l.Nsfw
		c.Analysis[analysis.CNsfw] = analysis.Nsfw
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketImages, id, &c) }); err != nil {
		return err
	}
	*im = c
	return nil
}

// LookStats reports the look stage's progress, and how many photos count
// as NSFW and as blurry.
func (s *Store) LookStats() (done, total, nsfw, blurry int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for id, im := range s.images {
		if !im.HasThumb || !s.present(id) {
			continue
		}
		total++
		if !s.lookBehind(im) {
			done++
		}
		if s.isNsfw(im) {
			nsfw++
		}
		if isBlurry(im) {
			blurry++
		}
	}
	return done, total, nsfw, blurry
}
