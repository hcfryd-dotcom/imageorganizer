package store

import (
	"context"
	"math/rand/v2"
	"testing"
	"time"
)

// A child born in 2008: faces filed as her in photos from before then
// (her older siblings as babies, say) are taken out, together, into one
// group known not to be her; her later faces stay.
func TestNotBefore(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(301, 302))
	ctx := context.Background()
	kid := identity(r)
	var fs []int64
	for y := 2004; y <= 2012; y++ {
		fs = append(fs, h.photo(day(y, 1), variant(r, kid, 0.02), 1))
	}
	pid, _ := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Mathilde")
	for _, f := range fs {
		if h.s.clusters[h.group(f)].PersonID == 0 {
			h.s.NameGroup(ctx, h.group(f), pid, "")
		}
	}
	// One of the early faces was tagged one by one as well.
	h.s.faces[fs[1]].Tagged = pid

	birth := time.Date(2008, 11, 10, 0, 0, 0, 0, time.UTC)
	n, err := h.s.NotBefore(ctx, pid, birth)
	if err != nil || n != 5 { // 2004–2008: the 2008 photo (June) is before November
		t.Fatalf("taken out %d, %v; want 5", n, err)
	}
	g := h.group(fs[0])
	for i, f := range fs {
		c := h.s.clusters[h.group(f)]
		early := i < 5
		if early && (h.group(f) != g || c.PersonID == pid || h.s.faces[f].Tagged == pid) {
			t.Errorf("early face %d still hers or apart: group %d person %d", i, h.group(f), c.PersonID)
		}
		if !early && c.PersonID != pid {
			t.Errorf("later face %d lost her", i)
		}
	}
	if c := h.s.clusters[g]; len(c.Rejected) != 1 || c.Rejected[0] != pid {
		t.Errorf("the new group doesn't know it isn't her: %v", c.Rejected)
	}
	if n, _ := h.s.NotBefore(ctx, pid, birth); n != 0 {
		t.Errorf("a second time took %d", n)
	}
}
