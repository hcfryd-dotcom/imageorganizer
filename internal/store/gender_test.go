package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// A person's gender is estimated from their confirmed adult faces (or
// entered); a face that clearly reads as the other gender is ranked lower
// for them, and says so; children's faces don't count.
func TestGender(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(151, 152))
	ctx := context.Background()
	bob := identity(r)
	var b1 int64
	for i := range 6 {
		f := h.photo(day(2010, 1+i), variant(r, bob, 0.02), 40)
		if err := h.s.SetGender(ctx, f, 0.02); err != nil {
			t.Fatal(err)
		}
		if b1 == 0 {
			b1 = f
		}
	}
	pb, _ := h.s.NameGroup(ctx, h.group(b1), 0, "Bob")
	h.s.mu.RLock()
	g := h.s.personIndex()[pb].gender
	h.s.mu.RUnlock()
	if g != Male {
		t.Fatalf("Bob's estimated gender %q", g)
	}
	// A face like Bob's that reads as a woman: still suggested, lower,
	// saying why.
	odd := h.photo(day(2011, 1), variant(r, bob, 0.06), 40)
	h.s.SetGender(ctx, odd, 0.97)
	var got *Suggestion
	for _, sg := range h.s.FaceSuggestions(ctx, odd) {
		if sg.ID == pb {
			got = &sg
		}
	}
	h.s.mu.RLock()
	like := dot(unitVec(h.s.faces[odd].emb), clusterUnit(h.s.clusters[h.group(b1)]))
	h.s.mu.RUnlock()
	if got == nil || got.Off != "reads as a woman" || got.Score > like-genderCost+1e-3 {
		t.Errorf("Bob for a face reading as a woman: %+v", got)
	}
	// A child's face says nothing; nor does an unsure one.
	for _, f := range []*faceRec{{Age: 8, Fem: 0.97, FemV: 1}, {Age: 40, Fem: 0.6, FemV: 1}, {Age: 40, Fem: 0.97}} {
		if genderMismatch(Male, f) {
			t.Errorf("face %+v counted against a man", *f)
		}
	}
	if !genderMismatch(Male, &faceRec{Age: 40, Fem: 0.97, FemV: 1}) {
		t.Error("a clear woman's face not counted against a man")
	}
	// Entered, it wins over the faces.
	female := Female
	if err := h.s.UpdatePerson(ctx, pb, PersonEdit{Gender: &female}); err != nil {
		t.Fatal(err)
	}
	h.s.mu.RLock()
	g = h.s.personIndex()[pb].gender
	h.s.mu.RUnlock()
	if g != Female {
		t.Errorf("entered gender ignored: %q", g)
	}
	if w := kinWord(KinParent, Female); w != "mother" {
		t.Errorf("a female parent is a %q", w)
	}
	if w := kinWord(KinPartner, Male); w != "partner" {
		t.Errorf("a male partner is a %q", w)
	}
}
