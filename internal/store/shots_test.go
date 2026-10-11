package store

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"
)

// Near-identical shots: the face at the same place counts for the person
// it is, against anyone else, and two shots confirmed as two people are
// asked about until settled.
func TestShots(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(301, 302))
	ctx := context.Background()
	scene := identity(r) // the picture as a whole
	at := day(2020, 1)
	shot := func(emb []float32, sec int) int64 {
		f := h.photo(at.Add(time.Duration(sec)*time.Second), emb, 30)
		h.s.vectors[h.s.faces[f].ImageID] = unitVec(variant(r, scene, 0.01))
		return f
	}
	a, b := identity(r), identity(r)
	fa := shot(a, 0)
	fb := shot(variant(r, a, 0.3), 5) // the same person, hardly alike this time
	other := shot(b, 3600)            // an hour on: not a burst
	pa, err := h.s.TagFace(ctx, fa, 0, "A")
	if err != nil {
		t.Fatal(err)
	}
	pb, err := h.s.TagFace(ctx, other, 0, "B")
	if err != nil {
		t.Fatal(err)
	}
	h.s.mu.RLock()
	m := h.s.newMatcher()
	got, gotB, gotOther := m.shot(h.s.faces[fb], pa), m.shot(h.s.faces[fb], pb), m.shot(h.s.faces[other], pa)
	h.s.mu.RUnlock()
	if got != shotBonus || gotB != -shotBonus || gotOther != 0 {
		t.Fatalf("shot evidence: for A %.2f, for B %.2f, an hour on %.2f", got, gotB, gotOther)
	}
	// Confirmed as two people: asked, until said to be two people.
	if _, err := h.s.TagFace(ctx, fb, pb, ""); err != nil {
		t.Fatal(err)
	}
	if cs := h.s.ShotConflicts(ctx); len(cs) != 1 || cs[0].APerson == cs[0].BPerson {
		t.Fatalf("conflicts %+v", cs)
	}
	if err := h.s.AnswerShot(ctx, fa, fb, 0); err != nil {
		t.Fatal(err)
	}
	if cs := h.s.ShotConflicts(ctx); len(cs) != 0 {
		t.Fatalf("asked again: %+v", cs)
	}
	// Or both one person: the other face moves to them.
	h.s.faces[fa].Apart, h.s.faces[fb].Apart = nil, nil
	if err := h.s.AnswerShot(ctx, fa, fb, pa); err != nil {
		t.Fatal(err)
	}
	if f := h.s.faces[fb]; f.Tagged != pa {
		t.Fatalf("face b confirmed as %d, want %d", f.Tagged, pa)
	}
}
