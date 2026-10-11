package store

import (
	"cmp"
	"context"
	"os"
	"slices"
	"testing"
	"time"
)

func TestEvalCandidatesSpeed(t *testing.T) {
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
	s.personIndex()
	s.mu.RUnlock()
	t.Logf("personIndex %v", time.Since(st))
	st = time.Now()
	s.Candidates(context.Background(), 1, 5, 4)
	t.Logf("Candidates %v", time.Since(st))
	st = time.Now()
	s.People(context.Background())
	t.Logf("People %v", time.Since(st))
}

// TestEvalCandidatesSame checks the fast suggestions against idx.score.
func TestEvalCandidatesSame(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for pid := range s.people {
		got := s.Candidates(context.Background(), pid, 20, 1)
		s.mu.RLock()
		idx := s.personIndex()
		type cs struct {
			id    int64
			score float32
		}
		var want []cs
		for _, c := range s.clusters {
			if c.PersonID != 0 || c.Hidden || c.n == 0 {
				continue
			}
			if sc, ok := idx.score(c, pid); ok && sc >= candidateMin && (c.n > 1 || sc >= singleCandidate) {
				want = append(want, cs{c.id, sc})
			}
		}
		s.mu.RUnlock()
		slices.SortFunc(want, func(a, b cs) int { return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.id, b.id)) })
		want = want[:min(20, len(want))]
		if len(got) != len(want) {
			t.Errorf("person %d: %d candidates, want %d", pid, len(got), len(want))
			continue
		}
		for i := range got {
			if got[i].ID != want[i].id {
				t.Errorf("person %d #%d: group %d (%.4f), want %d (%.4f)", pid, i, got[i].ID, got[i].Score, want[i].id, want[i].score)
				break
			}
		}
	}
}
