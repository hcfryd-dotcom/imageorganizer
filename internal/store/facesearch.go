package store

import (
	"cmp"
	"context"
	"slices"
)

const (
	similarMin   = 0.30 // a library face this alike to the searched one is shown
	similarFaces = 12   // faces shown per group, most alike first
)

// SimilarGroup is a group with faces like a searched face, for "find this
// face": who the group is (if anyone), and its most alike faces.
type SimilarGroup struct {
	Group  int64      `json:"group"`
	Person int64      `json:"person,omitempty"`
	Name   string     `json:"name,omitempty"`
	Auto   bool       `json:"auto,omitempty"`
	Hidden bool       `json:"hidden,omitempty"` // strangers
	Size   int        `json:"size"`
	Score  float32    `json:"score"` // its most alike face
	Alike  int        `json:"alike"` // its faces at least similarMin alike
	Faces  []FaceView `json:"faces"` // the most alike, with Score
}

// SimilarFaces finds the faces in the library most like emb (a face from
// outside it, say an uploaded photo), grouped as the library groups them,
// best group first. A face's age, when known (age >= 0), leaves out
// groups clearly of another age.
func (s *Store) SimilarFaces(ctx context.Context, emb []float32, age float32, limit int) []SimilarGroup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u := unitVec(emb)
	fs := make([]*faceRec, 0, len(s.faces))
	for _, f := range s.faces {
		if !f.NotFace && f.ClusterID != 0 && len(f.emb) == len(u) {
			fs = append(fs, f)
		}
	}
	sims := make([]float32, len(fs))
	parallel(len(fs), func(i int) { sims[i] = dot(u, unitVec(fs[i].emb)) })
	type hit struct {
		f *faceRec
		s float32
	}
	byGroup := map[int64][]hit{}
	for i, f := range fs {
		if sims[i] >= similarMin {
			byGroup[f.ClusterID] = append(byGroup[f.ClusterID], hit{f, sims[i]})
		}
	}
	var out []SimilarGroup
	for gid, hs := range byGroup {
		c := s.clusters[gid]
		if c == nil {
			continue
		}
		// A group clearly of another age: a parent's picture searched
		// among their children's, say.
		if _, ca, ok := c.birth(); ok && age >= 0 && abs32(ca-age) > 2*birthTolerance(max(ca, age)) {
			continue
		}
		slices.SortFunc(hs, func(a, b hit) int { return cmp.Compare(b.s, a.s) })
		g := SimilarGroup{Group: gid, Person: c.PersonID, Auto: c.Auto, Hidden: c.Hidden, Size: c.n, Score: hs[0].s, Alike: len(hs)}
		if p := s.people[c.PersonID]; p != nil {
			g.Name = p.Name
		}
		for _, h := range hs[:min(len(hs), similarFaces)] {
			v := s.faceView(h.f)
			v.Score = h.s
			g.Faces = append(g.Faces, v)
		}
		out = append(out, g)
	}
	slices.SortFunc(out, func(a, b SimilarGroup) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Group, b.Group))
	})
	return out[:min(len(out), limit)]
}
