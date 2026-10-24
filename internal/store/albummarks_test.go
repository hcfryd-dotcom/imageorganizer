package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// A photo's number and title in an album put it in place when the album
// is listed in its own order: numbered ones first by number, the rest
// oldest first; taking a photo out of the album forgets its mark.
func TestAlbumMarksOrder(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	add := func(n int, sec int64) int64 {
		t.Helper()
		p := fmt.Sprintf("m%d.jpg", n)
		taken := time.Unix(sec, 0)
		if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: p, Size: 1, MTime: 1}}); err != nil {
			t.Fatal(err)
		}
		id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: p, Size: 1, MTime: 1}, ImageData{
			ContentHash: fmt.Sprintf("%024x", 4000+n), PHash: uint64(4000 + n), Width: 10, Height: 10, HasThumb: true, TakenAt: &taken,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a, b, c, d := add(1, 100), add(2, 200), add(3, 300), add(4, 400)
	al, err := h.s.CreateAlbum(ctx, "Bog", []int64{a, b, c, d})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.s.SetAlbumMark(ctx, al.ID, d, AlbumMark{N: 1, Title: "Kapitel 1"}); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SetAlbumMark(ctx, al.ID, b, AlbumMark{N: 2}); err != nil {
		t.Fatal(err)
	}
	list := func() (out []int64) {
		ims, _, _ := h.s.Images(ctx, Filter{IDs: h.s.AlbumImageIDs(al.ID), Sort: SortAlbum, Order: h.s.AlbumOrder(al.ID)})
		for _, im := range ims {
			out = append(out, im.ID)
		}
		return out
	}
	if got, want := fmt.Sprint(list()), fmt.Sprint([]int64{d, b, a, c}); got != want {
		t.Fatalf("album order %s, want %s", got, want)
	}
	if m := h.s.AlbumMarks(al.ID)[d]; m.Title != "Kapitel 1" || m.N != 1 {
		t.Fatalf("mark of d: %+v", m)
	}
	// No number and no title: no mark.
	if err := h.s.SetAlbumMark(ctx, al.ID, b, AlbumMark{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.s.AlbumMarks(al.ID)[b]; ok {
		t.Fatal("an empty mark is kept")
	}
	h.s.RemoveFromAlbum(ctx, al.ID, []int64{d})
	if _, ok := h.s.AlbumMarks(al.ID)[d]; ok {
		t.Fatal("a photo taken out keeps its mark")
	}
	if err := h.s.SetAlbumMark(ctx, al.ID, d, AlbumMark{N: 3}); err != ErrNotFound {
		t.Fatalf("marking a photo not in the album: %v", err)
	}
}
