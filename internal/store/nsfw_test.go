package store

import (
	"context"
	"fmt"
	"testing"
)

// Photos marked NSFW are left out unless asked for, and the user's say
// wins over the classifier; blurry photos can be listed.
func TestLookFilters(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.s.NsfwModel = true
	var ids []int64
	for i := range 3 {
		p := fmt.Sprintf("look%d", i)
		if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: p, Size: 1, MTime: 1}}); err != nil {
			t.Fatal(err)
		}
		id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: p, Size: 1, MTime: 1}, ImageData{
			ContentHash: fmt.Sprintf("%024x", 900+i), PHash: uint64(900 + i), Width: 10, Height: 10, HasThumb: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if got := len(h.s.NeedLook(ctx, 10, nil)); got != 3 {
		t.Fatalf("%d photos waiting to be looked at, want 3", got)
	}
	hash := func(id int64) string { return h.s.images[id].Hash }
	h.s.SetLook(ctx, ids[0], hash(ids[0]), Look{Sharp: 900, Nsfw: 0.97}) // nudity
	h.s.SetLook(ctx, ids[1], hash(ids[1]), Look{Sharp: 12, Nsfw: 0.01})  // blurry
	h.s.SetLook(ctx, ids[2], hash(ids[2]), Look{Sharp: 900, Nsfw: 0.5})  // unsure: not hidden
	list := func(f Filter) (out []int64) {
		ims, _, _ := h.s.Images(ctx, f)
		for _, im := range ims {
			out = append(out, im.ID)
		}
		return out
	}
	if got := fmt.Sprint(list(Filter{HideNsfw: true, Sort: "added_desc"})); got != fmt.Sprint([]int64{ids[2], ids[1]}) {
		t.Errorf("hiding NSFW: %v", got)
	}
	if got := list(Filter{OnlyNsfw: true, HideNsfw: true}); len(got) != 1 || got[0] != ids[0] {
		t.Errorf("$nsfw: %v", got)
	}
	if got := list(Filter{Blurry: true}); len(got) != 1 || got[0] != ids[1] {
		t.Errorf("$blurry: %v", got)
	}
	// The user says it's fine, and another one isn't: their say wins.
	no, yes := false, true
	h.s.EditImage(ctx, ids[0], UserEdit{Nsfw: &no})
	h.s.EditImage(ctx, ids[2], UserEdit{Nsfw: &yes})
	if got := list(Filter{OnlyNsfw: true}); len(got) != 1 || got[0] != ids[2] {
		t.Errorf("after the user's say, $nsfw: %v", got)
	}
	h.s.EditImage(ctx, ids[0], UserEdit{ResetNsfw: true})
	if !h.s.isNsfw(h.s.images[ids[0]]) {
		t.Error("reset didn't go back to the classifier")
	}
	if got := len(h.s.NeedLook(ctx, 10, nil)); got != 0 {
		t.Errorf("%d still waiting", got)
	}
}
