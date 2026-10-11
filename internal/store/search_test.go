package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// A search lists the best match first; in another order (oldest first),
// its best matches so ordered.
func TestSearchOrder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	q := []float32{1, 0}
	var ids []int64
	// Older photos match less: 2010 0.1, 2011 0.6, 2012 0.9.
	for i, like := range []float32{0.1, 0.6, 0.9} {
		p := fmt.Sprintf("s%d", i)
		taken := time.Date(2010+i, 1, 1, 0, 0, 0, 0, time.UTC)
		if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: p, Size: 1, MTime: 1}}); err != nil {
			t.Fatal(err)
		}
		id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: p, Size: 1, MTime: 1}, ImageData{
			ContentHash: fmt.Sprintf("%024x", 700+i), PHash: uint64(700 + i), Width: 10, Height: 10, TakenAt: &taken, HasThumb: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := h.s.SetVector(ctx, id, h.s.images[id].Hash, []float32{like, 1 - like}); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	order := func(sort string) []int64 {
		ims, _, err := h.s.Search(ctx, q, Filter{Sort: sort, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		var out []int64
		for _, im := range ims {
			out = append(out, im.ID)
		}
		return out
	}
	if got := fmt.Sprint(order(SortRelevance)); got != fmt.Sprint([]int64{ids[2], ids[1], ids[0]}) {
		t.Errorf("most relevant first: %v", got)
	}
	if got := fmt.Sprint(order("taken_asc")); got != fmt.Sprint(ids) {
		t.Errorf("oldest first: %v", got)
	}
}
