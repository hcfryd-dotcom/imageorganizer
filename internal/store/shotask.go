package store

import (
	"cmp"
	"context"
	"slices"
)

// ShotPair is "Same person?": two faces at the same place in
// near-identical shots, confirmed as two different people. One of them is
// likely confirmed wrong (look-alike siblings, mostly).
type ShotPair struct {
	A       FaceView `json:"a"`
	B       FaceView `json:"b"`
	AName   string   `json:"a_name"`
	BName   string   `json:"b_name"`
	APerson int64    `json:"a_person"` // whom each is confirmed as
	BPerson int64    `json:"b_person"`
	Gap     int64    `json:"gap"` // seconds between the shots
}

// ShotConflicts lists the faces confirmed as two people though at the
// same place in near-identical shots, not yet said to be two people.
func (s *Store) ShotConflicts(ctx context.Context) []ShotPair {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []ShotPair
	for id, gs := range s.shotPartners() {
		f := s.faces[id]
		if f == nil || f.Tagged == 0 {
			continue
		}
		for _, g := range gs {
			if g.ID < f.ID || g.Tagged == 0 || g.Tagged == f.Tagged || slices.Contains(f.Apart, g.ID) || slices.Contains(g.Apart, f.ID) {
				continue
			}
			a, b := s.people[f.Tagged], s.people[g.Tagged]
			if a == nil || b == nil {
				continue
			}
			sp := ShotPair{A: s.faceView(f), B: s.faceView(g), AName: a.Name, BName: b.Name, APerson: a.ID, BPerson: b.ID}
			if ia, ib := s.images[f.ImageID], s.images[g.ImageID]; ia != nil && ib != nil && ia.TakenAt != nil && ib.TakenAt != nil {
				sp.Gap = max(*ia.TakenAt-*ib.TakenAt, *ib.TakenAt-*ia.TakenAt)
			}
			out = append(out, sp)
		}
	}
	slices.SortFunc(out, func(x, y ShotPair) int { return cmp.Or(cmp.Compare(x.Gap, y.Gap), cmp.Compare(x.A.ID, y.A.ID)) })
	return out
}

// AnswerShot settles "Same person?" for faces a and b: both are person
// pid (the other one confirmed as them instead), or (pid 0) they are two
// people after all, and aren't asked about again.
func (s *Store) AnswerShot(ctx context.Context, a, b, pid int64) error {
	if pid != 0 {
		_, err := s.ConfirmFaces(ctx, pid, "", []int64{a, b}, nil)
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	fa, fb := s.faces[a], s.faces[b]
	if fa == nil || fb == nil {
		return ErrNotFound
	}
	if !slices.Contains(fa.Apart, b) {
		fa.Apart = append(slices.Clone(fa.Apart), b)
	}
	if !slices.Contains(fb.Apart, a) {
		fb.Apart = append(slices.Clone(fb.Apart), a)
	}
	ch := s.newChanges()
	ch.face(fa)
	ch.face(fb)
	return ch.save()
}
