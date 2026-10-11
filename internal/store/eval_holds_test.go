package store

import (
	"cmp"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
)

// TestEvalHolds tallies why faces not filed aren't: each unnamed face's
// likeliest person, and what holds it back (Match.Hold), by score.
//
//	IO_EVAL_DB=<copy> go test -run TestEvalHolds -v
func TestEvalHolds(t *testing.T) {
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
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID != 0 || c.Hidden || f.Tagged != 0 || f.NotFace || f.Picture || len(f.emb) == 0 {
			continue
		}
		fs = append(fs, f)
	}
	mts := make([]Match, len(fs))
	parallel(len(fs), func(i int) {
		likes := m.likes(fs[i])
		if who := m.best(fs[i], likes); who != 0 {
			mts[i] = m.match(fs[i], who, likes)
		}
	})
	type key struct {
		bucket string
		hold   string
	}
	n := map[key]int{}
	right, wrong := map[key]int{}, map[key]int{} // by the old names (Was), where there is one
	for i, mt := range mts {
		if mt.Person == 0 {
			continue
		}
		b := "<0.45"
		switch {
		case mt.Score >= 0.70:
			b = "≥0.70"
		case mt.Score >= 0.60:
			b = "0.60–0.70"
		case mt.Score >= 0.55:
			b = "0.55–0.60"
		case mt.Score >= 0.45:
			b = "0.45–0.55"
		}
		h := mt.Hold
		for _, p := range []string{"looks", "almost as like", "as like someone", "reads as"} {
			if strings.HasPrefix(h, p) {
				h = p + "…"
			}
		}
		if mt.Sure {
			h = "(sure)"
		}
		n[key{b, h}]++
		switch was := fs[i].Was; {
		case was == mt.Person:
			right[key{b, h}]++
		case was != 0:
			wrong[key{b, h}]++
		}
	}
	var ks []key
	for k := range n {
		ks = append(ks, k)
	}
	slices.SortFunc(ks, func(a, b key) int { return cmp.Or(cmp.Compare(b.bucket, a.bucket), cmp.Compare(n[b], n[a])) })
	for _, k := range ks {
		t.Log(fmt.Sprintf("%-10s %6d  old name agrees %5d, differs %4d  %s", k.bucket, n[k], right[k], wrong[k], k.hold))
	}
}
