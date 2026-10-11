package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

func TestExplainFace(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(13, 14))
	ctx := context.Background()
	ann := identity(r)
	var fs []int64
	for i := range 4 {
		fs = append(fs, h.photo(day(2020, 1+i), variant(r, ann, 0.02), 30))
	}
	pid, err := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.TagFace(ctx, fs[1], pid, ""); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"named", "checked"} {
		w, err := h.s.ExplainFace(ctx, fs[i], nil)
		if err != nil {
			t.Fatal(err)
		}
		if w.Strength != want || w.Face.Strength != want {
			t.Errorf("face %d: strength %q (view %q), want %q", i, w.Strength, w.Face.Strength, want)
		}
		if len(w.People) == 0 || w.People[0].Person != pid || w.People[0].Like <= 0 {
			t.Errorf("face %d: people %+v, want Ann first", i, w.People)
		}
	}
	if _, err := h.s.ExplainFace(ctx, 999999, nil); err != ErrNotFound {
		t.Errorf("unknown face: %v", err)
	}
}

// A person's page asks only about faces that stand out: not those that
// match well, but those less like them than when the user confirmed them.
func TestPersonDoubtsOutliers(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(15, 16))
	ctx := context.Background()
	ann := identity(r)
	var fs []int64
	for i := range 6 {
		fs = append(fs, h.photo(day(2020, 1+i), variant(r, ann, 0.02), 30))
	}
	pid, err := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fs {
		if h.s.clusters[h.group(f)].PersonID == 0 {
			h.s.NameGroup(ctx, h.group(f), pid, "")
		}
	}
	if n, err := h.s.RecordApprovals(ctx); err != nil || n != len(fs) {
		t.Fatalf("approvals %d, %v; want %d", n, err, len(fs))
	}
	if ds := h.s.PersonDoubts(ctx, pid, 0); len(ds) != 0 {
		t.Errorf("faces that match well asked about: %+v", ds)
	}
	// As if it matched far better when confirmed than it does now.
	h.s.mu.Lock()
	h.s.faces[fs[2]].Approved.Score += 0.5
	h.s.mu.Unlock()
	ds := h.s.PersonDoubts(ctx, pid, 0)
	if len(ds) != 1 || ds[0].Face.ID != fs[2] || ds[0].Then == 0 || ds[0].Strength != "named" {
		t.Errorf("doubts %+v, want the face that matched better when confirmed", ds)
	}
	if n, _ := h.s.RecordApprovals(ctx); n != 0 {
		t.Errorf("recorded %d approvals again", n)
	}
}
