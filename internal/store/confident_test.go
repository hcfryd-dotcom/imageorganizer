package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

func TestConfidentReview(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(31, 32))
	ctx := context.Background()
	ann, bob := identity(r), identity(r)
	// Ann named from two photos at 30; more of her twelve years on form
	// another group (another period of life), still clearly her.
	a1 := h.photo(day(2018, 1), variant(r, ann, 0.02), 30)
	h.photo(day(2018, 2), variant(r, ann, 0.02), 30)
	pid, err := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	later := h.photo(day(2030, 1), variant(r, ann, 0.02), 42)
	h.photo(day(2030, 2), variant(r, ann, 0.02), 42)
	stranger := h.photo(day(2019, 1), variant(r, bob, 0.02), 50)
	h.photo(day(2019, 2), variant(r, bob, 0.02), 50)
	if h.group(later) == h.group(a1) {
		t.Fatal("setup: the later photos should be another group")
	}

	picks, _ := h.s.ConfidentReview(ctx, 2, 0.75)
	var got *ConfidentPick
	for i, p := range picks {
		if p.Group == h.group(stranger) {
			t.Fatalf("an unknown person was picked: %+v", p)
		}
		if p.Group == h.group(later) {
			got = &picks[i]
		}
	}
	if got == nil || got.Person != pid || got.Name != "Ann" || got.Score < 0.75 || got.Faces != 2 {
		t.Fatalf("later Ann group not picked as Ann: %+v (all %+v)", got, picks)
	}

	// Nothing is that alike at 100%.
	if picks, _ := h.s.ConfidentReview(ctx, 2, 1.0); len(picks) != 0 {
		t.Fatalf("picked at 100%%: %+v", picks)
	}

	// Filing it leaves it out of the next round.
	if _, err := h.s.NameGroup(ctx, got.Group, got.Person, ""); err != nil {
		t.Fatal(err)
	}
	picks, _ = h.s.ConfidentReview(ctx, 2, 0.75)
	for _, p := range picks {
		if p.Group == got.Group {
			t.Fatal("a filed group was picked again")
		}
	}
}

// Two people a face is as alike to (siblings) are left for the user.
func TestConfidentReviewAmbiguous(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(41, 42))
	ctx := context.Background()
	kid := identity(r)
	a := h.photo(day(2018, 1), variant(r, kid, 0.02), 10)
	h.photo(day(2018, 2), variant(r, kid, 0.02), 10)
	if _, err := h.s.NameGroup(ctx, h.group(a), 0, "Ann"); err != nil {
		t.Fatal(err)
	}
	b := h.photo(day(2022, 1), variant(r, kid, 0.02), 10) // born four years later: not Ann
	h.photo(day(2022, 2), variant(r, kid, 0.02), 10)
	if h.group(b) == h.group(a) {
		t.Fatal("setup: the younger sibling should be another group")
	}
	if _, err := h.s.NameGroup(ctx, h.group(b), 0, "Bea"); err != nil {
		t.Fatal(err)
	}
	c := h.photo(day(2020, 1), variant(r, kid, 0.02), -1) // no age: could be either
	h.photo(day(2020, 2), variant(r, kid, 0.02), -1)
	picks, amb := h.s.ConfidentReview(ctx, 2, 0.75)
	for _, p := range picks {
		if p.Group == h.group(c) {
			t.Fatalf("a face as alike to two people was picked: %+v", p)
		}
	}
	if amb == 0 {
		t.Fatal("the sibling group should count as ambiguous")
	}
}
