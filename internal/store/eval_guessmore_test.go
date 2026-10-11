package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestEvalGuessClash lists unknown pairs asked though they clash with the family entered.
func TestEvalGuessClash(t *testing.T) {
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
	shown, dropped := 0, 0
	for _, g := range s.guessRelations(0, true) {
		if _, ok := fam[g.A][g.B]; ok {
			continue
		}
		if !g.Admit {
			continue
		}
		if g.Clash == "" {
			shown++
			continue
		}
		dropped++
		fmt.Printf("clash %-22s %-22s %-11s %s days %3d share %.2f births %.0f/%.0f %s/%s\n", g.AName, g.BName, g.Kind, g.Clash, g.Days, g.Share, g.ABirth, g.BBirth, g.AGender, g.BGender)
	}
	t.Logf("unknown pairs asked %d, asked despite a clash %d", shown, dropped)
}

// TestEvalGuessMissed lists every known pair, however few days together.
func TestEvalGuessMissed(t *testing.T) {
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
	evalAllDays = true
	defer func() { evalAllDays = false }()
	fam := s.family()
	var unknown []RelGuess
	for _, g := range s.guessRelations(0, true) {
		k, ok := fam[g.A][g.B]
		if !ok {
			unknown = append(unknown, g)
			continue
		}
		fmt.Printf("known %-12s %-22s %-22s guess %-11s days %3d share %.2f lift %4.1f beside %3d/%3d duo %3d\n", k, g.AName, g.BName, g.Kind, g.Days, g.Share, g.Lift, g.Beside, g.Photos, g.Duo)
	}
	t.Logf("%d unknown pairs with any day together", len(unknown))
}

// TestEvalGuessWrong lists entered relations the ages contradict.
func TestEvalGuessWrong(t *testing.T) {
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
	for i, g := range s.guessRelations(0, false) {
		if g.Entered != "" || i < 10 {
			fmt.Printf("%3d %-20s %-20s guess %-10s entered %-10s births %.0f/%.0f clash %q\n", i+1, g.AName, g.BName, g.Kind, g.Entered, g.ABirth, g.BBirth, g.Clash)
		}
	}
}

// TestEvalDiscoverCount shows the Discover badge on a copy, and how long it takes.
func TestEvalDiscoverCount(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := time.Now()
	p, r := s.DiscoverCount(context.Background())
	t.Logf("people %d, relations %d, in %v", p, r, time.Since(start))
	s.mu.RLock()
	hist := map[int]int{}
	for _, sc := range s.discoverChains() {
		hist[min(len(sc.days), 10)]++
	}
	s.mu.RUnlock()
	t.Logf("chains by days (10 = 10+): %v", hist)
}
