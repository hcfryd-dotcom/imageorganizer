package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
