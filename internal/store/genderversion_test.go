package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// A face found of unknown gender by an age model without gender is
// judged again when a gender model comes (GenderVersion 2), once.
func TestGenderVersion(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(9, 10))
	ctx := context.Background()
	f := h.photo(day(2020, 1), identity(r), 40)
	h.s.Gender = true
	if err := h.s.SetGender(ctx, f, -1); err != nil { // the age model's "unknown"
		t.Fatal(err)
	}
	if n := len(h.s.NeedAge(ctx, 10, nil)); n != 0 {
		t.Fatalf("version 1: %d jobs", n)
	}
	h.s.GenderVersion = 2
	jobs := h.s.NeedAge(ctx, 10, nil)
	if len(jobs) != 1 || !jobs[0].GenderOnly {
		t.Fatalf("version 2: %+v", jobs)
	}
	if err := h.s.SetGender(ctx, f, .9); err != nil {
		t.Fatal(err)
	}
	if n := len(h.s.NeedAge(ctx, 10, nil)); n != 0 || h.s.faces[f].Fem != .9 {
		t.Fatalf("after: %d jobs, fem %v", n, h.s.faces[f].Fem)
	}
}
