package store

import (
	"cmp"
	"math"
	"os"
	"slices"
	"testing"
)

// TestEvalPairs measures whether near-duplicate photos (taken moments
// apart, alike as whole pictures) name each other's faces: the face at
// the same place in the other photo, is it the same person?
//
//	IO_EVAL_DB=<copy> go test -run TestEvalPairs -v
func TestEvalPairs(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.newMatcher()
	type img struct {
		id int64
		at int64
		u  []float32
		fs []*faceRec
	}
	var ims []img
	for id, im := range s.images {
		if im.TakenAt == nil || len(s.facesByImage[id]) == 0 || s.vectors[id] == nil {
			continue
		}
		var fs []*faceRec
		for _, fid := range s.facesByImage[id] {
			if f := s.faces[fid]; f != nil && !f.NotFace && len(f.emb) > 0 {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			ims = append(ims, img{id, *im.TakenAt, unitVec(s.vectors[id]), fs})
		}
	}
	slices.SortFunc(ims, func(a, b img) int { return cmp.Or(cmp.Compare(a.at, b.at), cmp.Compare(a.id, b.id)) })
	known := func(f *faceRec) int64 {
		if f.Tagged != 0 {
			return f.Tagged
		}
		if c := s.clusters[f.ClusterID]; c != nil {
			return c.PersonID
		}
		return 0
	}
	// partner is the face of b at a's place: nearest by centre (in a's
	// face widths), of a like size, and a's nearest back.
	near := func(f *faceRec, fs []*faceRec) (*faceRec, float64) {
		cx, cy, w := (f.Box[0]+f.Box[2])/2, (f.Box[1]+f.Box[3])/2, f.Box[2]-f.Box[0]
		var best *faceRec
		bd := math.Inf(1)
		for _, g := range fs {
			gw := g.Box[2] - g.Box[0]
			if gw < 0.7*w || gw > 1.4*w {
				continue
			}
			d := math.Hypot(float64((g.Box[0]+g.Box[2])/2-cx), float64((g.Box[1]+g.Box[3])/2-cy)) / float64(w)
			if d < bd {
				best, bd = g, d
			}
		}
		return best, bd
	}
	// With the face's own likeness too: how often the face at the same
	// place is the person when it also looks this much like them (judged
	// against faces the user confirmed), and how many unnamed faces under
	// the filing bar it would name.
	for _, L := range []float32{0, 0.30, 0.40, 0.45} {
		var agree, dis, gain int
		for i, a := range ims {
			for j := i + 1; j < len(ims) && ims[j].at-a.at <= 60; j++ {
				b := ims[j]
				if dot(a.u, b.u) < 0.90 {
					continue
				}
				for _, dir := range [2][2]img{{a, b}, {b, a}} {
					x, y := dir[0], dir[1]
					for _, f := range x.fs {
						g, d := near(f, y.fs)
						if g == nil || d > 0.5 {
							continue
						}
						if back, _ := near(g, x.fs); back != f {
							continue
						}
						p := known(f)
						if p == 0 {
							continue
						}
						likes := m.likes(g)
						l, ok := likes[p]
						if !ok || l < L {
							continue
						}
						if b := m.best(g, likes); b != 0 && b != p {
							continue // more like someone else
						}
						switch {
						case g.Tagged != 0 && g.Tagged == p:
							agree++
						case g.Tagged != 0:
							dis++
							if L == 0 {
								t.Logf("  dis: face %d (img %d, tag %d, filed %d) → face %d (img %d) tagged %d; like %.2f vs own %.2f; d %.2f; Δt %ds; alike %.3f",
									f.ID, f.ImageID, f.Tagged, known(f), g.ID, g.ImageID, g.Tagged, l, likes[g.Tagged], d, y.at-x.at, dot(x.u, y.u))
							}
						case known(g) == 0 && l < fileSim:
							gain++
						}
					}
				}
			}
		}
		t.Logf("alike ≥ 0.90 within 60s, face like ≥ %.2f and liked best: confirmed agree %d / %d (%.2f%%); unnamed faces under the filing bar it'd name %d",
			L, agree, dis, 100*float64(agree)/float64(max(agree+dis, 1)), gain)
	}
	type tally struct{ pairs, confAgree, confDis, knownAgree, knownDis, wouldName, lowLike int }
	for _, dt := range []int64{60, 600} {
		for _, tau := range []float32{0, 0.85, 0.90, 0.95} {
			var tl tally
			for i, a := range ims {
				for j := i + 1; j < len(ims) && ims[j].at-a.at <= dt; j++ {
					b := ims[j]
					if dot(a.u, b.u) < tau {
						continue
					}
					tl.pairs++
					for _, dir := range [2][2]img{{a, b}, {b, a}} {
						x, y := dir[0], dir[1]
						for _, f := range x.fs {
							g, d := near(f, y.fs)
							if g == nil || d > 0.5 {
								continue
							}
							if back, _ := near(g, x.fs); back != f {
								continue
							}
							p, q := known(f), known(g)
							if p == 0 {
								continue
							}
							if f.Tagged != 0 && g.Tagged != 0 {
								if f.Tagged == g.Tagged {
									tl.confAgree++
								} else {
									tl.confDis++
								}
							}
							if q != 0 {
								if p == q {
									tl.knownAgree++
								} else {
									tl.knownDis++
								}
								continue
							}
							if c := s.clusters[g.ClusterID]; c != nil && c.Hidden {
								continue
							}
							tl.wouldName++
							if l, ok := m.likes(g)[p]; !ok || l < fileSim {
								tl.lowLike++
							}
						}
					}
				}
			}
			pct := func(a, b int) float64 { return 100 * float64(a) / float64(max(a+b, 1)) }
			t.Logf("within %4ds, picture alike ≥ %.2f: %6d photo pairs; confirmed agree %5d / %3d (%.1f%%), named agree %6d / %4d (%.2f%%); unnamed faces it'd name %5d (of which below filing likeness %d)",
				dt, tau, tl.pairs, tl.confAgree, tl.confDis, pct(tl.confAgree, tl.confDis), tl.knownAgree, tl.knownDis, pct(tl.knownAgree, tl.knownDis), tl.wouldName, tl.lowLike)
		}
	}
}
