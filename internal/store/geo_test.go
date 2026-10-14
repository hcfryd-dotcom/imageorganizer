package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
)

// geoPhoto records a photo without faces at path, with a position or none.
func (h *harness) geoPhoto(path string, taken time.Time, format string, pos ...float64) int64 {
	h.t.Helper()
	h.n++
	ctx := context.Background()
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: int64(h.n), MTime: int64(h.n)}}); err != nil {
		h.t.Fatal(err)
	}
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", h.n), PHash: uint64(h.n), Width: 1000, Height: 800, TakenAt: &taken,
		Format: format, HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1},
	}
	if len(pos) == 2 {
		d.Lat, d.Lon = &pos[0], &pos[1]
	}
	id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: int64(h.n), MTime: int64(h.n)}, d)
	if err != nil {
		h.t.Fatal(err)
	}
	return id
}

func TestPlaces(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	at := func(hh, mm int) time.Time { return time.Date(2012, 7, 1, hh, mm, 0, 0, time.UTC) }
	const aLat, aLon, cLat, cLon = 56.15, 10.21, 55.68, 12.57 // Aarhus, Copenhagen
	a := h.geoPhoto("a", at(10, 0), "jpg", aLat, aLon)
	b := h.geoPhoto("b", at(10, 40), "jpg")   // 40 min after a
	png := h.geoPhoto("d", at(10, 10), "png") // a screenshot: never borrows
	f := h.geoPhoto("f", at(11, 0), "jpg")    // an hour from a and from e, far apart: unsure
	e := h.geoPhoto("e", at(12, 0), "jpg", cLat, cLon)
	g := h.geoPhoto("g", at(11, 50), "jpg") // 10 min before e
	c := h.geoPhoto("c", at(16, 0), "jpg")  // 4 hours after e

	n, err := h.s.BorrowPlaces(ctx, 3*time.Hour, false)
	if err != nil || n != 2 {
		t.Fatalf("count: %d %v, want 2", n, err)
	}
	if h.s.images[b].Lat != nil {
		t.Fatal("counting changed a photo")
	}
	if _, err := h.s.BorrowPlaces(ctx, 3*time.Hour, true); err != nil {
		t.Fatal(err)
	}
	pos := func(id int64) string {
		im := h.s.images[id]
		if im.Lat == nil {
			return "none"
		}
		return fmt.Sprintf("%.2f,%.2f %s", *im.Lat, *im.Lon, im.GeoSrc)
	}
	for id, want := range map[int64]string{
		a: "56.15,10.21 ", e: "55.68,12.57 ", b: "56.15,10.21 near", png: "none", f: "none", g: "55.68,12.57 near", c: "none",
	} {
		if got := pos(id); got != want {
			t.Errorf("image %d: %s, want %s", id, got, want)
		}
	}
	// Borrowed places aren't lent on: a second run finds nothing new.
	if n, _ := h.s.BorrowPlaces(ctx, 3*time.Hour, false); n != 0 {
		t.Errorf("second run: %d", n)
	}

	// The user's place: not over a file's own.
	set, skipped, err := h.s.SetPlace(ctx, []int64{a, c}, 57.72, 10.58)
	if err != nil || set != 1 || skipped != 1 {
		t.Fatalf("set %d skipped %d %v", set, skipped, err)
	}
	if pos(c) != "57.72,10.58 user" || pos(a) != "56.15,10.21 " {
		t.Errorf("after set: a %s, c %s", pos(a), pos(c))
	}

	// The file changes: without a position the given place stays, with one
	// the file's wins.
	h.n++
	reRecord := func(id int64, path string, p ...float64) {
		taken := at(10, 40)
		d := ImageData{ContentHash: fmt.Sprintf("%024x", 1000+h.n), PHash: uint64(1000 + h.n), Width: 1000, Height: 800,
			TakenAt: &taken, Format: "jpg", HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1}}
		h.n++
		if len(p) == 2 {
			d.Lat, d.Lon = &p[0], &p[1]
		}
		got, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 999 + int64(h.n), MTime: 999 + int64(h.n)}, d)
		if err != nil || got != id {
			t.Fatalf("re-record %s: %d %v", path, got, err)
		}
	}
	reRecord(b, "b")
	if pos(b) != "56.15,10.21 near" {
		t.Errorf("edited file without a position: %s", pos(b))
	}
	reRecord(b, "b", 56.0, 9.0)
	if pos(b) != "56.00,9.00 " {
		t.Errorf("edited file with a position: %s", pos(b))
	}

	// Taking away every borrowed place leaves the user's and the files'.
	if n, err := h.s.ClearPlace(ctx, nil, GeoNear); err != nil || n != 1 {
		t.Fatalf("clear near: %d %v", n, err)
	}
	if pos(g) != "none" || pos(c) != "57.72,10.58 user" || pos(a) != "56.15,10.21 " {
		t.Errorf("after clearing: g %s, c %s, a %s", pos(g), pos(c), pos(a))
	}
	// Persisted: a reopened store has the same.
	path := h.s.db.Path()
	h.s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	h.s = s2
	if pos(c) != "57.72,10.58 user" || pos(g) != "none" {
		t.Errorf("after reopening: c %s, g %s", pos(c), pos(g))
	}
}
