package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// A child photographed every year, born 2000; one print of them at 4
// photographed in 2020 carries 2020: its faces say 2004. Once dated (or
// said to be right), it isn't asked about again.
func TestDoubtfulDates(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(201, 202))
	ctx := context.Background()
	kid := identity(r)
	var fs []int64
	for y := 2002; y <= 2015; y++ {
		fs = append(fs, h.photo(day(y, 1), variant(r, kid, 0.02), float32(y-2000)))
	}
	fs = append(fs, h.photo(day(2020, 2), variant(r, kid, 0.02), 4)) // the print
	pid, _ := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Kid")
	for _, f := range fs {
		if h.s.clusters[h.group(f)].PersonID == 0 {
			h.s.NameGroup(ctx, h.group(f), pid, "")
		}
	}
	ds := h.s.DoubtfulDates(ctx, 10)
	if len(ds) != 1 || ds[0].Year < 2003 || ds[0].Year > 2005 {
		t.Fatalf("doubtful %+v", ds)
	}
	img := ds[0].Image
	ok := true
	if err := h.s.EditImage(ctx, img, UserEdit{DateOK: &ok}); err != nil {
		t.Fatal(err)
	}
	if ds := h.s.DoubtfulDates(ctx, 10); len(ds) != 0 {
		t.Errorf("asked again after 'date is right': %+v", ds)
	}
}

// A face filed under a child that looks 30 in a photo from when they were
// 5 is doubted; once confirmed (tagged), it isn't asked about again.
func TestDoubtfulFaces(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(211, 212))
	ctx := context.Background()
	kid := identity(r)
	var fs []int64
	for y := 2002; y <= 2015; y++ {
		fs = append(fs, h.photo(day(y, 1), variant(r, kid, 0.02), float32(y-2000)))
	}
	odd := h.photo(day(2005, 2), variant(r, kid, 0.02), 30)
	fs = append(fs, odd)
	pid, _ := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Kid")
	for _, f := range fs {
		if h.s.clusters[h.group(f)].PersonID == 0 {
			h.s.NameGroup(ctx, h.group(f), pid, "")
		}
	}
	// The odd one filed under them unseen (as a group could, once).
	if _, err := h.s.ConfirmFaces(ctx, pid, "", nil, []int64{odd}); err != nil {
		t.Fatal(err)
	}
	h.s.mu.Lock()
	o := h.s.faces[odd]
	o.Not = nil
	h.s.placeFiled(o, pid)
	h.s.mu.Unlock()
	ds := h.s.DoubtfulFaces(ctx, 10)
	if len(ds) != 1 || ds[0].Face.ID != odd || !ds[0].AgeOff || ds[0].Person != pid {
		t.Fatalf("doubtful %+v", ds)
	}
	if _, err := h.s.TagFace(ctx, odd, pid, ""); err != nil {
		t.Fatal(err)
	}
	if ds := h.s.DoubtfulFaces(ctx, 10); len(ds) != 0 {
		t.Errorf("asked again once confirmed: %+v", ds)
	}
}
