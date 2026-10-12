package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

func TestExportTarget(t *testing.T) {
	dir := t.TempDir()
	p, exists := exportTarget(dir, "IMG_1.jpg", 3)
	if exists || filepath.Base(p) != "IMG_1.jpg" {
		t.Fatalf("empty folder: %s %v", p, exists)
	}
	if err := writeFileAtomic(p, strings.NewReader("abc")); err != nil {
		t.Fatal(err)
	}
	// The same photo again (same name and size): skipped.
	if p2, exists := exportTarget(dir, "IMG_1.jpg", 3); !exists || p2 != p {
		t.Fatalf("same file: %s %v", p2, exists)
	}
	// Another photo with that name: numbered, not overwritten.
	p3, exists := exportTarget(dir, "IMG_1.jpg", 5)
	if exists || filepath.Base(p3) != "IMG_1 (2).jpg" {
		t.Fatalf("name clash: %s %v", p3, exists)
	}
	if b, _ := os.ReadFile(p); string(b) != "abc" {
		t.Fatal("the first file changed")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".copying-*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestExportDir(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "Ann")
	if got, err := exportDir(` "` + abs + `" `); err != nil || got != abs {
		t.Fatalf("quoted path: %q %v", got, err)
	}
	for _, bad := range []string{"", "   ", "Pictures/Ann"} {
		if _, err := exportDir(bad); err == nil {
			t.Fatalf("%q accepted", bad)
		}
	}
}

func TestPlaceFolders(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	items := []store.ExportItem{
		{ImageID: 1, Lat: f(57.7209), Lon: f(10.5839)}, // Skagen
		{ImageID: 2, Lat: f(55.6761), Lon: f(12.5683)}, // Copenhagen
		{ImageID: 3, Lat: f(41.4036), Lon: f(2.1744)},  // Barcelona
		{ImageID: 4}, // no position
	}
	if got := placeFolders(items, ""); len(got) != 0 {
		t.Fatalf("years only, yet places: %v", got)
	}
	area := placeFolders(items, "area")
	want := map[int64]string{1: "Frederikshavn", 2: "København", 3: "Barcelona, Spanien"}
	for id, w := range want {
		if area[id] != w {
			t.Errorf("area: photo %d in %q, want %q", id, area[id], w)
		}
	}
	if _, ok := area[4]; ok {
		t.Error("a photo without a position got a place")
	}
	if town := placeFolders(items, "town"); town[1] != "Skagen" {
		t.Errorf("town: Skagen photo in %q", town[1])
	}
}
