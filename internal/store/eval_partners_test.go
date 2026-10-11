package store

import (
	"fmt"
	"os"
	"testing"
)

// TestEvalPartners lists the partner questions on a database copy, in the
// order asked, with when the pair was photographed together.
func TestEvalPartners(t *testing.T) {
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
		if g.Kind == KinPartner {
			fmt.Printf("%3d %-20s %-20s %d–%d then %q of %q with %q clash %q conf %.2f\n", i+1, g.AName, g.BName, g.FirstYear, g.LastYear, g.Then, g.ThenOf, g.ThenWith, g.Clash, g.Confidence)
		}
	}
}
