package store

import (
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestEvalAgeModel compares ways to predict how old a person looks at a
// given date, learnt from their confirmed faces ("at 50 he looks 36"):
//
//	birth:  one implied birth year (today's rule)
//	linear: apparent age as a straight line over capture time
//	local:  the person's faces from around the same date, aged by the
//	        time between them; the line where there are too few
//
// Each confirmed group is predicted from the person's other groups. The
// tolerance is set so 99% of the right person's groups pass; the score is
// how many other people's groups the age check then rules out.
func TestEvalAgeModel(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	type pt struct {
		t, a float64
		gid  int64
	}
	byPerson := map[int64][]pt{}
	groups := map[int64][]pt{}
	gPerson := map[int64]int64{}
	for _, f := range s.faces {
		if f.NotFace || f.AgeVersion == 0 || f.Age < 0 || f.bg {
			continue
		}
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID == 0 || c.Auto {
			continue
		}
		im := s.images[f.ImageID]
		if im == nil || im.TakenAt == nil || strings.Contains(strings.ToLower(im.CameraModel), "scan") {
			continue
		}
		tm := time.Unix(*im.TakenAt, 0).UTC()
		p := pt{float64(tm.Year()) + float64(tm.YearDay())/366, float64(f.Age), c.id}
		byPerson[c.PersonID] = append(byPerson[c.PersonID], p)
		groups[c.id] = append(groups[c.id], p)
		gPerson[c.id] = c.PersonID
	}
	for pid, ps := range byPerson {
		slices.SortFunc(ps, func(a, b pt) int { return cmpF(a.t, b.t) })
		byPerson[pid] = ps
	}

	type model func(ps []pt, skip int64, t float64) (float64, bool)
	birth := func(ps []pt, skip int64, t float64) (float64, bool) {
		var sum float64
		n := 0
		for _, p := range ps {
			if p.gid != skip {
				sum += p.t - p.a
				n++
			}
		}
		if n < 5 {
			return 0, false
		}
		return t - sum/float64(n), true
	}
	linear := func(ps []pt, skip int64, t float64) (float64, bool) {
		var n, st, sa, stt, sta float64
		lo, hi := math.Inf(1), math.Inf(-1)
		for _, p := range ps {
			if p.gid == skip {
				continue
			}
			n++
			st += p.t
			sa += p.a
			stt += p.t * p.t
			sta += p.t * p.a
			lo, hi = min(lo, p.t), max(hi, p.t)
		}
		if n < 5 {
			return 0, false
		}
		if hi-lo < 3 {
			return birth(ps, skip, t)
		}
		b := (n*sta - st*sa) / (n*stt - st*st)
		b = min(max(b, 0.3), 1.5) // ageing slower or faster, never backwards
		a := (sa - b*st) / n
		return a + b*t, true
	}
	local := func(ps []pt, skip int64, t float64) (float64, bool) {
		i := slices.IndexFunc(ps, func(p pt) bool { return p.t >= t-2 })
		var v []float64
		if i >= 0 {
			for _, p := range ps[i:] {
				if p.t > t+2 {
					break
				}
				if p.gid != skip {
					v = append(v, p.a+(t-p.t))
				}
			}
		}
		if len(v) < 10 {
			return linear(ps, skip, t)
		}
		slices.Sort(v)
		return v[len(v)/2], true
	}

	// Residual of a group against a person: mean over (up to 20) faces.
	resid := func(m model, g []pt, ps []pt, skip int64) (float64, float64, bool) {
		step := max(1, len(g)/20)
		var r, a float64
		n := 0
		for i := 0; i < len(g); i += step {
			pr, ok := m(ps, skip, g[i].t)
			if !ok {
				return 0, 0, false
			}
			r += g[i].a - pr
			a += g[i].a
			n++
		}
		return r / float64(n), a / float64(n), true
	}

	for _, mm := range []struct {
		name string
		m    model
	}{{"birth", birth}, {"linear", linear}, {"local", local}} {
		for _, band := range [][2]float64{{0, 16}, {16, 200}} {
			var same, other []float64
			for gid, g := range groups {
				if len(g) < 3 {
					continue
				}
				pid := gPerson[gid]
				r, a, ok := resid(mm.m, g, byPerson[pid], gid)
				if !ok || a < band[0] || a >= band[1] {
					continue
				}
				norm := func(r, a float64) float64 { return math.Abs(r) / (1 + 0.1*a) }
				same = append(same, norm(r, a))
				for qid, qs := range byPerson {
					if qid == pid || len(qs) < 20 {
						continue
					}
					if r, a, ok := resid(mm.m, g, qs, 0); ok {
						other = append(other, norm(r, a))
					}
				}
			}
			if len(same) < 10 {
				continue
			}
			slices.Sort(same)
			line := ""
			for _, q := range []int{90, 95, 99} {
				tol := same[len(same)*q/100]
				out := 0
				for _, o := range other {
					if o > tol {
						out++
					}
				}
				line += fmt.Sprintf("  keep %d%%: out %5.1f%%", q, 100*float64(out)/float64(len(other)))
			}
			t.Logf("%-6s ages %3.0f-%-3.0f  groups %4d  same p50 %.2f %s", mm.name, band[0], band[1], len(same), same[len(same)/2], line)
		}
	}
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
