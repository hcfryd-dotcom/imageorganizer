package store

import (
	"context"
	"encoding/json"

	"github.com/lkarlslund/imageorganizer/internal/analysis"

	bolt "go.etcd.io/bbolt"
)

// Faces are detected during import (while the photo is decoded anyway) and
// stored with their identity embedding and apparent age. People are not
// defined up front: naming a cluster of faces creates a person.

var (
	bucketFaces  = []byte("faces")
	bucketPeople = []byte("people")
)

// FaceData is one detected face as produced by analysis.
type FaceData struct {
	Box       [4]float32    // x1, y1, x2, y2, 0..1 of the upright photo
	Landmarks [5][2]float32 // 0..1
	Score     float32
	Embedding []float32
	CropBox   [4]float32 // the face inside its stored crop, 0..1
	// Age and AgeVersion when the age was estimated during analysis
	// (AgeVersion 0: leave it to the age stage).
	Age        float32
	AgeVersion int
}

type faceRec struct {
	ID        int64         `json:"id"`
	ImageID   int64         `json:"image"`
	Index     int           `json:"i"` // position in the image's face list; names the crop file
	Box       [4]float32    `json:"box"`
	Landmarks [5][2]float32 `json:"kps"`
	Score     float32       `json:"score"`
	CropBox   [4]float32    `json:"cbox"`
	// Age is the apparent age in years, estimated by a separate stage from
	// the stored crop; AgeVersion is the analysis.Age* method that produced
	// it (0: not yet; Age -1: too small to judge).
	Age        float32 `json:"age"`
	AgeVersion int     `json:"agev,omitempty"`
	LegacyAged bool    `json:"aged,omitempty"` // pre-versioning; migrated at load
	Embedding  []byte  `json:"emb"`            // little-endian float32
	// ClusterID is the face's match group; the group's person (if any) is
	// who this face is. 0 until the people stage has placed it.
	ClusterID int64 `json:"cluster,omitempty"`
	// NotFace is the user saying this detection is not a person (a cat, a
	// pattern); it is kept so re-analysis doesn't bring it back, but is
	// never grouped or shown.
	NotFace bool `json:"notface,omitempty"`
	// Tagged is the person the user tagged this very face as (in a photo),
	// as opposed to naming its whole group. It survives regrouping.
	Tagged int64 `json:"tag,omitempty"`

	emb []float32 // decoded Embedding, in memory only
	bg  bool      // a background face (isBackground) when added to its group
}

type personRec struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	BirthYear int    `json:"birth_year,omitempty"` // known, or 0
	CoverFace int64  `json:"cover_face,omitempty"` // the face the user picked as their picture, or 0
	CreatedAt int64  `json:"created"`
}

func (s *Store) loadFaces(tx *bolt.Tx) error {
	for _, b := range [][]byte{bucketFaces, bucketPeople} {
		if _, err := tx.CreateBucketIfNotExists(b); err != nil {
			return err
		}
	}
	if err := tx.Bucket(bucketPeople).ForEach(func(_, v []byte) error {
		var p personRec
		if err := json.Unmarshal(v, &p); err != nil {
			return err
		}
		s.people[p.ID] = &p
		return nil
	}); err != nil {
		return err
	}
	if err := tx.Bucket(bucketFaces).ForEach(func(_, v []byte) error {
		var f faceRec
		if err := json.Unmarshal(v, &f); err != nil {
			return err
		}
		f.emb = decodeVec(f.Embedding)
		if f.LegacyAged && f.AgeVersion == 0 {
			f.AgeVersion = 1
		}
		s.indexFace(&f)
		return nil
	}); err != nil {
		return err
	}
	s.computeJunkDir()
	s.rebuildClusters()
	if err := s.loadGroups(tx); err != nil {
		return err
	}
	if err := s.regroupIfNeeded(tx); err != nil {
		return err
	}
	return s.recheckAuto(tx)
}

func (s *Store) indexFace(f *faceRec) {
	s.faces[f.ID] = f
	s.facesByImage[f.ImageID] = append(s.facesByImage[f.ImageID], f.ID)
}

func (s *Store) unindexFace(f *faceRec) {
	if c := s.clusters[f.ClusterID]; c != nil {
		s.clusterRemove(c, f)
	}
	delete(s.faces, f.ID)
	ids := s.facesByImage[f.ImageID]
	for i, id := range ids {
		if id == f.ID {
			ids = append(ids[:i:i], ids[i+1:]...)
			break
		}
	}
	if len(ids) == 0 {
		delete(s.facesByImage, f.ImageID)
	} else {
		s.facesByImage[f.ImageID] = ids
	}
}

// replaceFaces swaps an image's faces for newly detected ones inside tx,
// keeping the match group of new faces that overlap old ones (an edited
// photo keeps who is in it). It returns a function that applies the
// change to the in-memory indexes after the transaction commits.
func (s *Store) replaceFaces(tx *bolt.Tx, imageID int64, data []FaceData) (func(), error) {
	var old []*faceRec
	for _, id := range s.facesByImage[imageID] {
		old = append(old, s.faces[id])
	}
	for _, f := range old {
		if err := tx.Bucket(bucketFaces).Delete(key(f.ID)); err != nil {
			return nil, err
		}
	}
	var fresh []*faceRec
	for i, d := range data {
		id, err := nextID(tx, bucketFaces)
		if err != nil {
			return nil, err
		}
		f := &faceRec{ID: int64(id), ImageID: imageID, Index: i, Box: d.Box, Landmarks: d.Landmarks,
			Score: d.Score, CropBox: d.CropBox, Age: -1, Embedding: encodeVec(d.Embedding), emb: d.Embedding}
		if d.AgeVersion > 0 {
			f.Age, f.AgeVersion = d.Age, d.AgeVersion
		}
		for _, o := range old {
			if boxIoU(o.Box, d.Box) > 0.5 {
				f.ClusterID, f.NotFace = o.ClusterID, o.NotFace
				break
			}
		}
		if err := put(tx, bucketFaces, f.ID, f); err != nil {
			return nil, err
		}
		fresh = append(fresh, f)
	}
	return func() {
		for _, f := range old {
			s.unindexFace(f)
		}
		for _, f := range fresh {
			s.indexFace(f)
			if f.ClusterID != 0 {
				s.clusterAdd(s.clusterOf(f.ClusterID), f)
			}
		}
	}, nil
}

// dropFaces deletes all of an image's faces inside tx; the returned func
// updates memory after commit.
func (s *Store) dropFaces(tx *bolt.Tx, imageID int64) (func(), error) {
	var old []*faceRec
	for _, id := range s.facesByImage[imageID] {
		old = append(old, s.faces[id])
	}
	for _, f := range old {
		if err := tx.Bucket(bucketFaces).Delete(key(f.ID)); err != nil {
			return nil, err
		}
	}
	return func() {
		for _, f := range old {
			s.unindexFace(f)
		}
	}, nil
}

func boxIoU(a, b [4]float32) float32 {
	x1, y1 := max(a[0], b[0]), max(a[1], b[1])
	x2, y2 := min(a[2], b[2]), min(a[3], b[3])
	inter := max(0, x2-x1) * max(0, y2-y1)
	u := (a[2]-a[0])*(a[3]-a[1]) + (b[2]-b[0])*(b[3]-b[1]) - inter
	if u <= 0 {
		return 0
	}
	return inter / u
}

// NeedsOriginal reports whether the image with this hash exists but has
// analysis that needs the original file out of date, so import must decode
// it rather than just link it.
func (s *Store) NeedsOriginal(hash string, faces bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byHash[hash]
	return ok && s.images[id].Analysis.NeedsOriginal(faces)
}

// RequeueForAnalysis marks one present file of every image whose
// original-dependent analysis is out of date as pending, so the import stage
// reads it once more. It returns how many images were queued.
func (s *Store) RequeueForAnalysis(ctx context.Context, faces bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var files []*fileRec
	for id, im := range s.images {
		if !im.Analysis.NeedsOriginal(faces) {
			continue
		}
		if f := s.primaryFile(id); f != nil && !f.Pending {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return 0, nil
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range files {
			c := *f
			c.Pending = true
			if err := put(tx, bucketFiles, c.ID, &c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, f := range files {
		f.Pending = true
	}
	return len(files), nil
}

// FaceStats summarises face analysis progress.
type FaceStats struct {
	Analyzed int `json:"analyzed"` // images analysed with the current version
	Images   int `json:"images"`   // images that should be
	Faces    int `json:"faces"`
	Named    int `json:"named"` // faces assigned to a person
	People   int `json:"people"`
}

func (s *Store) FaceStats() FaceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st FaceStats
	for id, im := range s.images {
		if im.HasThumb && s.present(id) {
			st.Images++
			if !im.Analysis.Behind(analysis.CFaces, analysis.Faces) {
				st.Analyzed++
			}
		}
	}
	st.Faces = len(s.faces)
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 {
			st.Named++
		}
	}
	st.People = len(s.people)
	return st
}

// AgeJob is a face waiting for age estimation.
type AgeJob struct {
	FaceID  int64
	Hash    string // content hash of the image, names the crop
	Index   int
	CropBox [4]float32
}

// NeedAge returns up to n faces whose age has not been estimated yet.
func (s *Store) NeedAge(ctx context.Context, n int, skip map[int64]bool) []AgeJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []AgeJob
	for _, f := range s.faces {
		if f.AgeVersion > 0 || skip[f.ID] {
			continue
		}
		im, ok := s.images[f.ImageID]
		if !ok {
			continue
		}
		out = append(out, AgeJob{FaceID: f.ID, Hash: im.Hash, Index: f.Index, CropBox: f.CropBox})
		if len(out) == n {
			break
		}
	}
	return out
}

// SetAge records an age estimate (age < 0: could not be judged).
func (s *Store) SetAge(ctx context.Context, faceID int64, age float32) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.faces[faceID]
	if !ok {
		return nil // replaced meanwhile
	}
	c := *f
	c.Age, c.AgeVersion = age, analysis.AgeFromCrop
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, c.ID, &c) }); err != nil {
		return err
	}
	// The face's group tracks implied birth years; swap its contribution.
	cl := s.clusters[f.ClusterID]
	if cl != nil {
		s.clusterRemove(cl, f)
	}
	f.Age, f.AgeVersion = c.Age, c.AgeVersion
	if cl != nil {
		s.clusterAdd(cl, f)
	}
	return nil
}

// AgeStats reports age estimation progress.
func (s *Store) AgeStats() (done, total int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, f := range s.faces {
		total++
		if f.AgeVersion > 0 {
			done++
		}
	}
	return done, total
}
