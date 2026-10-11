package store

import (
	"math"
	"os"
	"slices"
	"testing"
	"time"
)

// TestEvalKnownAge looks at people whose birth year the user entered: how
// far apparent ages are off at each true age, and whether comparing a
// face with the person's faces from around the same age separates them
// from others better than comparing with all of the person's faces.
func TestEvalKnownAge(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	type fa struct {
		f       *faceRec
		pid     int64
		trueAge float32
		hasTrue bool
	}
	var all []fa
	for _, f := range s.faces {
		if f.NotFace || f.AgeVersion == 0 || f.Age < 0 || f.bg {
			continue
		}
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID == 0 || c.Auto {
			continue
		}
		p := s.people[c.PersonID]
		im := s.images[f.ImageID]
		if p == nil || im == nil || im.TakenAt == nil {
			continue
		}
		x := fa{f: f, pid: p.ID}
		if p.BirthYear != 0 {
			tm := time.Unix(*im.TakenAt, 0).UTC()
			x.trueAge = float32(tm.Year()) + float32(tm.YearDay())/366 - float32(p.BirthYear) - 0.5
			x.hasTrue = x.trueAge >= 0
		}
		all = append(all, x)
	}

	// 1. Apparent age error by true age.
	buckets := []float32{0, 2, 4, 7, 10, 13, 18, 25, 35, 50, 65, 100}
	t.Logf("apparent - true age, by true age (known birth years only)")
	t.Logf("%8s %6s %7s %7s %7s %7s", "age", "n", "bias", "sd", "p90|e|", "tol")
	for i := 0; i+1 < len(buckets); i++ {
		var errs []float64
		for _, x := range all {
			if x.hasTrue && x.trueAge >= buckets[i] && x.trueAge < buckets[i+1] {
				errs = append(errs, float64(x.f.Age-x.trueAge))
			}
		}
		if len(errs) < 20 {
			continue
		}
		var m, v float64
		for _, e := range errs {
			m += e
		}
		m /= float64(len(errs))
		for _, e := range errs {
			v += (e - m) * (e - m)
		}
		abs := make([]float64, len(errs))
		for j, e := range errs {
			abs[j] = math.Abs(e)
		}
		slices.Sort(abs)
		mid := (buckets[i] + buckets[i+1]) / 2
		t.Logf("%3.0f-%-4.0f %6d %+7.1f %7.1f %7.1f %7.1f", buckets[i], buckets[i+1], len(errs), m, math.Sqrt(v/float64(len(errs))), abs[len(abs)*9/10], birthTolerance(mid))
	}

	// 2. Same person vs other people: whole-person centroid against
	// centroid of the person's faces within ±window years of true age.
	known := map[int64]bool{}
	for _, x := range all {
		if x.hasTrue {
			known[x.pid] = true
		}
	}
	byPerson := map[int64][]fa{}
	for _, x := range all {
		byPerson[x.pid] = append(byPerson[x.pid], x)
	}
	mean := func(xs []fa, skip *faceRec, keep func(fa) bool) ([]float32, int) {
		var c []float32
		n := 0
		for _, x := range xs {
			if x.f == skip || !keep(x) {
				continue
			}
			if c == nil {
				c = make([]float32, len(x.f.emb))
			}
			for i, v := range x.f.emb {
				c[i] += v
			}
			n++
		}
		if n == 0 {
			return nil, 0
		}
		var l float32
		for _, v := range c {
			l += v * v
		}
		l = float32(math.Sqrt(float64(l)))
		for i := range c {
			c[i] /= l
		}
		return c, n
	}
	// Sample: up to 400 faces per known person, spread over the list.
	for pid := range known {
		xs := byPerson[pid]
		step := max(1, len(xs)/400)
		for _, win := range []float32{0, 2, 4} {
			var same, other []float64
			for i := 0; i < len(xs); i += step {
				x := xs[i]
				if !x.hasTrue {
					continue
				}
				keep := func(y fa) bool { return true }
				if win > 0 {
					keep = func(y fa) bool { return y.hasTrue && abs32(y.trueAge-x.trueAge) <= win }
				}
				c, n := mean(xs, x.f, keep)
				if n < 5 {
					c, _ = mean(xs, x.f, func(fa) bool { return true })
				}
				same = append(same, float64(dot(x.f.emb, c)))
			}
			// Other people's faces against this person's age-matched
			// centroid: use the other face's apparent age as the guess.
			for qid, ys := range byPerson {
				if qid == pid {
					continue
				}
				st := max(1, len(ys)/40)
				for i := 0; i < len(ys); i += st {
					y := ys[i]
					keep := func(z fa) bool { return true }
					if win > 0 {
						est := y.f.Age
						keep = func(z fa) bool { return z.hasTrue && abs32(z.trueAge-est) <= win+2 }
					}
					c, n := mean(xs, nil, keep)
					if n < 5 {
						c, _ = mean(xs, nil, func(fa) bool { return true })
					}
					other = append(other, float64(dot(y.f.emb, c)))
				}
			}
			slices.Sort(same)
			slices.Sort(other)
			// How many others score above the same-person 5th percentile,
			// and same-person faces below the others' 99th percentile.
			p5 := same[len(same)/20]
			p99 := other[len(other)*99/100]
			fa := 0
			for _, o := range other {
				if o >= p5 {
					fa++
				}
			}
			miss := 0
			for _, v := range same {
				if v < p99 {
					miss++
				}
			}
			t.Logf("%-8s win=%v  same p5=%.3f  other p99=%.3f  others≥p5 %.2f%%  same<p99 %.1f%%",
				s.people[pid].Name, win, p5, p99, 100*float64(fa)/float64(len(other)), 100*float64(miss)/float64(len(same)))
		}
	}
}
