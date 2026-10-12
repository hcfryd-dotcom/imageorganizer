package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

func TestMarkFilesGoneHidesThePhoto(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(51, 52))
	ctx := context.Background()
	f := h.photo(day(2020, 1), identity(r), 30)
	img := h.s.faces[f].ImageID
	files, err := h.s.PresentFiles(ctx, img)
	if err != nil || len(files) != 1 {
		t.Fatalf("files %+v, %v", files, err)
	}
	if _, total, _ := h.s.Images(ctx, Filter{}); total != 1 {
		t.Fatalf("setup: %d photos shown", total)
	}
	if err := h.s.MarkFilesGone(ctx, []int64{files[0].FileID}); err != nil {
		t.Fatal(err)
	}
	if _, total, _ := h.s.Images(ctx, Filter{}); total != 0 {
		t.Fatal("a deleted photo is still shown")
	}
	if left, _ := h.s.PresentFiles(ctx, img); len(left) != 0 {
		t.Fatal("its file still counts as present")
	}
	if h.s.faces[f] == nil {
		t.Fatal("what was known about the photo was dropped")
	}
}
