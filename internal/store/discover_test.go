package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// Discover asks "Is this also …?" for a group like a named person at
// another age; No keeps it apart; strangers stay unnamed rows.
func TestDiscoverAlso(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(151, 152))
	ctx := context.Background()
	kid, stranger := identity(r), identity(r)
	grown := variant(r, kid, 0.055) // the same face, ten years on: alike enough to ask, not to file
	var young, old, other int64
	for i := range 8 {
		f := h.photo(day(2005, 1+i), variant(r, kid, 0.01), 8)
		g := h.photo(day(2015, 1+i), variant(r, grown, 0.01), 18)
		o := h.photo(day(2010, 1+i), variant(r, stranger, 0.01), 40)
		if i == 0 {
			young, old, other = f, g, o
		}
	}
	if h.group(young) == h.group(old) {
		t.Skip("the two ages were grouped together: nothing to chain")
	}
	pid, err := h.s.NameGroup(ctx, h.group(young), 0, "Kid")
	if err != nil {
		t.Fatal(err)
	}
	var also, unnamed bool
	for _, ch := range h.s.DiscoverPeople(ctx, 10) {
		for _, g := range ch.Groups {
			if g == h.group(old) {
				also = ch.Person == pid && ch.PersonName == "Kid"
			}
			if g == h.group(other) {
				unnamed = ch.Person == 0
			}
		}
	}
	if !also || !unnamed {
		t.Fatalf("also Kid %v, stranger unnamed %v", also, unnamed)
	}
	if err := h.s.RejectGroup(ctx, h.group(old), pid); err != nil {
		t.Fatal(err)
	}
	for _, ch := range h.s.DiscoverPeople(ctx, 10) {
		if ch.Person == pid {
			t.Errorf("asked again after No: %+v", ch)
		}
	}
}

// The very same face over many years is a picture, not a person who ages.
func TestDiscoverStill(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(161, 162))
	ctx := context.Background()
	poster, person := identity(r), identity(r)
	for i := range 12 {
		h.photo(day(2006+i, 1), variant(r, poster, 0.01), 39)
		person = variant(r, person, 0.02) // a living face changes, year by year
		h.photo(day(2006+i, 2), variant(r, person, 0.01), float32(30+i))
	}
	var still, alive int
	for _, ch := range h.s.DiscoverPeople(ctx, 10) {
		if ch.Still {
			still++
		} else {
			alive++
		}
	}
	if still != 1 || alive != 1 {
		t.Fatalf("%d still, %d not", still, alive)
	}
}
