package store

import (
	"cmp"
	"os"
	"slices"
	"testing"
)

// TestEvalAutoAccept: for faces in confirmed groups, the best person by
// likeness to everyone's other confirmed groups (the face's own group left
// out); how often that best person is wrong, by likeness and lead over the
// runner-up, among faces whose age fits and that aren't tiny.
func TestEvalAutoAccept(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx := s.personIndex()
	type ref struct {
		pid int64
		pi  *personInfo
		gs  []*cluster
		us  [][]float32
	}
	var people []ref
	for pid, pi := range idx {
		r := ref{pid: pid, pi: pi}
		for _, g := range pi.groups {
			if !g.Auto {
				r.gs, r.us = append(r.gs, g), append(r.us, clusterUnit(g))
			}
		}
		if len(r.gs) > 0 {
			people = append(people, r)
		}
	}
	type res struct {
		sim, lead float32
		right     bool
	}
	var rs []res
	n := 0
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID == 0 || c.Auto || f.NotFace || f.bg || len(f.emb) == 0 || s.facePx(f) < 40 {
			continue
		}
		if n++; n%7 != 0 { // a sample
			continue
		}
		u := unitVec(f.emb)
		by, byOK := s.impliedBirth(f)
		type sc struct {
			pid int64
			s   float32
		}
		var all []sc
		for _, p := range people {
			best := float32(-1)
			for k, g := range p.gs {
				if g != c {
					best = max(best, dot(u, p.us[k]))
				}
			}
			if best < 0 || byOK && !p.pi.ages.fits(by, f.Age, true) {
				continue
			}
			all = append(all, sc{p.pid, best})
		}
		if len(all) < 2 {
			continue
		}
		slices.SortFunc(all, func(a, b sc) int { return cmp.Compare(b.s, a.s) })
		rs = append(rs, res{all[0].s, all[0].s - all[1].s, all[0].pid == c.PersonID})
	}
	for _, lead := range []float32{0, 0.10, 0.15, 0.20} {
		for _, min := range []float32{0.55, 0.60, 0.65, 0.70, 0.75} {
			pass, wrong := 0, 0
			for _, r := range rs {
				if r.sim >= min && r.lead >= lead {
					pass++
					if !r.right {
						wrong++
					}
				}
			}
			t.Logf("lead ≥%.2f likeness ≥%.2f: %5d of %d pass (%.0f%%), %d wrong (%.2f%%)", lead, min, pass, len(rs), 100*float64(pass)/float64(len(rs)), wrong, 100*float64(wrong)/float64(max(pass, 1)))
		}
	}
}
