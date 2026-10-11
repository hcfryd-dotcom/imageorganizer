package store

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"
)

// A small child looks little like their older self, but is photographed
// on the days their parents are: their faces on those days are offered,
// if their age fits, even when only a bit alike. The household whose
// photos these are is on every day, so it says nothing.
func TestFamilyDayCandidates(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(111, 112))
	ctx := context.Background()
	mum, home, kid := identity(r), identity(r), identity(r)
	at := func(y, d int) time.Time { return day(y, d).Add(12 * time.Hour) }
	name := func(f int64, n string) int64 {
		t.Helper()
		pid, err := h.s.NameGroup(ctx, h.group(f), 0, n)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	// The household, on every day in these years (alone, too).
	var home1 int64
	for y := 2010; y <= 2020; y++ {
		for d := 1; d <= 28; d += 3 {
			f := h.photo(at(y, d), variant(r, home, 0.02), float32(y-1975))
			if home1 == 0 {
				home1 = f
			}
		}
	}
	name(home1, "Home")
	// The child at 10-11, with mum, on a few days: confirmed.
	var kid1, mum1 int64
	for i := range 12 {
		y := 2019 + i%2
		k, m := h.side(at(y, 2+i), variant(r, kid, 0.02), variant(r, mum, 0.02), float32(y-2009), float32(y-1980))
		if kid1 == 0 {
			kid1, mum1 = k, m
		}
	}
	pk := name(kid1, "Kid")
	pm := name(mum1, "Mum")
	// Mum, in 2012; on one of those days the child at 3, alone, looking
	// rather different from her older self.
	little := variant(r, kid, 0.15)
	if _, err := h.s.TagFace(ctx, h.photo(at(2012, 4), variant(r, mum, 0.02), 32), pm, ""); err != nil {
		t.Fatal(err)
	}
	small := h.photo(at(2012, 4), variant(r, little, 0.02), 3)
	// The same small face on a day only the household was photographed.
	homeDay := h.photo(at(2012, 13), variant(r, little, 0.02), 3)
	if c := h.s.clusters[h.group(small)]; c.PersonID != 0 {
		t.Fatalf("test setup: the small child was filed as %d", c.PersonID)
	}
	sim := dot(unitVec(h.s.faces[small].emb), clusterUnit(h.s.clusters[h.group(kid1)]))
	if sim >= faceCandMin || sim < familySim {
		t.Fatalf("test setup: the small child is %.2f alike; want between %.2f and %.2f", sim, familySim, faceCandMin)
	}
	got := map[int64]FaceView{}
	for _, v := range h.s.FaceCandidates(ctx, pk, 50) {
		got[v.ID] = v
	}
	// A face is only offered to whom its photo's card, looking at the face
	// alone, offers first: too unlike the child, it isn't, mum's day or not.
	if _, ok := got[small]; ok {
		t.Errorf("the small child on mum's day offered, though only %.2f alike: its card doesn't offer her", sim)
	}
	if _, ok := got[homeDay]; ok {
		t.Error("the small child on a household-only day offered")
	}
}
