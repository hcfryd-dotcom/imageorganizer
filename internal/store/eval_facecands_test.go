package store

import (
	"context"
	"os"
	"sort"
	"testing"
	"time"
)

// TestEvalFaceCandidates measures the per-face suggestions on a database
// copy: speed, and how a face's best likeness to a person's other groups
// separates them (genuine) from other named people (impostors).
func TestEvalFaceCandidates(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	s.mu.RLock()
	idx := s.personIndex()
	s.mu.RUnlock()
	for pid, pi := range idx {
		if len(pi.groups) < 3 {
			continue
		}
		st := time.Now()
		out := s.FaceCandidates(ctx, pid, 60)
		auto := 0
		for _, v := range out {
			if v.Auto {
				auto++
			}
		}
		lo, hi := float32(0), float32(0)
		if len(out) > 0 {
			lo, hi = out[len(out)-1].Score, out[0].Score
		}
		t.Logf("%-12s %3d groups: %2d faces (%2d auto) %.2f..%.2f in %v", pi.p.Name, len(pi.groups), len(out), auto, lo, hi, time.Since(st).Round(time.Millisecond))
	}
	// Genuine vs impostor scores at the thresholds.
	s.mu.RLock()
	defer s.mu.RUnlock()
	var gen, imp []float32
	for pid, pi := range idx {
		var conf []*cluster
		for _, g := range pi.groups {
			if !g.Auto {
				conf = append(conf, g)
			}
		}
		if len(conf) < 3 {
			continue
		}
		for _, f := range s.faces {
			c := s.clusters[f.ClusterID]
			if c == nil || c.PersonID == 0 || c.Auto || f.NotFace || f.bg || len(f.emb) == 0 {
				continue
			}
			var refs [][]float32
			for _, g := range conf {
				if g != c {
					refs = append(refs, clusterUnit(g))
				}
			}
			if by, ok := s.impliedBirth(f); ok && !pi.ages.fits(by, f.Age, true) && c.PersonID != pid {
				continue
			}
			sc := maxSims(1, func(int) []float32 { return unitVec(f.emb) }, refs)[0]
			if c.PersonID == pid {
				gen = append(gen, sc)
			} else {
				imp = append(imp, sc)
			}
		}
	}
	sort.Slice(gen, func(i, j int) bool { return gen[i] < gen[j] })
	sort.Slice(imp, func(i, j int) bool { return imp[i] < imp[j] })
	above := func(xs []float32, x float32) float64 {
		i := sort.Search(len(xs), func(i int) bool { return xs[i] >= x })
		return float64(len(xs)-i) / float64(len(xs))
	}
	for _, x := range []float32{0.2, 0.25, 0.3, 0.35, 0.4, 0.45, 0.5} {
		t.Logf("≥%.2f: genuine %5.1f%%  impostor %5.2f%%  (%d / %d)", x, 100*above(gen, x), 100*above(imp, x), len(gen), len(imp))
	}
}

func TestEvalFaceMatches(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for range 2 {
		st := time.Now()
		out := s.FaceMatches(context.Background(), 60, MatchOptions{})
		t.Logf("%d matches in %v", len(out), time.Since(st).Round(time.Millisecond))
		for _, m := range out[:min(len(out), 8)] {
			t.Logf("face %d → %s %.2f (others %v)", m.Face.ID, m.Name, m.Score, m.Others)
		}
		if len(out) > 0 {
			var fr float32
			low := 0
			s.mu.RLock()
			for _, m := range out {
				x := s.frontal(s.faces[m.Face.ID])
				fr += x
				if x < 0.3 {
					low++
				}
			}
			s.mu.RUnlock()
			t.Logf("last %.2f; face-on %.2f on average, %d of %d below 0.3", out[len(out)-1].Score, fr/float32(len(out)), low, len(out))
		}
	}
	s.mu.RLock()
	n := 0
	for _, c := range s.clusters {
		if c.PersonID == 0 && !c.Hidden && c.n > 0 {
			n++
		}
	}
	s.mu.RUnlock()
	t.Logf("%d unnamed groups", n)
}

func TestEvalCompany(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	st := time.Now()
	s.mu.RLock()
	s.company()
	s.mu.RUnlock()
	t.Logf("company %v", time.Since(st))
	st = time.Now()
	items, total := s.Review(context.Background(), s.ReviewMin(), 0, 15, 4, ReviewKnown)
	t.Logf("review %v, %d groups", time.Since(st), total)
	for _, it := range items {
		if k := it.Company; k != nil {
			t.Logf("group %d (%d faces): %d days %d-%d, %d places, with %v → %.0f", it.ID, it.Size, k.Days, k.From, k.To, k.Places, k.With, k.Score)
		}
	}
}
