package store

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
)

// The matcher offers unknown faces with the person they most likely are;
// a "No" passes the face to the next likeliest person, or drops it.
func TestFaceMatches(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(51, 52))
	ctx := context.Background()
	ann, bob, eve := identity(r), identity(r), identity(r)
	var a1, b1 int64
	var anns []int64
	for i := range 4 {
		f := h.photo(day(2010, 1+i), variant(r, ann, 0.02), 30)
		g := h.photo(day(2010, 10+i), variant(r, bob, 0.02), 50)
		anns = append(anns, f)
		if i == 0 {
			a1, b1 = f, g
		}
	}
	pa, _ := h.s.NameGroup(ctx, h.group(a1), 0, "Ann")
	h.s.NameGroup(ctx, h.group(b1), 0, "Bob")
	// The user saw three of Ann's faces when naming her; the fourth was
	// only filed with them, and is never shown as her.
	if err := h.s.MarkSeen(ctx, pa, anns[:3]); err != nil {
		t.Fatal(err)
	}
	like := variant(r, ann, 0.1)
	var alike, eves []int64
	for i := range 3 {
		alike = append(alike, h.photo(day(2020, 1+i), variant(r, like, 0.02), 40))
		eves = append(eves, h.photo(day(2020, 10+i), variant(r, eve, 0.02), 40))
	}
	matches := func() map[int64]FaceMatch {
		m := map[int64]FaceMatch{}
		for _, fm := range h.s.FaceMatches(ctx, 50, MatchOptions{}) {
			m[fm.Face.ID] = fm
		}
		return m
	}
	got := matches()
	for _, f := range alike {
		if fm, ok := got[f]; !ok || fm.Person != pa || fm.Like.PersonID != pa {
			t.Errorf("face %d: %+v, want Ann", f, fm)
		} else if fm.Like.ID == anns[3] {
			t.Errorf("face %d shown with a face of Ann's nobody confirmed", f)
		}
	}
	for _, f := range eves {
		if fm, ok := got[f]; ok {
			t.Errorf("Eve's face %d offered as %s", f, fm.Name)
		}
	}
	// An age range: Ann's look-alikes look 40.
	ids := func(o MatchOptions) (out []int64) {
		for _, fm := range h.s.FaceMatches(ctx, 50, o) {
			out = append(out, fm.Face.ID)
		}
		return out
	}
	if got := ids(MatchOptions{MinAge: 41}); len(got) != 0 {
		t.Errorf("41 and up: %v, want none", got)
	}
	if got := ids(MatchOptions{MinAge: 35, MaxAge: 45}); len(got) != len(alike) {
		t.Errorf("35 to 45: %v, want %v", got, alike)
	}
	// By when the photos were taken, oldest first: in the order taken.
	if got := ids(MatchOptions{Sort: "oldphoto"}); fmt.Sprint(got) != fmt.Sprint(alike) {
		t.Errorf("oldest photo first: %v, want %v", got, alike)
	}
	if got := ids(MatchOptions{Sort: "new"}); len(got) != 3 || got[0] != alike[2] {
		t.Errorf("newest photo first: %v", got)
	}
	// A skipped face goes to the back of the queue, the latest skip last.
	order := func() (ids []int64) {
		for _, fm := range h.s.FaceMatches(ctx, 50, MatchOptions{}) {
			ids = append(ids, fm.Face.ID)
		}
		return ids
	}
	first := order()
	if len(first) < 3 {
		t.Fatalf("only %d matches", len(first))
	}
	if err := h.s.SkipFace(ctx, first[0]); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SkipFace(ctx, first[1]); err != nil {
		t.Fatal(err)
	}
	if o := order(); o[len(o)-2] != first[0] || o[len(o)-1] != first[1] || o[0] != first[2] {
		t.Errorf("after skipping %d, %d: order %v", first[0], first[1], o)
	}
	if err := h.s.RejectFace(ctx, alike[0], pa); err != nil {
		t.Fatal(err)
	}
	if fm, ok := matches()[alike[0]]; ok && fm.Person == pa {
		t.Error("face still offered as Ann after a no")
	}
}
