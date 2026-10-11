package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestEvalFrozenSets lists the pictures FrozenFor finds for IO_EVAL_PERSON on a
// database copy, with whoever else is in them.
func TestEvalFrozenSets(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var pid int64
	for id, p := range s.people {
		if p.Name == os.Getenv("IO_EVAL_PERSON") {
			pid = id
		}
	}
	st := time.Now()
	sets := s.FrozenFor(context.Background(), pid)
	t.Logf("%d sets in %v", len(sets), time.Since(st).Round(time.Millisecond))
	var crops []byte
	for _, fz := range sets {
		t.Logf("  %d faces, %d photos, %d–%d, age %.1f", len(fz.Faces), fz.Photos, time.Unix(fz.From, 0).Year(), time.Unix(fz.To, 0).Year(), fz.Age)
		for _, w := range fz.With {
			t.Logf("    with %q (%d): %d faces", w.Name, w.Person, len(w.Faces))
			for _, f := range w.Faces {
				crops = append(crops, []byte(w.Name+" "+f.Crop+"\n")...)
			}
		}
	}
	if out := os.Getenv("IO_EVAL_OUT"); out != "" {
		os.WriteFile(out, crops, 0o644)
	}
}

func TestEvalFrozenAll(t *testing.T) {
	path, out := os.Getenv("IO_EVAL_DB"), os.Getenv("IO_EVAL_OUT")
	if path == "" || out == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_OUT")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var pid int64
	for id, p := range s.people {
		if p.Name == os.Getenv("IO_EVAL_PERSON") {
			pid = id
		}
	}
	var b []byte
	for _, fz := range s.FrozenFor(context.Background(), pid) {
		for _, f := range fz.Faces {
			b = append(b, []byte(fmt.Sprintf("%d %.1f %s\n", time.Unix(*f.Taken, 0).Year(), f.Age, f.Crop))...)
		}
	}
	os.WriteFile(out, b, 0o644)
}
