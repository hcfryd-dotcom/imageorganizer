package store

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
)

// side records a photo with two foreground faces side by side.
func (h *harness) side(taken time.Time, left, right []float32, ageL, ageR float32) (int64, int64) {
	h.t.Helper()
	h.n++
	ctx := context.Background()
	path := fmt.Sprint(h.n)
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: 1, MTime: 1}}); err != nil {
		h.t.Fatal(err)
	}
	img, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 1, MTime: 1}, ImageData{
		ContentHash: fmt.Sprintf("%024x", h.n), PHash: uint64(h.n), Width: 1000, Height: 1000, TakenAt: &taken, HasThumb: true,
		Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1},
		Faces: []FaceData{
			{Box: [4]float32{.1, .3, .4, .6}, Score: .9, Embedding: left},
			{Box: [4]float32{.6, .3, .9, .6}, Score: .9, Embedding: right},
		},
	})
	if err != nil {
		h.t.Fatal(err)
	}
	ids := h.s.facesByImage[img]
	for i, fid := range ids {
		h.s.SetAge(ctx, fid, []float32{ageL, ageR}[i])
		h.s.ProcessFace(ctx, fid)
	}
	return ids[0], ids[1]
}

func TestPictureOfSomeoneOnTheWall(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(31, 32))
	ctx := context.Background()
	ann := identity(r)
	var first int64
	for y := 2010; y <= 2020; y++ {
		f := h.photo(day(y, 1), variant(r, ann, 0.02), float32(y-1985))
		if first == 0 {
			first = f
		}
	}
	pid, _ := h.s.NameGroup(ctx, h.group(first), 0, "Ann")
	for _, c := range h.s.clusters {
		if c.n > 0 && c.PersonID == 0 {
			h.s.NameGroup(ctx, c.id, pid, "")
		}
	}
	// 2020: Ann (35) next to a framed photo of herself at 25.
	real, pic := h.side(day(2020, 5), variant(r, ann, 0.02), variant(r, ann, 0.02), 35, 25)
	h.s.FileFaces(ctx)
	img := h.s.faces[real].ImageID
	named := 0
	var hinted bool
	for _, v := range h.s.ImageFaces(ctx, img) {
		if v.PersonID == pid {
			named++
		}
		if v.ID == pic && v.MaybePicture == pid {
			hinted = true
		}
	}
	if named != 1 {
		t.Fatalf("before marking, Ann is named %d times", named)
	}
	if h.s.clusters[h.group(pic)].PersonID == pid && !hinted {
		// Grouped as Ann but shown unknown: it must be offered as a picture.
		t.Error("the younger second Ann isn't suggested as a picture of her")
	}
	if _, err := h.s.TagFace(ctx, pic, pid, ""); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SetFacePicture(ctx, pic, true); err != nil {
		t.Fatal(err)
	}
	named = 0
	for _, v := range h.s.ImageFaces(ctx, img) {
		if v.PersonID == pid {
			named++
			if (v.ID == pic) != v.Picture {
				t.Errorf("face %d picture=%v", v.ID, v.Picture)
			}
		}
	}
	if named != 2 {
		t.Errorf("Ann and her picture: named %d times, want 2", named)
	}
	if !slices.Contains(h.s.AloneImageIDs(), img) {
		t.Error("Ann next to a picture of herself is $alone")
	}
	if _, ok := h.s.impliedBirth(h.s.faces[pic]); ok {
		t.Error("a picture's date says nothing about age")
	}
	if h.s.clusters[h.group(pic)].imgs[img] != 0 {
		t.Error("a picture holds the person's place in the photo")
	}
	// Only a picture of Ann: not a photo of Ann.
	_, onlyPic := h.side(day(2020, 6), variant(r, identity(r), 0.02), variant(r, ann, 0.02), 40, 25)
	h.s.TagFace(ctx, onlyPic, pid, "")
	h.s.SetFacePicture(ctx, onlyPic, true)
	if ids := h.s.PersonImageIDs(pid); slices.Contains(ids, h.s.faces[onlyPic].ImageID) || !slices.Contains(ids, img) {
		t.Errorf("photos of Ann %v: want %d, not %d", ids, img, h.s.faces[onlyPic].ImageID)
	}
}

func TestPaintingNeverAges(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(33, 34))
	ctx := context.Background()
	painting, ann := identity(r), identity(r)
	p1 := h.photo(day(2005, 1), variant(r, painting, 0.02), 50)
	pid, _ := h.s.NameGroup(ctx, h.group(p1), 0, "Portrait")
	yes := true
	if err := h.s.UpdatePerson(ctx, pid, PersonEdit{Picture: &yes}); err != nil {
		t.Fatal(err)
	}
	// 15 years on it still looks 50: any age fits a picture.
	late := h.photo(day(2020, 1), variant(r, painting, 0.02), 50)
	if c := h.s.clusters[h.group(late)]; c.PersonID != pid {
		if sc, ok := h.s.personIndex().score(c, pid); !ok {
			t.Errorf("the painting at 'age 65' is ruled out (score %.2f)", sc)
		}
	}
	for _, v := range h.s.People(ctx) {
		if v.ID == pid && (v.EstBirth != 0 || !v.Picture) {
			t.Errorf("painting shown as born %.0f, picture=%v", v.EstBirth, v.Picture)
		}
	}
	// Ann in front of the painting is alone, and a photo of just the
	// painting has nobody known in it.
	a, _ := h.side(day(2006, 1), variant(r, ann, 0.02), variant(r, painting, 0.02), 30, 50)
	h.s.FileFaces(ctx)
	if !slices.Contains(h.s.AloneImageIDs(), h.s.faces[a].ImageID) {
		t.Error("Ann in front of the painting is $alone")
	}
	if slices.Contains(h.s.FaceImageIDs("known", ""), h.s.faces[p1].ImageID) {
		t.Error("a photo of the painting counts as $known")
	}
}

func TestUserDate(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(35, 36))
	ctx := context.Background()
	f := h.photo(day(2020, 1), variant(r, identity(r), 0.02), 10)
	img := h.s.faces[f].ImageID
	c := h.s.clusters[h.group(f)]
	b0, _, _ := c.birth()
	fixed := day(2000, 1).Unix()
	if err := h.s.EditImage(ctx, img, UserEdit{Taken: &fixed}); err != nil {
		t.Fatal(err)
	}
	if b1, _, _ := c.birth(); b0-b1 < 19.9 || b0-b1 > 20.1 {
		t.Errorf("group's birth year %.1f -> %.1f, want 20 years earlier", b0, b1)
	}
	v := h.s.view(h.s.images[img])
	if *v.TakenAt != fixed || v.CameraTaken == nil || *v.CameraTaken != day(2020, 1).Unix() {
		t.Errorf("view taken %v camera %v", v.TakenAt, v.CameraTaken)
	}
	// The file changes on disk (same path): the user's date stays.
	taken := day(2021, 1)
	if _, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: "1", Size: 2, MTime: 2}, ImageData{
		ContentHash: fmt.Sprintf("%024x", 999), PHash: 999, Width: 1000, Height: 1000, TakenAt: &taken,
	}); err != nil {
		t.Fatal(err)
	}
	im := h.s.images[img]
	if *im.TakenAt != fixed || *im.CameraTaken != taken.Unix() {
		t.Errorf("after the file changed: taken %d camera %d", *im.TakenAt, *im.CameraTaken)
	}
	if err := h.s.EditImage(ctx, img, UserEdit{ResetTaken: true}); err != nil {
		t.Fatal(err)
	}
	if im := h.s.images[img]; *im.TakenAt != taken.Unix() || im.UserTaken != nil {
		t.Errorf("reset: taken %d, user %v", *im.TakenAt, im.UserTaken)
	}
}

// A face filed automatically shows as unconfirmed until the user confirms
// it, one face at a time.
func TestConfirmOneAutoFace(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(37, 38))
	ctx := context.Background()
	ann := identity(r)
	f1 := h.photo(day(2010, 1), variant(r, ann, 0.02), 30)
	f0 := h.photo(day(2010, 2), variant(r, ann, 0.02), 30)
	pid, _ := h.s.ConfirmFaces(ctx, 0, "Ann", []int64{f1, f0}, nil)
	f2 := h.photo(day(2020, 1), variant(r, ann, 0.02), 40)
	f3 := h.photo(day(2020, 2), variant(r, ann, 0.02), 40)
	h.s.FileFaces(ctx)
	if v := h.s.faceView(h.s.faces[f2]); v.PersonID != pid || !v.Auto {
		t.Fatalf("filed face: person %d auto %v", v.PersonID, v.Auto)
	}
	if v := h.s.faceView(h.s.faces[f1]); v.PersonID != pid || v.Auto {
		t.Fatalf("confirmed face: person %d auto %v", v.PersonID, v.Auto)
	}
	if _, err := h.s.TagFace(ctx, f2, pid, ""); err != nil {
		t.Fatal(err)
	}
	if v := h.s.faceView(h.s.faces[f2]); v.PersonID != pid || v.Auto {
		t.Errorf("confirmed face: person %d auto %v", v.PersonID, v.Auto)
	}
	if v := h.s.faceView(h.s.faces[f3]); !v.Auto {
		t.Error("the other filed face is still unconfirmed")
	}
}

// "More of …?": unnamed faces that look like the person are offered, one
// by one; a "No" to one takes just that face off the list and leaves its
// group alone, a "No" to a filed face takes it out of the person.
func TestFaceCandidates(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(41, 42))
	ctx := context.Background()
	ann, bob := identity(r), identity(r)
	var first int64
	for i := range 6 {
		f := h.photo(day(2010, 1+i), variant(r, ann, 0.02), 30)
		if first == 0 {
			first = f
		}
	}
	pid, err := h.s.NameGroup(ctx, h.group(first), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	// Ten years on: filed as Ann automatically. Someone like her who is
	// not filed, and Bob.
	like := variant(r, ann, 0.1)
	var auto, alike, bobs []int64
	for i := range 3 {
		auto = append(auto, h.photo(day(2020, 1+i), variant(r, ann, 0.02), 40))
		alike = append(alike, h.photo(day(2020, 10+i), variant(r, like, 0.02), 40))
		bobs = append(bobs, h.photo(day(2020, 20+i), variant(r, bob, 0.02), 40))
	}
	if c := h.s.clusters[h.group(auto[0])]; c.PersonID != 0 {
		t.Fatalf("test setup: later face named (%d) before anyone saw it", c.PersonID)
	}
	if c := h.s.clusters[h.group(alike[0])]; c.PersonID != 0 {
		t.Fatalf("test setup: look-alike group filed as %d (%.2f like Ann)", c.PersonID, groupSim(c, h.s.clusters[h.group(first)]))
	}
	t.Logf("look-alike group %.2f like Ann", groupSim(h.s.clusters[h.group(alike[0])], h.s.clusters[h.group(first)]))
	ids := func() map[int64]bool {
		m := map[int64]bool{}
		for _, v := range h.s.FaceCandidates(ctx, pid, 50) {
			m[v.ID] = true
		}
		return m
	}
	got := ids()
	for _, f := range append(slices.Clone(auto), alike...) {
		if !got[f] {
			t.Errorf("face %d not offered", f)
		}
	}
	for _, f := range bobs {
		if got[f] {
			t.Errorf("Bob's face %d offered as Ann", f)
		}
	}
	if got[first] {
		t.Error("an already confirmed face offered")
	}
	g := h.group(alike[0])
	if err := h.s.RejectFace(ctx, alike[0], pid); err != nil {
		t.Fatal(err)
	}
	if h.group(alike[0]) != g {
		t.Error("a no for an unnamed face moved it out of its group")
	}
	h.s.FileFaces(ctx)
	if c := h.s.clusters[h.group(auto[0])]; c.PersonID != pid {
		t.Fatalf("test setup: later face not filed under Ann (%d)", c.PersonID)
	}
	if err := h.s.RejectFace(ctx, auto[0], pid); err != nil {
		t.Fatal(err)
	}
	if c := h.s.clusters[h.group(auto[0])]; c.PersonID == pid {
		t.Error("a no for a filed face left it filed")
	}
	if _, err := h.s.TagFace(ctx, auto[1], pid, ""); err != nil {
		t.Fatal(err)
	}
	if got := ids(); got[alike[0]] || got[auto[0]] || got[auto[1]] {
		t.Errorf("answered faces still offered: %v %v %v", got[alike[0]], got[auto[0]], got[auto[1]])
	}
}

// A whole group can be "not a person", and that can be undone.
func TestNotFacesInGroup(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(61, 62))
	ctx := context.Background()
	statue := identity(r)
	var fs []int64
	for i := range 3 {
		fs = append(fs, h.photo(day(2015, 1+i), variant(r, statue, 0.02), 40))
	}
	g := h.group(fs[0])
	ids, err := h.s.NotFacesInGroup(ctx, g)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 3 || h.s.clusters[g].n != 0 {
		t.Fatalf("marked %d faces, group has %d left", len(ids), h.s.clusters[g].n)
	}
	for _, id := range fs {
		if f := h.s.faces[id]; !f.NotFace || f.ClusterID != 0 {
			t.Errorf("face %d: notface %v group %d", id, f.NotFace, f.ClusterID)
		}
	}
	if err := h.s.RestoreGroupFaces(ctx, g, ids); err != nil {
		t.Fatal(err)
	}
	if h.s.clusters[g].n != 3 || h.s.faces[fs[1]].NotFace {
		t.Errorf("after undo: group has %d faces, notface %v", h.s.clusters[g].n, h.s.faces[fs[1]].NotFace)
	}
}

// A photo turned again while it was being analysed for the last turn
// stays waiting: that analysis was for the old orientation.
func TestTurnedDuringImport(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(71, 72))
	ctx := context.Background()
	fid := h.photo(day(2015, 1), identity(r), 30)
	img := h.s.faces[fid].ImageID
	im := h.s.images[img]
	if _, err := h.s.RotateImage(ctx, img, 90); err != nil {
		t.Fatal(err)
	}
	record := func(rot int) {
		t.Helper()
		if _, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: "1", Size: 1, MTime: 1}, ImageData{
			ContentHash: im.Hash, Width: 1000, Height: 1000, HasThumb: true, Rotated: rot,
			Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1},
			Faces:    []FaceData{{Box: [4]float32{.4, .4, .6, .6}, Score: .9, Embedding: identity(r)}},
		}); err != nil {
			t.Fatal(err)
		}
	}
	file := func() *fileRec { return h.s.files[h.s.byImage[img][0]] }
	record(0) // started before the turn
	if !file().Pending || len(h.s.images[img].Analysis) != 0 {
		t.Errorf("stale analysis kept: pending %v analysis %v", file().Pending, h.s.images[img].Analysis)
	}
	record(90)
	if file().Pending || len(h.s.images[img].Analysis) == 0 {
		t.Errorf("current analysis not kept: pending %v analysis %v", file().Pending, h.s.images[img].Analysis)
	}
}

// A very alike adult may look quite a bit younger than their age model
// says (sun, glasses, a small face); a child may not: the strict check
// keeps look-alike siblings apart.
func TestFitsLike(t *testing.T) {
	adult := ageModel{ok: true, birth: 1936, age: 75}
	child := ageModel{ok: true, birth: 2009, age: 11}
	for _, c := range []struct {
		m     ageModel
		taken float32
		looks float32
		sim   float32
		want  bool
		why   string
	}{
		{adult, 2010, 74, 0.3, true, "the right age"},
		{adult, 2010, 57, 0.77, true, "17 years young, very alike"},
		{adult, 2010, 57, 0.45, false, "17 years young, only somewhat alike"},
		{adult, 2010, 20, 0.9, false, "50 years young"},
		{child, 2015, 6, 0.4, true, "the right age"},
		{child, 2015, 11, 0.9, false, "a child five years older, however alike"},
	} {
		if got := c.m.fitsLike(c.taken-c.looks, c.looks, true, c.sim); got != c.want {
			t.Errorf("%s: fits %v, want %v", c.why, got, c.want)
		}
	}
}

// A photo turned but not drawn again yet has its thumbnail turned when
// served; once imported again, as drawn.
func TestThumbRotation(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(161, 162))
	ctx := context.Background()
	img := h.s.faces[h.photo(day(2015, 1), identity(r), 30)].ImageID
	hash := h.s.images[img].Hash
	if got := h.s.ThumbRotation(hash); got != 0 {
		t.Fatalf("fresh photo: turn thumbnail %d", got)
	}
	if _, err := h.s.RotateImage(ctx, img, -90); err != nil {
		t.Fatal(err)
	}
	if got := h.s.ThumbRotation(hash); got != 270 {
		t.Errorf("turned left, not drawn again: turn thumbnail %d, want 270", got)
	}
	// Imported again: drawn turned.
	if _, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: "1", Size: 1, MTime: 1}, ImageData{
		ContentHash: hash, Width: 1000, Height: 1000, HasThumb: true, Rotated: 270, ThumbRot: 270,
		Analysis: analysis.Versions{analysis.CMedia: 1},
	}); err != nil {
		t.Fatal(err)
	}
	if got := h.s.ThumbRotation(hash); got != 0 {
		t.Errorf("drawn again: turn thumbnail %d", got)
	}
}
