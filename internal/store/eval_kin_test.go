package store

import (
	"fmt"
	"os"
	"slices"
	"testing"
)

// TestEvalKin measures how much the people in a photo help name a face:
// confirmed faces in photos with other confirmed people, each compared
// with everyone's confirmed groups but its own (as if unnamed), ranked by
// likeness alone and with bonuses for relatives in the photo.
//
//	IO_EVAL_DB=<copy> go test -run TestEvalKin -v
func TestEvalKin(t *testing.T) {
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
	idx, fam := s.confirmedIndex(), s.family()
	type ref struct {
		gid int64
		u   []float32
	}
	refs := map[int64][]ref{}
	for pid, pi := range idx {
		for _, g := range pi.groups {
			refs[pid] = append(refs[pid], ref{g.id, clusterUnit(g)})
		}
	}
	close := map[string]bool{KinPartner: true, KinParent: true, KinChild: true, KinSibling: true, KinHalfSibling: true}
	type cand struct {
		pid      int64
		sim      float32
		kin      float32 // kinNear's bonus
		near     bool    // beside
		closeKin bool
		strict   bool // the age fits strictly, and the gender: as lane auto-accept wants
	}
	type item struct {
		truth int64
		cs    []cand
	}
	var items []item
	n := 0
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if f.NotFace || f.Picture || len(f.emb) == 0 || c == nil || c.Auto || c.PersonID == 0 || s.facePx(f) < 40 {
			continue
		}
		here := s.whoIn(f.ImageID, f.ID)
		if len(here) == 0 {
			continue
		}
		if n++; n%5 != 0 { // a sample
			continue
		}
		u := unitVec(f.emb)
		by, byOK := s.impliedBirth(f)
		it := item{truth: c.PersonID}
		for pid, rs := range refs {
			if here[pid] != nil {
				continue // in the photo already
			}
			best := float32(-1)
			for _, r := range rs {
				if r.gid != c.id {
					best = max(best, dot(u, r.u))
				}
			}
			if best < 0 {
				continue
			}
			if byOK && !idx[pid].ages.fitsLike(by, f.Age, true, best) {
				continue
			}
			cd := cand{pid: pid, sim: best, strict: byOK && idx[pid].ages.fits(by, f.Age, true) && !genderMismatch(idx[pid].gender, f)}
			for q, of := range here {
				k, ok := fam[pid][q]
				if !ok {
					continue
				}
				b := float32(kinPhoto)
				if beside(f, of) {
					b = kinNext
				}
				if b > cd.kin || b == cd.kin && close[k] {
					cd.kin, cd.near, cd.closeKin = b, b == kinNext, close[k]
				}
			}
			it.cs = append(it.cs, cd)
		}
		if len(it.cs) > 0 && slices.ContainsFunc(it.cs, func(c cand) bool { return c.pid == it.truth }) {
			items = append(items, it)
		}
	}
	t.Logf("%d faces with company, the right person among the candidates", len(items))
	type variant struct {
		name  string
		score func(c cand) float32
	}
	vs := []variant{
		{"likeness only", func(c cand) float32 { return c.sim }},
		{"current (+.06 / +.12 beside)", func(c cand) float32 { return c.sim + c.kin }},
		{"double", func(c cand) float32 { return c.sim + 2*c.kin }},
		{"close kin x2, others x1", func(c cand) float32 {
			if c.closeKin {
				return c.sim + 2*c.kin
			}
			return c.sim + c.kin
		}},
		{"close kin x3, others x1", func(c cand) float32 {
			if c.closeKin {
				return c.sim + 3*c.kin
			}
			return c.sim + c.kin
		}},
		{"flat +.10 any kin", func(c cand) float32 {
			if c.kin > 0 {
				return c.sim + .10
			}
			return c.sim
		}},
	}
	for _, v := range vs {
		top := 0
		type acc struct{ ok, bad int }
		at := map[float32]*acc{}
		ths := []float32{.40, .45, .50, .55, .60}
		for _, th := range ths {
			at[th] = &acc{}
		}
		for _, it := range items {
			var b1, b2 cand
			s1, s2 := float32(-9), float32(-9)
			for _, c := range it.cs {
				sc := v.score(c)
				if sc > s1 {
					b2, s2, b1, s1 = b1, s1, c, sc
				} else if sc > s2 {
					b2, s2 = c, sc
				}
			}
			_ = b2
			if b1.pid == it.truth {
				top++
			}
			for _, th := range ths {
				if s1 >= th && s1-s2 >= .10 {
					if b1.pid == it.truth {
						at[th].ok++
					} else {
						at[th].bad++
					}
				}
			}
		}
		// Lane auto-accept's rules: strict age and gender, a lead of .15.
		laneOK, laneBad := 0, 0
		for _, it := range items {
			var b1 cand
			s1, s2 := float32(-9), float32(-9)
			for _, c := range it.cs {
				sc := v.score(c)
				if sc > s1 {
					s2, b1, s1 = s1, c, sc
				} else if sc > s2 {
					s2 = sc
				}
			}
			if s1 >= laneAutoSim && s1-s2 >= laneAutoLead && b1.strict {
				if b1.pid == it.truth {
					laneOK++
				} else {
					laneBad++
				}
			}
		}
		fmt.Printf("%-30s lane auto-accept: %5d right %3d wrong\n", v.name, laneOK, laneBad)
		line := fmt.Sprintf("%-30s top-1 %5.1f%%", v.name, 100*float32(top)/float32(len(items)))
		for _, th := range ths {
			a := at[th]
			line += fmt.Sprintf(" | ≥%.2f: %5d ok %4d wrong (%.1f%%)", th, a.ok, a.bad, 100*float32(a.bad)/float32(max(1, a.ok+a.bad)))
		}
		fmt.Println(line)
	}
}
