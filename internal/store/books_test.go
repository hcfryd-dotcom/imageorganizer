package store

import (
	"context"
	"encoding/json"
	"testing"
)

// A book is stored with its data, listed with its counts, changed in
// place and deleted.
func TestBooks(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.s.SaveBook(ctx, Book{Title: "No source"}); err == nil {
		t.Fatal("a book without a source was saved")
	}
	data := json.RawMessage(`{"order":[5,6,7],"out":[5],"pages":[{"n":2,"text":null},{"n":0,"text":"Hi"}],"theme":"summer"}`)
	bk, err := h.s.SaveBook(ctx, Book{Title: "Summer", Source: "album=1", Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if bk.ID == 0 || bk.Photos != 2 || bk.Pages != 3 || bk.Cover != 6 || bk.Data != nil {
		t.Fatalf("saved: %+v", bk)
	}
	bk.Title = "Summer 2024"
	bk.Data = json.RawMessage(`{"order":[6],"out":[],"pages":[{"n":1}]}`)
	if _, err := h.s.SaveBook(ctx, bk); err != nil {
		t.Fatal(err)
	}
	got, err := h.s.GetBook(ctx, bk.ID)
	if err != nil || got.Title != "Summer 2024" || got.Photos != 1 || string(got.Data) != `{"order":[6],"out":[],"pages":[{"n":1}]}` {
		t.Fatalf("read back: %+v %v", got, err)
	}
	// The page's own form: photos on each page, a cover chosen, a back.
	bk.Data = json.RawMessage(`{"v":2,"pages":[{"n":2,"ids":[5,6]},{"n":0,"ids":[],"text":"x"},{"n":1,"ids":[7]}],"cover":7,"back":{"text":""}}`)
	if bk, err = h.s.SaveBook(ctx, bk); err != nil || bk.Photos != 3 || bk.Pages != 5 || bk.Cover != 7 {
		t.Fatalf("v2: %+v %v", bk, err)
	}
	list, _ := h.s.Books(ctx)
	if len(list) != 1 || list[0].Data != nil || list[0].Created == 0 {
		t.Fatalf("list: %+v", list)
	}
	if _, err := h.s.SaveBook(ctx, Book{ID: 99, Source: "album=1"}); err != ErrNotFound {
		t.Fatalf("saving over a missing book: %v", err)
	}
	if err := h.s.DeleteBook(ctx, bk.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.GetBook(ctx, bk.ID); err != ErrNotFound {
		t.Fatalf("after delete: %v", err)
	}
}
