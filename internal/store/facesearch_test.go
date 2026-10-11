package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// A face from outside the library finds the person's group first, with
// its faces most alike first, and not someone of a clearly other age.
func TestSimilarFaces(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(91, 92))
	ctx := context.Background()
	ann, bob := identity(r), identity(r)
	var a1 int64
	for i := range 4 {
		f := h.photo(day(2010, 1+i), variant(r, ann, 0.02), 30)
		h.photo(day(2010, 10+i), variant(r, bob, 0.02), 50)
		if a1 == 0 {
			a1 = f
		}
	}
	pa, _ := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	gs := h.s.SimilarFaces(ctx, variant(r, ann, 0.02), 31, 10)
	if len(gs) == 0 || gs[0].Person != pa || gs[0].Name != "Ann" || len(gs[0].Faces) != 4 {
		t.Fatalf("results %+v, want Ann's group first with 4 faces", gs)
	}
	if f := gs[0].Faces; f[0].Score < f[len(f)-1].Score {
		t.Error("faces not most alike first")
	}
	for _, g := range gs {
		if g.Group == h.group(a1) {
			continue
		}
		if g.Score > gs[0].Score {
			t.Errorf("group %d more alike than Ann's", g.Group)
		}
	}
	// Ann at 30 searched as a 5-year-old: her group is left out.
	if gs := h.s.SimilarFaces(ctx, variant(r, ann, 0.02), 5, 10); len(gs) > 0 && gs[0].Person == pa {
		t.Error("a group 25 years off in age offered")
	}
}
