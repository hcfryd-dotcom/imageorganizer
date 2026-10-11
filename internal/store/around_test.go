package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestAround(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	add := func(p string, taken *time.Time) int64 {
		t.Helper()
		h.n++
		if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: p, Size: 1, MTime: 1}}); err != nil {
			t.Fatal(err)
		}
		id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: p, Size: 1, MTime: 1}, ImageData{
			ContentHash: fmt.Sprintf("%024x", h.n), PHash: uint64(h.n), Width: 10, Height: 10, TakenAt: taken,
		})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	at := func(h int) *time.Time { t := time.Date(2020, 5, 1, h, 0, 0, 0, time.UTC); return &t }
	// Dated photos are in time order, whatever their folders.
	d3 := add("b/3.jpg", at(3))
	d1 := add("a/1.jpg", at(1))
	d2 := add("c/2.jpg", at(2))
	d4 := add("a/4.jpg", at(4))
	// Undated ones go by name in their folder.
	u2 := add("a/u2.jpg", nil)
	u1 := add("a/u1.jpg", nil)
	add("b/u0.jpg", nil)

	ids := func(a Around) (out []int64) {
		for _, im := range a.Items {
			out = append(out, im.ID)
		}
		return out
	}
	for _, c := range []struct {
		id   int64
		n    int
		by   string
		want []int64
	}{
		{d2, 1, "time", []int64{d1, d2, d3}},
		{d1, 2, "time", []int64{d1, d2, d3}},
		{d4, 5, "time", []int64{d1, d2, d3, d4}},
		{u2, 5, "folder", []int64{d1, d4, u1, u2}},
	} {
		a, err := h.s.Around(ctx, c.id, c.n)
		if err != nil {
			t.Fatal(err)
		}
		if a.By != c.by || fmt.Sprint(ids(a)) != fmt.Sprint(c.want) {
			t.Errorf("around %d: %s %v, want %s %v", c.id, a.By, ids(a), c.by, c.want)
		}
	}
}
