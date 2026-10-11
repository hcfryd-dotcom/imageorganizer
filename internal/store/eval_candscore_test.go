package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestEvalCandScore: of the faces offered as "Is this also …?", how many
// are the person at each score, by the names before the start-over (Was),
// and how many came in on their company or a relative.
//
//	IO_EVAL_DB=<copy> go test -run TestEvalCandScore -v
func TestEvalCandScore(t *testing.T) {
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
	type band struct{ right, wrong, why, whyRight int }
	bands := map[int]*band{}
	only, _ := strconv.ParseInt(os.Getenv("IO_EVAL_PERSON"), 10, 64)
	was := map[int64]int{}
	for pid := range s.people {
		if only != 0 && pid != only {
			continue
		}
		for _, v := range s.faceCandidates(m, pid, 120) {
			f := s.faces[v.ID]
			if only != 0 {
				was[f.Was]++
				mt := m.match(f, pid, m.likes(f))
				t.Logf("cand was %3d score %.2f like %.2f kin %.2f event %.2f shot %.2f cost %.2f age %.0f why %q company %v",
					f.Was, v.Score, mt.Like, mt.Kin, mt.Event, mt.Shot, mt.Cost, f.Age, v.Why, len(v.Company) > 0)
			}
			if f.Was == 0 {
				continue
			}
			k := int(v.Score * 10)
			if bands[k] == nil {
				bands[k] = &band{}
			}
			b := bands[k]
			why := v.Why != "" || len(v.Company) > 0
			if f.Was == pid {
				b.right++
				if why {
					b.whyRight++
				}
			} else {
				b.wrong++
			}
			if why {
				b.why++
			}
		}
	}
	if only != 0 {
		for w, n := range was {
			name := "no old name"
			if p := s.people[w]; p != nil {
				name = p.Name
			}
			t.Logf("was %d %s: %d", w, name, n)
		}
	}
	for k := 0; k <= 10; k++ {
		if b := bands[k]; b != nil {
			t.Logf("score %3d–%3d%%: right %5d wrong %5d (%.0f%% right); in on company/relative %d, of them right %d",
				k*10, k*10+9, b.right, b.wrong, 100*float64(b.right)/float64(b.right+b.wrong), b.why, b.whyRight)
		}
	}
}

func TestEvalAgeOf(t *testing.T) {
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
	pid, _ := strconv.ParseInt(os.Getenv("IO_EVAL_PERSON"), 10, 64)
	m := s.newMatcher()
	pi := m.idx[pid]
	p := s.people[pid]
	t.Logf("birth year %d unknown %v picture %v; ages %+v; groups %d", p.BirthYear, p.BirthUnknown, p.Picture, pi.ages, len(pi.groups))
	n, aged := 0, 0
	for _, f := range s.faces {
		if f.Tagged == pid {
			n++
			if _, ok := s.impliedBirth(f); ok {
				aged++
			}
		}
	}
	t.Logf("confirmed %d, with an implied birth %d", n, aged)
}

func TestEvalDoubtCount(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ds := s.DoubtfulFaces(context.Background(), 0)
	conf, other, age := 0, 0, 0
	for _, d := range ds {
		if !d.Auto {
			conf++
		}
		if d.Other != 0 {
			other++
		}
		if d.AgeOff {
			age++
		}
	}
	t.Logf("doubts %d: confirmed %d, more like someone else %d, age off %d", len(ds), conf, other, age)
	t0 := time.Now()
	pd := s.PersonDoubts(context.Background(), 1, 40)
	t.Logf("person 1: %d in %v; first scores %.2f %.2f %.2f", len(pd), time.Since(t0).Round(time.Millisecond), pd[0].Score, pd[1].Score, pd[2].Score)
}

func TestEvalFiledScore(t *testing.T) {
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
	var fs []*faceRec
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 && f.Tagged == 0 && f.Was != 0 && !f.NotFace && len(f.emb) > 0 {
			fs = append(fs, f)
		}
	}
	sc := make([]float32, len(fs))
	parallel(len(fs), func(i int) {
		f := fs[i]
		sc[i] = m.match(f, s.clusters[f.ClusterID].PersonID, m.likes(f)).Score
	})
	type band struct{ right, wrong int }
	bands := map[int]*band{}
	for i, f := range fs {
		k := int(sc[i] * 20)
		if bands[k] == nil {
			bands[k] = &band{}
		}
		if f.Was == s.clusters[f.ClusterID].PersonID {
			bands[k].right++
		} else {
			bands[k].wrong++
		}
	}
	for k := 0; k <= 20; k++ {
		if b := bands[k]; b != nil {
			t.Logf("filed, score %3d–%3d%%: %6d right, %4d wrong (%.2f%%)", k*5, k*5+4, b.right, b.wrong, 100*float64(b.wrong)/float64(b.right+b.wrong))
		}
	}
}
