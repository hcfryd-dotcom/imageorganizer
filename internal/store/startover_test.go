package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// Starting over forgets who is who and keeps the analysis.
func TestStartOver(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(201, 202))
	ctx := context.Background()
	ann, bob := identity(r), identity(r)
	var a1, b1 int64
	for i := range 4 {
		a := h.photo(day(2010, 1+i), variant(r, ann, 0.02), 30)
		b := h.photo(day(2010, 10+i), variant(r, bob, 0.02), 50)
		if i == 0 {
			a1, b1 = a, b
		}
	}
	pa, _ := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	pb, _ := h.s.NameGroup(ctx, h.group(b1), 0, "Bob")
	h.s.MarkSeen(ctx, pa, []int64{a1})
	h.s.SetRelation(ctx, pa, pb, KinPartner, true)
	h.s.SetFacePicture(ctx, b1, true)
	if err := h.s.StartOver(ctx); err != nil {
		t.Fatal(err)
	}
	if len(h.s.people) != 0 {
		t.Errorf("%d people left", len(h.s.people))
	}
	for _, f := range h.s.faces {
		c := h.s.clusters[f.ClusterID]
		if c == nil || c.PersonID != 0 || f.Tagged != 0 || f.Picture {
			t.Fatalf("face %d: group %v tagged %d picture %v", f.ID, c, f.Tagged, f.Picture)
		}
		if len(f.emb) == 0 || f.AgeVersion == 0 {
			t.Fatalf("face %d lost its analysis", f.ID)
		}
	}
	if h.group(a1) == h.group(b1) || h.group(a1) != h.group(a1+2) {
		t.Error("faces not grouped afresh as before")
	}
}
