package store

import (
	"cmp"
	"math"
	"slices"
)

// Near-identical shots: photos taken moments apart that look alike as
// whole pictures (a burst, a second try). The face at the same place in
// the other shot is nearly always the same person (TestEvalPairs: 99%
// with the face's likeness, the rest look-alike brothers confirmed the
// wrong way round), so it counts as evidence: for the person it is, and
// against anyone else.

const (
	shotGap   = 60   // seconds between two shots, at most
	shotAlike = 0.90 // their likeness as whole pictures, at least
	shotNear  = 0.5  // the faces' centres apart, in face widths, at most
	shotSize  = 1.4  // and their widths at most this many times apart
)

// shotBonus is what the face at the same place in a near-identical shot
// adds for the person it is, and takes off anyone else. A variable for
// TestEvalStartOver (0: off): from no names, 0.05 filed 2% more faces for
// one more wrong in 27,000; 0.10 filed 3% more for two.
var shotBonus float32 = 0.05

// shotKey says when the shot partners were found: the library they were
// found in.
type shotKey struct{ images, faces, vectors int }

// shotPartners is each face's partners: the face at its place in each
// near-identical shot. Worked out once, again when photos or faces are
// added. Call with s.mu held.
func (s *Store) shotPartners() map[int64][]*faceRec {
	key := shotKey{len(s.images), len(s.faces), len(s.vectors)}
	s.shotMu.Lock()
	defer s.shotMu.Unlock()
	if s.shots != nil && s.shotAt == key {
		return s.shots
	}
	type shot struct {
		id int64
		at int64
		u  []float32
		fs []*faceRec
	}
	var shots []shot
	for id, ids := range s.facesByImage {
		im, u := s.images[id], s.vectors[id]
		if im == nil || im.TakenAt == nil || u == nil {
			continue
		}
		var fs []*faceRec
		for _, fid := range ids {
			if f := s.faces[fid]; f != nil && s.shown(f) && !f.Picture && len(f.emb) > 0 {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			shots = append(shots, shot{id, *im.TakenAt, u, fs})
		}
	}
	slices.SortFunc(shots, func(a, b shot) int { return cmp.Or(cmp.Compare(a.at, b.at), cmp.Compare(a.id, b.id)) })
	out := map[int64][]*faceRec{}
	for i, a := range shots {
		for _, b := range shots[i+1:] {
			if b.at-a.at > shotGap {
				break
			}
			if dot(a.u, b.u) < shotAlike {
				continue
			}
			for _, f := range a.fs {
				g := atPlace(f, b.fs)
				if g != nil && atPlace(g, a.fs) == f {
					out[f.ID] = append(out[f.ID], g)
					out[g.ID] = append(out[g.ID], f)
				}
			}
		}
	}
	s.shots, s.shotAt = out, key
	return out
}

// atPlace is the face of fs at face f's place: the nearest by centre, of
// a like size, near enough; nil if none.
func atPlace(f *faceRec, fs []*faceRec) *faceRec {
	cx, cy, w := (f.Box[0]+f.Box[2])/2, (f.Box[1]+f.Box[3])/2, f.Box[2]-f.Box[0]
	var best *faceRec
	bd := shotNear
	for _, g := range fs {
		gw := g.Box[2] - g.Box[0]
		if w <= 0 || gw < w/shotSize || gw > w*shotSize {
			continue
		}
		if d := math.Hypot(float64((g.Box[0]+g.Box[2])/2-cx), float64((g.Box[1]+g.Box[3])/2-cy)) / float64(w); d <= bd {
			best, bd = g, d
		}
	}
	return best
}

// shot is what near-identical shots say about face f being person pid:
// shotBonus if the face at its place in one is them (confirmed, or filed
// by itself), less shotBonus if it's someone else and none is them.
func (m *matcher) shot(f *faceRec, pid int64) float32 {
	if shotBonus == 0 {
		return 0
	}
	var them, other bool
	for _, g := range m.shots[f.ID] {
		if slices.Contains(f.Apart, g.ID) {
			continue
		}
		who := g.Tagged
		if who == 0 {
			if c := m.s.clusters[g.ClusterID]; c != nil {
				who = c.PersonID
			}
		}
		switch {
		case who == 0:
		case who == pid:
			them = true
		default:
			other = true
		}
	}
	switch {
	case them:
		return shotBonus
	case other:
		return -shotBonus
	}
	return 0
}
