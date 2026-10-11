package store

import (
	"os"
	"testing"
)

// TestEvalAgeLoose: what the looser age check for very alike adults
// changes on a database copy: the Spain face's suggestions, and how many
// unnamed groups would now be filed automatically.
func TestEvalAgeLoose(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx := s.confirmedIndex()
	minSim := s.autoMinLocked()
	if f := s.faces[92928]; f != nil {
		c := s.clusters[f.ClusterID]
		for pid, pi := range idx {
			if sc, ok := idx.score(c, pid); ok && sc >= candidateMin {
				t.Logf("spain face's group: %s %.2f", pi.p.Name, sc)
			}
		}
	}
	strict, loose := 0, 0
	for _, c := range s.clusters {
		if c.PersonID != 0 || c.Hidden || c.n == 0 {
			continue
		}
		pid, sc := idx.best(c)
		if pid == 0 || sc < minSim {
			continue
		}
		loose++
		cb, ca, cok := c.birth()
		if idx[pid].ages.fits(cb, ca, cok) {
			strict++
		} else {
			t.Logf("newly filed: group %d (%d faces, looks %.0f) as %s, %.2f alike", c.id, c.n, ca, idx[pid].p.Name, sc)
		}
	}
	t.Logf("auto-file threshold %.2f: %d unnamed groups pass strictly, %d loosely", minSim, strict, loose)
	var cs []*cluster
	for _, c := range s.clusters {
		if c.PersonID == 0 && !c.Hidden && c.n > 0 {
			cs = append(cs, c)
		}
	}
	s.mu.RUnlock()
	s.mu.Lock()
	n := len(s.autoAssign(cs)) // in memory only: nothing is saved
	s.mu.Unlock()
	s.mu.RLock()
	t.Logf("autoAssign would file %d", n)
}
