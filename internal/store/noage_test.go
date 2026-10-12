package store

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
)

// unagedPhoto records a one-face photo without an age estimate, as
// happens when no age model is loaded.
func (h *harness) unagedPhoto(taken time.Time, emb []float32) int64 {
	h.t.Helper()
	h.n++
	ctx := context.Background()
	path := fmt.Sprint(h.n)
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: 1, MTime: 1}}); err != nil {
		h.t.Fatal(err)
	}
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", h.n), PHash: uint64(h.n), Width: 1000, Height: 1000, TakenAt: &taken,
		HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1},
		Faces: []FaceData{{Box: [4]float32{.4, .4, .6, .6}, Score: .9, Embedding: emb}},
	}
	imgID, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 1, MTime: 1}, d)
	if err != nil {
		h.t.Fatal(err)
	}
	return h.s.facesByImage[imgID][0]
}

// Without an age model no face ever gets an age, so grouping must not wait
// for one: before, no face was ever grouped and people were never found.
func TestGroupingWithoutAgeModel(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(11, 12))
	ctx := context.Background()
	a, b := identity(r), identity(r)
	a1 := h.unagedPhoto(day(2020, 1), variant(r, a, 0.02))
	a2 := h.unagedPhoto(day(2020, 3), variant(r, a, 0.02))
	b1 := h.unagedPhoto(day(2020, 2), variant(r, b, 0.02))

	if got := h.s.NeedPeople(ctx, 100, nil); len(got) != 0 {
		t.Fatalf("with an age model, unaged faces must wait for it; queued %v", got)
	}

	h.s.NoAgeModel = true
	queued := h.s.NeedPeople(ctx, 100, nil)
	for _, f := range []int64{a1, a2, b1} {
		if !slices.Contains(queued, f) {
			t.Fatalf("face %d not queued for grouping without an age model (queued %v)", f, queued)
		}
	}
	for _, f := range queued {
		if err := h.s.ProcessFace(ctx, f); err != nil {
			t.Fatal(err)
		}
	}
	if h.group(a1) == 0 || h.group(a1) != h.group(a2) {
		t.Fatalf("the same person's faces should share a group: %d, %d", h.group(a1), h.group(a2))
	}
	if h.group(b1) == 0 || h.group(b1) == h.group(a1) {
		t.Fatalf("another person should get their own group: %d vs %d", h.group(b1), h.group(a1))
	}
}
