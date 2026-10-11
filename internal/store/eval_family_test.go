package store

import (
	"fmt"
	"os"
	"testing"
)

// TestEvalFamily counts the relations worked out from those entered on a
// database copy, by kind, and lists the more distant ones.
func TestEvalFamily(t *testing.T) {
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
	fam, in := s.family(), s.entered()
	count := map[string]int{}
	for a, rel := range fam {
		for b, k := range rel {
			if _, ok := in[a][b]; ok {
				count["entered "+k]++
				continue
			}
			count[k]++
			switch k {
			case KinParent, KinChild, KinSibling, KinGrandchild, KinNieceNephew, KinStepChild:
			default:
				fmt.Printf("%-22s is %-22s's %s\n", s.people[b].Name, s.people[a].Name, k)
			}
		}
	}
	t.Logf("%v", count)
}
