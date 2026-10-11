package store

import (
	"context"
	"math/rand/v2"
	"slices"
	"testing"
	"time"
)

// Everyone nobody has named at an event (performers, an audience) is
// hidden in one go; named people stay, and someone also seen elsewhere
// stays there.
func TestHideEventStrangers(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(81, 82))
	ctx := context.Background()
	ann, knight, fan := identity(r), identity(r), identity(r)
	at := func(d, hour int) time.Time { return day(2015, d).Add(time.Duration(hour) * time.Hour) }
	var a1 int64
	for i := range 3 {
		f := h.photo(at(1, 9+i), variant(r, ann, 0.02), 30)
		if a1 == 0 {
			a1 = f
		}
	}
	pa, _ := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	// The event: from 10 to 14 o'clock, an hour apart; Ann is there too.
	var knights, fans []int64
	for i := range 5 {
		knights = append(knights, h.photo(at(1, 10+i), variant(r, knight, 0.02), 40))
		fans = append(fans, h.photo(at(1, 10+i), variant(r, fan, 0.02), 25))
	}
	annThere := h.photo(at(1, 12), variant(r, ann, 0.02), 30)
	h.s.FileFaces(ctx) // Ann at the event, filed under her
	// The fan, a week later (another event).
	later := h.photo(at(8, 12), variant(r, fan, 0.02), 25)
	if h.group(later) != h.group(fans[0]) {
		t.Fatal("test setup: the fan's photos are in different groups")
	}
	img := h.s.faces[knights[2]].ImageID
	ev, err := h.s.EventStrangers(ctx, img, true)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Faces != 10 {
		t.Errorf("preview: %d unnamed faces, want 10 (%d photos)", ev.Faces, ev.Photos)
	}
	// Ann's 3 photos that morning, 10 at the event, Ann there: one face each.
	if ev.Photos != 14 || len(ev.Images) != 14 || ev.Unnamed[img] != 1 {
		t.Errorf("details: %d photos, %d listed, %d unnamed in the photo", ev.Photos, len(ev.Images), ev.Unnamed[img])
	}
	// A photo left out, and a face kept, stay as they are.
	skipImg, keep := h.s.faces[knights[0]].ImageID, fans[1]
	ev, err = h.s.HideEventStrangers(ctx, img, []int64{skipImg}, []int64{keep})
	if err != nil {
		t.Fatal(err)
	}
	hidden := func(f int64) bool { return h.s.clusters[h.group(f)].Hidden }
	for _, f := range append(slices.Clone(knights[1:]), fans[0], fans[2], fans[3], fans[4]) {
		if !hidden(f) {
			t.Errorf("face %d at the event not hidden", f)
		}
	}
	if hidden(knights[0]) || hidden(keep) {
		t.Error("a photo left out, or a face kept, was hidden")
	}
	if hidden(later) {
		t.Error("the fan a week later was hidden too")
	}
	if c := h.s.clusters[h.group(annThere)]; c.Hidden || c.PersonID != pa {
		t.Error("Ann, named, was hidden")
	}
	if err := h.s.UnhideGroups(ctx, ev.Groups); err != nil {
		t.Fatal(err)
	}
	if hidden(knights[1]) || hidden(fans[2]) {
		t.Error("undo left faces hidden")
	}
}
