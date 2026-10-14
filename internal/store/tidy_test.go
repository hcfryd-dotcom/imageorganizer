package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
)

// tidyPhoto records a photo without faces at path.
func (h *harness) tidyPhoto(path string, taken *time.Time, w, hgt int, size int64, phash uint64, format, make, model string) int64 {
	h.t.Helper()
	h.n++
	ctx := context.Background()
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: size, MTime: int64(h.n)}}); err != nil {
		h.t.Fatal(err)
	}
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", h.n), PHash: phash, Width: w, Height: hgt, FileSize: size, TakenAt: taken,
		Format: format, CameraMake: make, CameraModel: model, HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1},
	}
	id, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: size, MTime: int64(h.n)}, d)
	if err != nil {
		h.t.Fatal(err)
	}
	return id
}

func TestTidyExport(t *testing.T) {
	h := newHarness(t)
	at := func(s int) *time.Time { x := time.Date(2011, 5, 4, 10, 0, s, 0, time.UTC); return &x }
	// A folder copied in twice: the same photo, a few bytes apart. The
	// bigger file is kept.
	orig := h.tidyPhoto("2011-05-04/037.JPG", at(0), 3888, 2592, 3_388_000, 0xF0F0, "jpg", "Canon", "Canon EOS 1000D")
	copy := h.tidyPhoto("2011-10-12/037.JPG", at(0), 3888, 2592, 3_387_900, 0xF0F1, "jpg", "Canon", "Canon EOS 1000D")
	// A burst the same second: alike, but different in bytes. Both stay.
	b1 := h.tidyPhoto("burst/1.JPG", at(5), 2448, 3264, 143_000, 0xAAAA, "jpg", "Apple", "iPhone 6")
	b2 := h.tidyPhoto("burst/2.JPG", at(5), 2448, 3264, 163_000, 0xAAAB, "jpg", "Apple", "iPhone 6")
	shot := h.tidyPhoto("Skærmbilleder/Skærmbillede 1.png", at(9), 1505, 917, 83_000, 0x1, "png", "", "")
	undated := h.tidyPhoto("scan.jpg", nil, 800, 600, 50_000, 0x2, "jpg", "", "")
	oly := h.tidyPhoto("o.jpg", at(20), 800, 600, 51_000, 0x3, "jpg", "OLYMPUS IMAGING CORP.", "E-410")

	items, left := h.s.TidyExport(context.Background())
	if left != 1 {
		t.Errorf("left out %d, want 1", left)
	}
	got := map[int64]string{}
	for _, it := range items {
		got[it.ImageID] = it.Source
	}
	if _, ok := got[copy]; ok {
		t.Errorf("the smaller copy was kept")
	}
	want := map[int64]string{orig: "Canon", b1: "iPhone", b2: "iPhone", shot: "screenshot", undated: "", oly: "Olympus"}
	for id, src := range want {
		s, ok := got[id]
		if !ok || s != src {
			t.Errorf("image %d: source %q (listed %v), want %q", id, s, ok, src)
		}
	}
	if len(items) != len(want) {
		t.Errorf("%d items, want %d", len(items), len(want))
	}
	if items[len(items)-1].ImageID != undated {
		t.Errorf("undated photo not last")
	}
}

func TestPhotoSource(t *testing.T) {
	for _, c := range []struct{ make, model, format, path, want string }{
		{"Apple", "iPad Air", "heic", "a.heic", "iPad"},
		{"Apple", "", "jpg", "a.jpg", "Apple"},
		{"FUJIFILM", "FinePix", "jpg", "a.jpg", "Fujifilm"},
		{"NIKON CORPORATION", "D70", "jpg", "a.jpg", "Nikon"},
		{"Sony Ericsson", "K800i", "jpg", "a.jpg", "Sony Ericsson"},
		{"Hewlett-Packard", "", "jpg", "a.jpg", "HP"},
		{"HP", "", "jpg", "a.jpg", "HP"},
		{"phaseonea/s/S", "", "jpg", "a.jpg", "Phaseoneass"},
		{"", "", "jpg", "Screenshots/Screenshot 1.jpg", "screenshot"},
		{"", "", "jpg", "a.jpg", ""},
	} {
		im := &imageRec{CameraMake: c.make, CameraModel: c.model, Format: c.format}
		if got := photoSource(im, c.path); got != c.want {
			t.Errorf("%q %q %s: %q, want %q", c.make, c.model, c.path, got, c.want)
		}
	}
}
