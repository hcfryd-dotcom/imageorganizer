package server

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

func TestFetch(t *testing.T) {
	ctx := context.Background()
	data := t.TempDir()
	st, err := store.Open(filepath.Join(data, "t.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := &Server{Config: Config{Store: st, DataDir: data, Log: slog.New(slog.DiscardHandler)}, scanCtx: ctx}
	from, dir := t.TempDir(), t.TempDir()
	pic := func(name string, shade uint8, when time.Time) []byte {
		img := image.NewGray(image.Rect(0, 0, 8, 8))
		for i := range img.Pix {
			img.Pix[i] = shade
		}
		var b bytes.Buffer
		png.Encode(&b, img)
		p := filepath.Join(from, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, b.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, when, when)
		return b.Bytes()
	}
	march := time.Date(2020, 3, 5, 12, 0, 0, 0, time.UTC)
	pic("a.png", 10, march)
	pic("sub/a copy.png", 10, march) // the same photo twice: fetched once
	known := pic("old.png", 200, march)
	// The library has old.png already.
	if _, err := st.AddFolder(ctx, t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Discover(ctx, 1, 1, []store.Discovered{{Path: "old.png", Size: 1, MTime: 1}}); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%016x%08x", xxhash.Sum64(known), len(known))
	if _, _, err := st.RecordFile(ctx, store.FileRef{FolderID: 1, Path: "old.png", Size: 1, MTime: 1}, store.ImageData{ContentHash: hash, Width: 8, Height: 8}); err != nil {
		t.Fatal(err)
	}
	run := func() exportStatus {
		ctx, cancel := context.WithCancel(ctx)
		s.export.st = exportStatus{Running: true}
		s.runFetch(ctx, cancel, from, dir, tidyLang["da"])
		return s.export.st
	}
	got := run()
	if got.Total != 3 || got.Copied != 1 || got.Skipped != 2 || got.Failed != 0 {
		t.Fatalf("first fetch: %+v", got)
	}
	want := filepath.Join(dir, "2020", "03 Marts", "Skærmbilleder", "a.png")
	if _, err := os.Stat(want); err != nil {
		// The copy of a.png in sub/ may have been met first.
		if _, err2 := os.Stat(filepath.Join(filepath.Dir(want), "a copy.png")); err2 != nil {
			t.Fatalf("not fetched to %s: %v", want, err)
		}
	}
	if fi, err := os.Stat(filepath.Dir(want)); err != nil || !fi.IsDir() {
		t.Fatalf("no tidy folder: %v", err)
	}
	// Files looked at are remembered: a second fetch reads nothing again,
	// even with the fetched copy gone.
	os.RemoveAll(filepath.Join(dir, "2020"))
	if got := run(); got.Copied != 0 || got.Skipped != 3 {
		t.Fatalf("second fetch: %+v", got)
	}
	// A new photo is fetched.
	pic("new.png", 90, march.AddDate(1, 0, 0))
	if got := run(); got.Copied != 1 || got.Skipped != 3 {
		t.Fatalf("third fetch: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "2021", "03 Marts", "Skærmbilleder", "new.png")); err != nil {
		t.Fatal(err)
	}
}

func TestWithin(t *testing.T) {
	sep := string(filepath.Separator)
	a := filepath.Join(sep+"x", "Photos")
	for _, c := range []struct {
		p    string
		want bool
	}{{a, true}, {filepath.Join(a, "sub"), true}, {a + " tidy", false}, {filepath.Join(sep + "x"), false}} {
		if got := within(c.p, a); got != c.want {
			t.Errorf("within(%s, %s) = %v", c.p, a, got)
		}
	}
}
