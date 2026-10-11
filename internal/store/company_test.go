package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// Someone photographed with a named person on several days over the
// years comes first in "people you probably know"; a stranger seen with
// them once, at one event, comes after.
func TestReviewKnownCompany(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(101, 102))
	ctx := context.Background()
	ann, friend, stranger := identity(r), identity(r), identity(r)
	var a1 int64
	for i := range 4 {
		f := h.photo(day(2010, 1+i), variant(r, ann, 0.02), 30)
		if a1 == 0 {
			a1 = f
		}
	}
	pa, _ := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	// The friend: with Ann on three days, in 2010, 2011 and 2012.
	var fr int64
	for i, y := range []int{2010, 2011, 2012} {
		_, f := h.side(day(y, 20+i), variant(r, ann, 0.02), variant(r, friend, 0.02), 30+float32(y-2010), 30+float32(y-2010))
		fr = f
	}
	// The stranger: with Ann once, and on their own at that event, more
	// often than the friend is seen at all.
	_, st := h.side(day(2013, 5), variant(r, ann, 0.02), variant(r, stranger, 0.02), 33, 50)
	for range 5 {
		h.photo(day(2013, 5), variant(r, stranger, 0.02), 50)
	}
	h.s.FileFaces(ctx) // Ann beside them, filed under her
	if c := h.s.clusters[h.group(fr)]; c.PersonID == pa {
		t.Fatal("test setup: the friend was filed as Ann")
	}
	items, _ := h.s.Review(ctx, 1, 0, 20, 4, ReviewKnown)
	pos := map[int64]int{}
	for i, it := range items {
		pos[it.ID] = i + 1
	}
	gf, gs := h.group(fr), h.group(st)
	if pos[gf] == 0 || pos[gs] == 0 || pos[gf] > pos[gs] {
		t.Fatalf("friend at %d, stranger at %d; want the friend first", pos[gf], pos[gs])
	}
	k := items[pos[gf]-1].Company
	if k == nil || k.Days != 3 || k.From != 2010 || k.To != 2012 || len(k.With) != 1 || k.With[0] != "Ann" {
		t.Errorf("friend's company %+v", k)
	}
}
