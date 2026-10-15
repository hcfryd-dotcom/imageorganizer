package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// Faces wait for their eyes to be judged once; a face that couldn't be
// judged (-1) counts as done too, and the openness shows on the face.
func TestEyes(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(7, 8))
	ctx := context.Background()
	a := h.photo(day(2020, 1), identity(r), 1)
	b := h.photo(day(2020, 2), identity(r), 2)
	if done, total := h.s.EyesStats(); done != 0 || total != 2 {
		t.Fatalf("before: %d of %d", done, total)
	}
	jobs := h.s.NeedEyes(ctx, 10, nil)
	if len(jobs) != 2 {
		t.Fatalf("jobs: %+v", jobs)
	}
	if err := h.s.SetEyes(ctx, a, .31); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SetEyes(ctx, b, -1); err != nil {
		t.Fatal(err)
	}
	if done, _ := h.s.EyesStats(); done != 2 || len(h.s.NeedEyes(ctx, 10, nil)) != 0 {
		t.Fatalf("after: %d done", done)
	}
	fs := h.s.ImageFaces(ctx, h.s.faces[a].ImageID)
	if len(fs) != 1 || fs[0].Eyes != float32(.31) {
		t.Fatalf("view: %+v", fs)
	}
}
