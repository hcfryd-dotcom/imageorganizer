package store

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestEvalRelate applies relations to a database copy and prints people's
// family after: IO_EVAL_REL="A|kind|B;…" (kind "-x" removes) and
// IO_EVAL_SHOW="A;B".
func TestEvalRelate(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" || os.Getenv("IO_EVAL_REL") == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_REL")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	id := map[string]int64{}
	for pid, p := range s.people {
		id[p.Name] = pid
	}
	for _, r := range strings.Split(os.Getenv("IO_EVAL_REL"), ";") {
		f := strings.Split(r, "|")
		on := !strings.HasPrefix(f[1], "-")
		if err := s.SetRelation(context.Background(), id[f[0]], id[f[2]], strings.TrimPrefix(f[1], "-"), on); err != nil {
			t.Fatalf("%s: %v", r, err)
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	fam := s.family()
	for _, n := range strings.Split(os.Getenv("IO_EVAL_SHOW"), ";") {
		var out []string
		for q, k := range fam[id[n]] {
			out = append(out, s.people[q].Name+" ("+kinWord(k, "")+")")
		}
		fmt.Printf("%s: %s\n", n, strings.Join(out, ", "))
	}
}
