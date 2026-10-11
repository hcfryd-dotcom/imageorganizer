package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// A picture of a child on the wall: the same face at 6 in photos from
// years when the child was much older. It is found, with the picture of
// their mother beside it on the wall; a "yes" files them as pictures.
func TestPictureOnTheWall(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(191, 192))
	ctx := context.Background()
	kid, mum := identity(r), identity(r)
	// The child, born 2010, photographed growing up; confirmed.
	var k1 int64
	for y := 2012; y <= 2023; y++ {
		f := h.photo(day(y, 1), variant(r, kid, 0.03), float32(y-2010))
		if k1 == 0 {
			k1 = f
		}
	}
	pk, _ := h.s.NameGroup(ctx, h.group(k1), 0, "Kid")
	m1 := h.photo(day(2012, 2), variant(r, mum, 0.02), 32)
	pm, _ := h.s.NameGroup(ctx, h.group(m1), 0, "Mum")
	// The wall: the child at 6 and the mother at 34, side by side, in
	// photos from 2017 to 2023.
	atSix := variant(r, kid, 0.03)
	var wall []int64
	for y := 2017; y <= 2023; y++ {
		a, _ := h.side(day(y, 10), variant(r, atSix, 0.02), variant(r, mum, 0.02), 6, 34)
		wall = append(wall, a)
	}
	sets := h.s.FrozenFor(ctx, pk)
	if len(sets) != 1 {
		t.Fatalf("%d pictures found, want 1", len(sets))
	}
	fz := sets[0]
	got := map[int64]bool{}
	for _, f := range fz.Faces {
		got[f.ID] = true
	}
	for _, f := range wall {
		if !got[f] {
			t.Errorf("wall face %d not in the picture", f)
		}
	}
	if len(fz.With) != 1 || fz.With[0].Person != pm || len(fz.With[0].Faces) != len(wall) {
		t.Fatalf("others on the wall: %+v", fz.With)
	}
	// Yes: they're pictures now, and not found again.
	var ids []int64
	for _, f := range fz.Faces {
		ids = append(ids, f.ID)
	}
	if err := h.s.SetFrozen(ctx, pk, ids, true); err != nil {
		t.Fatal(err)
	}
	if !h.s.faces[wall[0]].Picture {
		t.Error("not marked as a picture")
	}
	if sets := h.s.FrozenFor(ctx, pk); len(sets) != 0 {
		t.Errorf("found again after yes: %d", len(sets))
	}
}
