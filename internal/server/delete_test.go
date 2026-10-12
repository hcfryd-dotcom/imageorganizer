package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lkarlslund/imageorganizer/internal/source"
)

func TestMoveFileAndFreeName(t *testing.T) {
	lib, trash := t.TempDir(), t.TempDir()
	src := filepath.Join(lib, "IMG_1.jpg")
	if err := os.WriteFile(src, []byte("photo"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := freeName(filepath.Join(trash, "IMG_1.jpg"))
	if err := moveFile(src, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("the original is still in the library folder")
	}
	if b, _ := os.ReadFile(dest); string(b) != "photo" {
		t.Fatal("the moved file lost its contents")
	}
	// Another photo of the same name doesn't replace it.
	if got := freeName(filepath.Join(trash, "IMG_1.jpg")); filepath.Base(got) != "IMG_1 (2).jpg" {
		t.Fatalf("freeName = %s", got)
	}
}

func TestLocalPathOnlyForLocalFolders(t *testing.T) {
	dir := t.TempDir()
	sp, err := source.Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, ok := source.LocalPath(sp, "sub/a.jpg")
	if !ok || p != filepath.Join(dir, "sub", "a.jpg") {
		t.Fatalf("local: %q %v", p, ok)
	}
	smb, err := source.Parse("smb://nas/photos")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := source.LocalPath(smb, "a.jpg"); ok {
		t.Fatal("a network share was treated as local")
	}
}
