package store

import (
	"context"
	"fmt"
	"testing"
)

// TestEvalAutoGrid: after playing Discover on the started-over copy, how
// many unnamed faces would auto-accept pick at other cut-offs, and how
// many of them wrongly (by the truth)? IO_EVAL_DIR as for TestEvalWizard.
func TestEvalAutoGrid(t *testing.T) {
	s, tr := wizardRun(t)
	ctx := context.Background()
	s.playDiscover(ctx, tr, 10, 60)
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx := s.personIndex()
	type ref struct {
		pid int64
		us  [][]float32
	}
	var refs []ref
	for q, qi := range idx {
		r := ref{pid: q}
		for _, g := range qi.groups {
			if !g.Auto {
				r.us = append(r.us, clusterUnit(g))
			}
		}
		if len(r.us) > 0 {
			refs = append(refs, r)
		}
	}
	type cand struct {
		sim, lead float32
		px        float32
		ageOK     bool
		right     bool
		known     bool
	}
	var cs []cand
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID != 0 || c.Hidden || f.NotFace || f.Picture || f.Tagged != 0 || len(f.emb) == 0 {
			continue
		}
		u := unitVec(f.emb)
		best, second := float32(-1), float32(-1)
		var who int64
		for _, r := range refs {
			b, _ := bestRef(u, r.us)
			if b > best {
				best, second, who = b, best, r.pid
			} else if b > second {
				second = b
			}
		}
		if best < 0.5 {
			continue
		}
		pi := idx[who]
		if pi.imgs[f.ImageID] {
			continue
		}
		by, ok := s.impliedBirth(f)
		want, known := tr.Face[f.ID]
		cs = append(cs, cand{sim: best, lead: best - second, px: s.facePx(f),
			ageOK: ok && pi.ages.fits(by, f.Age, true) && !genderMismatch(pi.gender, f),
			right: known && want == pi.p.Name, known: known})
	}
	for _, px := range []float32{0, 25, 40} {
		for _, age := range []bool{false, true} {
			for _, sim := range []float32{0.55, 0.60, 0.65, 0.70} {
				for _, lead := range []float32{0.05, 0.10, 0.15} {
					var right, wrong, unknown int
					for _, c := range cs {
						if c.sim < sim || c.lead < lead || c.px < px || (age && !c.ageOK) {
							continue
						}
						switch {
						case !c.known:
							unknown++
						case c.right:
							right++
						default:
							wrong++
						}
					}
					fmt.Printf("px %2.0f age %-5v sim %.2f lead %.2f: right %5d wrong %4d unknown %5d\n", px, age, sim, lead, right, wrong, unknown)
				}
			}
		}
	}
}
