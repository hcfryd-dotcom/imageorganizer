package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// Near-certain faces are filed under the person by themselves, as not
// confirmed; a so-so face isn't; an undo puts it back as it was.
func TestAutoAccept(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(181, 182))
	ctx := context.Background()
	ann, bob := identity(r), identity(r)
	h.s.mu.Lock()
	h.s.tune.autoOff = true // only AutoAccept files anything here
	h.s.mu.Unlock()
	var a1, b1 int64
	for i := range 4 {
		a := h.photo(day(2010, 1+i), variant(r, ann, 0.02), 30)
		b := h.photo(day(2010, 10+i), variant(r, bob, 0.02), 50)
		if i == 0 {
			a1, b1 = a, b
		}
	}
	pa, _ := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	h.s.NameGroup(ctx, h.group(b1), 0, "Bob")
	// Ann years later (a group of her own, unnamed), and someone a bit
	// like her.
	sure := h.photo(day(2015, 1), variant(r, ann, 0.03), 35)
	soSo := h.photo(day(2015, 2), variant(r, ann, 0.1), 35)
	// In a library, such faces sit in unnamed groups formed earlier; here
	// they joined Ann's, so give each a group of its own.
	for _, id := range []int64{sure, soSo} {
		f := h.s.faces[id]
		h.s.mu.Lock()
		h.s.clusterRemove(h.s.clusters[f.ClusterID], f)
		h.s.nextCluster++
		nc := &cluster{id: h.s.nextCluster}
		h.s.clusters[nc.id] = nc
		f.ClusterID = nc.id
		h.s.clusterAdd(nc, f)
		h.s.mu.Unlock()
	}
	got, err := h.s.AutoAccept(ctx, pa)
	if err != nil {
		t.Fatal(err)
	}
	accepted := map[int64]int64{}
	for _, a := range got {
		accepted[a.Face] = a.From
	}
	if _, ok := accepted[sure]; !ok {
		t.Fatalf("the near-certain face not accepted: %+v", got)
	}
	if _, ok := accepted[soSo]; ok {
		t.Error("a so-so face accepted")
	}
	c := h.s.clusters[h.group(sure)]
	if c.PersonID != pa || !c.Auto || h.s.faces[sure].Tagged != 0 {
		t.Errorf("accepted face: person %d auto %v tagged %d; want Ann, not confirmed", c.PersonID, c.Auto, h.s.faces[sure].Tagged)
	}
	if err := h.s.UndoAccept(ctx, sure, accepted[sure]); err != nil {
		t.Fatal(err)
	}
	if c := h.s.clusters[h.group(sure)]; c.PersonID != 0 {
		t.Error("undo left it filed")
	}
}
