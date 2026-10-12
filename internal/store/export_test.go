package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

func TestPersonExport(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(21, 22))
	ctx := context.Background()
	ann, bob := identity(r), identity(r)
	a2018 := h.photo(day(2018, 1), variant(r, ann, 0.02), 30)
	a2015 := h.photo(day(2015, 1), variant(r, ann, 0.02), 27)
	h.photo(day(2016, 1), variant(r, bob, 0.02), 40)
	if h.group(a2018) != h.group(a2015) {
		t.Fatal("setup: Ann's faces should share a group")
	}
	pid, err := h.s.NameGroup(ctx, h.group(a2018), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}

	name, items, err := h.s.PersonExport(ctx, pid, false)
	if err != nil {
		t.Fatal(err)
	}
	if name != "Ann" || len(items) != 2 {
		t.Fatalf("got %q with %d photos, want Ann with 2", name, len(items))
	}
	img := func(fid int64) int64 { return h.s.faces[fid].ImageID }
	if items[0].ImageID != img(a2015) || items[1].ImageID != img(a2018) {
		t.Fatalf("photos not oldest first: %+v", items)
	}
	for _, it := range items {
		if it.Taken == nil || it.Path == "" || it.FolderID == 0 {
			t.Fatalf("incomplete item %+v", it)
		}
	}
	// A named group is confirmed, so it is kept with confirmedOnly too.
	if _, items, _ := h.s.PersonExport(ctx, pid, true); len(items) != 2 {
		t.Fatalf("confirmed only: %d photos, want 2", len(items))
	}
	if _, _, err := h.s.PersonExport(ctx, 9999, false); err == nil {
		t.Fatal("an unknown person should be an error")
	}
}
