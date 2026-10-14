package store

import (
	"context"
	"testing"
	"time"
)

func TestAlbums(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	at := time.Date(2024, 7, 1, 10, 0, 0, 0, time.UTC)
	a := h.geoPhoto("a.jpg", at, "jpg")
	b := h.geoPhoto("b.jpg", at.Add(time.Hour), "jpg")
	c := h.geoPhoto("c.jpg", at.Add(2*time.Hour), "jpg")

	if _, err := h.s.CreateAlbum(ctx, "  ", nil); err == nil {
		t.Fatal("an album without a name was made")
	}
	summer, err := h.s.CreateAlbum(ctx, "Sommer 2024", []int64{a, b, a, 999})
	if err != nil || summer.Photos != 2 {
		t.Fatalf("create: %+v %v", summer, err)
	}
	if n, _ := h.s.AddToAlbum(ctx, summer.ID, []int64{b, c}); n != 1 {
		t.Errorf("added %d, want 1 (b was there)", n)
	}
	if got := h.s.AlbumImageIDs(summer.ID); len(got) != 3 || got[0] != a || got[2] != c {
		t.Errorf("photos %v", got)
	}
	jul, _ := h.s.CreateAlbum(ctx, "Jul", []int64{c})
	if refs := h.s.AlbumsOf(ctx, c); len(refs) != 2 || refs[0].Name != "Jul" {
		t.Errorf("albums of c: %+v", refs)
	}
	if n, _ := h.s.RemoveFromAlbum(ctx, summer.ID, []int64{a}); n != 1 {
		t.Errorf("removed %d", n)
	}
	if err := h.s.RenameAlbum(ctx, jul.ID, "Jul 2024"); err != nil {
		t.Fatal(err)
	}
	if err := h.s.DeleteAlbum(ctx, summer.ID); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.s.images[b]; !ok {
		t.Fatal("deleting the album removed a photo")
	}
	// Kept over a reopening.
	path := h.s.db.Path()
	h.s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	al := s2.Albums(ctx)
	if len(al) != 1 || al[0].Name != "Jul 2024" || al[0].Photos != 1 || al[0].CoverHash == "" {
		t.Fatalf("after reopening: %+v", al)
	}
}
