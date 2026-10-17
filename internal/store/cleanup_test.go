package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Near copies are photos taken close together that look alike; the
// sharpest is kept, and a favourite always. Screenshots are PNGs no
// camera made, or named so.
func TestCleanupCopiesAndShots(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	add := func(n int, path, format, make string, sec int64, phash uint64) int64 {
		t.Helper()
		taken := time.Unix(sec, 0)
		if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: 1, MTime: 1}}); err != nil {
			t.Fatal(err)
		}
		id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 1, MTime: 1}, ImageData{
			ContentHash: fmt.Sprintf("%024x", 3000+n), PHash: phash, Width: 10, Height: 10, HasThumb: true,
			Format: format, CameraMake: make, TakenAt: &taken,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := add(1, "a.jpg", "jpg", "Apple", 1000, 0xff00)
	b := add(2, "b.jpg", "jpg", "Apple", 1020, 0xff01)     // 20 s later, alike: a copy of a
	c := add(3, "c.jpg", "jpg", "Apple", 1030, 0xffffffff) // close in time, but another picture
	d := add(4, "d.jpg", "jpg", "Apple", 5000, 0xff00)     // alike, but long after
	shot := add(5, "IMG_1.PNG", "png", "", 6000, 1)
	named := add(6, "Skærmbillede 2024.jpg", "jpg", "", 7000, 2)
	h.s.SetLook(ctx, a, h.s.images[a].Hash, Look{Sharp: 100})
	h.s.SetLook(ctx, b, h.s.images[b].Hash, Look{Sharp: 300})

	got := h.s.CleanupCopies(ctx)
	if len(got) != 2 || got[0].ID != b || !got[0].Keep || got[1].ID != a || got[1].Keep || got[0].Set != got[1].Set {
		t.Fatalf("copies: %+v (want b kept, then a)", got)
	}
	_, _ = c, d
	// A favourite is the one kept, however much blurrier.
	fav := true
	h.s.EditImage(ctx, a, UserEdit{Favorite: &fav})
	got = h.s.CleanupCopies(ctx)
	if len(got) != 2 || got[0].ID != a || !got[0].Keep || got[1].Keep {
		t.Fatalf("with a favourite: %+v (want a kept, then b)", got)
	}

	shots := h.s.CleanupShots(ctx)
	if len(shots) != 2 || shots[0].ID != named || shots[1].ID != shot {
		t.Fatalf("screenshots: %+v", shots)
	}
}
