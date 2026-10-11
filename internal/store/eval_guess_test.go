package store

import (
	"fmt"
	"os"
	"testing"
)

// TestEvalGuessRelations measures the relation guesses on a copy of a real
// library against the family entered there:
//
//	IO_EVAL_DB=<copy> go test -run TestEvalGuessRelations -v
func TestEvalGuessRelations(t *testing.T) {
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
	fam := s.family()
	gs := s.guessRelations(0, true)
	var related, kindRight, known int
	for _, rel := range fam {
		known += len(rel)
	}
	for i, g := range gs {
		k, ok := fam[g.A][g.B]
		mark := "?"
		if ok {
			related++
			mark = "related: " + k
			if k == g.Kind {
				kindRight++
				mark = "RIGHT " + k
			}
		}
		if !g.Admit {
			if ok {
				fmt.Printf("filtered: %s – %s (%s) days %d share %.2f lift %.1f beside %d/%d duo %d\n", g.AName, g.BName, k, g.Days, g.Share, g.Lift, g.Beside, g.Photos, g.Duo)
			}
			continue
		}
		if i < 60 {
			fmt.Printf("%2d %-22s %-22s guess %-11s %-22s days %3d share %.2f lift %4.1f beside %3d/%3d duo %3d births %.0f/%.0f %s/%s conf %.2f\n",
				i+1, g.AName, g.BName, g.Kind, mark, g.Days, g.Share, g.Lift, g.Beside, g.Photos, g.Duo, g.ABirth, g.BBirth, g.AGender, g.BGender, g.Confidence)
		}
	}
	t.Logf("%d guesses; %d are related as entered (%d known pairs, both ways), kind right for %d", len(gs), related, known/2, kindRight)
	// Known pairs that weren't guessed at all.
	guessed := map[[2]int64]bool{}
	for _, g := range gs {
		guessed[[2]int64{g.A, g.B}], guessed[[2]int64{g.B, g.A}] = true, true
	}
	for a, rel := range fam {
		for b, k := range rel {
			if a < b && !guessed[[2]int64{a, b}] {
				fmt.Printf("missed: %s – %s (%s)\n", s.people[a].Name, s.people[b].Name, k)
			}
		}
	}
}
