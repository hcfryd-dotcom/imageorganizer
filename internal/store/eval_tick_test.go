package store

import (
	"context"
	"os"
	"testing"
)

// TestEvalTick confirms one automatically filed face, as the photo's tick
// does, and counts the faces elsewhere whose tick (filed, not confirmed)
// comes or goes: at once, and after the learning that follows.
//
//	IO_EVAL_DB=<copy> go test -run TestEvalTick -v
func TestEvalTick(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	ticks := func() map[int64]bool {
		s.mu.RLock()
		defer s.mu.RUnlock()
		out := map[int64]bool{}
		for _, f := range s.faces {
			if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 && f.Tagged != c.PersonID {
				out[f.ID] = true
			}
		}
		return out
	}
	diff := func(a, b map[int64]bool) (came, went int) {
		for id := range b {
			if !a[id] {
				came++
			}
		}
		for id := range a {
			if !b[id] {
				went++
			}
		}
		return
	}
	before := ticks()
	var pick *faceRec
	s.mu.RLock()
	for _, f := range s.faces {
		if before[f.ID] && f.Tagged == 0 {
			pick = f
			break
		}
	}
	pid := s.clusters[pick.ClusterID].PersonID
	s.mu.RUnlock()
	t.Logf("ticks %d; confirming face %d as %d", len(before), pick.ID, pid)
	if _, err := s.TagFace(ctx, pick.ID, pid, ""); err != nil {
		t.Fatal(err)
	}
	now := ticks()
	came, went := diff(before, now)
	t.Logf("at once: ticks %d, came %d, went %d", len(now), came, went)
	// Confirmed faces showing a tick (filed under someone else):
	s.mu.RLock()
	wrong := 0
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && f.Tagged != 0 && c.PersonID != f.Tagged {
			wrong++
		}
	}
	s.mu.RUnlock()
	t.Logf("confirmed faces in a group not theirs: %d", wrong)
	filed, unfiled, _ := s.FileFaces(ctx)
	after := ticks()
	came, went = diff(now, after)
	t.Logf("after learning (filed %d, unfiled %d): ticks %d, came %d, went %d", filed, unfiled, len(after), came, went)
}
