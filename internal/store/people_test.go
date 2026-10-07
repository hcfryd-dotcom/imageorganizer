package store

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
	bolt "go.etcd.io/bbolt"
)

func identity(r *rand.Rand) []float32 {
	v := make([]float32, 512)
	for i := range v {
		v[i] = float32(r.NormFloat64())
	}
	return unit(v)
}

// variant is base plus noise: another photo of the same face, or (with more
// noise) the same person years later.
func variant(r *rand.Rand, base []float32, noise float64) []float32 {
	v := make([]float32, len(base))
	for i := range v {
		v[i] = base[i] + float32(r.NormFloat64()*noise)
	}
	return unit(v)
}

func unit(v []float32) []float32 {
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	inv := float32(1 / math.Sqrt(n))
	for i := range v {
		v[i] *= inv
	}
	return v
}

type harness struct {
	t *testing.T
	s *Store
	n int
}

func newHarness(t *testing.T) *harness {
	s, err := Open(filepath.Join(t.TempDir(), "t.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if _, err := s.AddFolder(context.Background(), "/x", ""); err != nil {
		t.Fatal(err)
	}
	return &harness{t: t, s: s}
}

// photo records a one-face photo and runs it through aging and grouping.
func (h *harness) photo(taken time.Time, emb []float32, age float32) int64 {
	h.t.Helper()
	h.n++
	ctx := context.Background()
	path := fmt.Sprint(h.n)
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: 1, MTime: 1}}); err != nil {
		h.t.Fatal(err)
	}
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", h.n), PHash: uint64(h.n), Width: 1000, Height: 1000, TakenAt: &taken,
		HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1},
		Faces: []FaceData{{Box: [4]float32{.4, .4, .6, .6}, Score: .9, Embedding: emb}},
	}
	imgID, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 1, MTime: 1}, d)
	if err != nil {
		h.t.Fatal(err)
	}
	fid := h.s.facesByImage[imgID][0]
	if err := h.s.SetAge(ctx, fid, age); err != nil {
		h.t.Fatal(err)
	}
	if err := h.s.ProcessFace(ctx, fid); err != nil {
		h.t.Fatal(err)
	}
	return fid
}

func (h *harness) group(fid int64) int64 { return h.s.faces[fid].ClusterID }

func day(y, d int) time.Time { return time.Date(y, 6, d, 0, 0, 0, 0, time.UTC) }

func groupIDs(gs []GroupView) []int64 {
	var ids []int64
	for _, g := range gs {
		ids = append(ids, g.ID)
	}
	return ids
}

// The user's case: two children who looked the same at 6, born 5 years
// apart.
func TestSiblingsBornApartStaySeparate(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(7, 7))
	older := identity(r)
	younger := variant(r, older, 0.014) // near-identical face
	var o8, y8 []int64
	for i := range 6 {
		o8 = append(o8, h.photo(day(2001, 1+i), variant(r, older, 0.02), 8+float32(i%3)*0.4))
		y8 = append(y8, h.photo(day(2006, 1+i), variant(r, younger, 0.02), 8+float32(i%3)*0.4))
	}
	gOlder, gYounger := h.group(o8[0]), h.group(y8[0])
	for _, f := range o8 {
		if h.group(f) != gOlder {
			t.Fatal("older child's photos split across groups")
		}
	}
	for _, f := range y8 {
		if h.group(f) != gYounger {
			t.Fatal("younger child's photos split across groups")
		}
	}
	if gOlder == gYounger {
		t.Fatal("siblings born 5 years apart merged into one group")
	}
	sim := groupSim(h.s.clusters[gOlder], h.s.clusters[gYounger])
	if sim < clusterJoin {
		t.Fatalf("test setup: groups only %.2f similar, not a look-alike test", sim)
	}
	t.Logf("sibling groups are %.2f similar (join threshold %.2f) yet kept apart by birth year", sim, clusterJoin)

	// The older child at 12 looks different enough to form its own group.
	older12 := variant(r, older, 0.035)
	var o12 []int64
	for i := range 4 {
		o12 = append(o12, h.photo(day(2005, 1+i), variant(r, older12, 0.02), 12+float32(i%2)*0.5))
	}
	gOlder12 := h.group(o12[0])

	// Name the older child at 8. "Is this also …?" must offer the same child
	// at 12, and must not offer the younger sibling.
	ctx := context.Background()
	pid, err := h.s.NameGroup(ctx, gOlder, 0, "Older")
	if err != nil {
		t.Fatal(err)
	}
	// It may also be filed automatically, as a confident match; the
	// younger sibling must be neither offered nor filed.
	cands := groupIDs(h.s.Candidates(ctx, pid, 10, 1))
	if slices.Contains(cands, gYounger) {
		t.Errorf("younger sibling offered as the older one: candidates %v", cands)
	}
	if h.s.clusters[gYounger].PersonID == pid {
		t.Errorf("younger sibling filed as the older one")
	}
	if c := h.s.clusters[gOlder12]; gOlder12 != gOlder && c.PersonID != pid && !slices.Contains(cands, gOlder12) {
		t.Errorf("older child at 12 (group %d) neither filed nor offered: candidates %v", gOlder12, cands)
	}
	if _, err := h.s.NameGroup(ctx, gOlder12, pid, ""); err != nil {
		t.Fatal(err)
	}

	// New photos join automatically: the older child's to the person, the
	// younger child's not.
	fo := h.photo(day(2001, 20), variant(r, older, 0.02), 8.2)
	if c := h.s.clusters[h.group(fo)]; c.PersonID != pid {
		t.Errorf("new photo of the older child not recognised")
	}
	fy := h.photo(day(2006, 20), variant(r, younger, 0.02), 8.2)
	if c := h.s.clusters[h.group(fy)]; c.PersonID == pid {
		t.Errorf("younger sibling recognised as the older one")
	}
	if got := len(h.s.PersonImageIDs(pid)); got != 6+4+1 {
		t.Errorf("person has %d photos, want 11", got)
	}
}

// Without ages, grouping falls back on capture time: the same face in the
// same period groups together, but not across years.
func TestNoAgeUsesTimeWindow(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(7, 7))
	face := identity(r)
	a := h.photo(day(2010, 1), variant(r, face, 0.02), -1)
	b := h.photo(time.Date(2010, 9, 1, 0, 0, 0, 0, time.UTC), variant(r, face, 0.02), -1)
	c := h.photo(day(2015, 1), variant(r, face, 0.02), -1)
	if h.group(a) != h.group(b) {
		t.Error("same face, same period, no age: should share a group")
	}
	if h.group(a) == h.group(c) {
		t.Error("same-looking faces five years apart, no age: should not share a group")
	}
}

// A younger sibling as a toddler, with the age overestimated (2 read as
// 6.5), implies the older sibling's birth year. The time window keeps the
// toddler out of the older one's group from years before.
func TestOverestimatedToddlerStaysOut(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(3, 9))
	older := identity(r)
	younger := variant(r, older, 0.014)
	var o []int64
	for i := range 6 { // older child at ~3 in 2003: implied birth ~2000
		o = append(o, h.photo(day(2003, 1+i), variant(r, older, 0.02), 3))
	}
	// Younger child at 2 in 2006, read as 6.5: implied birth ~1999.5.
	y := h.photo(day(2006, 1), variant(r, younger, 0.02), 6.5)
	if h.group(y) == h.group(o[0]) {
		t.Fatal("toddler with overestimated age joined the older sibling's group from 3 years earlier")
	}
}

func TestRejectRemoveMergeAndPersist(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(1, 2))
	ctx := context.Background()
	a, b := identity(r), identity(r)
	fa := h.photo(day(2020, 1), variant(r, a, 0.02), 30)
	h.photo(day(2020, 2), variant(r, a, 0.02), 30)
	fb := h.photo(day(2020, 3), variant(r, b, 0.02), 40)
	h.photo(day(2020, 4), variant(r, b, 0.02), 40)

	pa, _ := h.s.NameGroup(ctx, h.group(fa), 0, "Ann")
	pb, _ := h.s.NameGroup(ctx, h.group(fb), 0, "ann ") // same name, any case: same person
	if pa != pb {
		t.Fatalf("naming with an existing name should reuse the person")
	}
	if err := h.s.RejectGroup(ctx, h.group(fb), pa); err != nil {
		t.Fatal(err)
	}
	if h.s.clusters[h.group(fb)].PersonID != 0 {
		t.Fatal("rejected group still assigned")
	}
	if slices.Contains(groupIDs(h.s.Candidates(ctx, pa, 10, 1)), h.group(fb)) {
		t.Fatal("rejected group offered again")
	}

	// Taking a face out of a group moves it to its own group.
	before := h.group(fa)
	if err := h.s.RemoveFace(ctx, fa); err != nil {
		t.Fatal(err)
	}
	if h.group(fa) == before || h.s.clusters[h.group(fa)].PersonID != 0 {
		t.Fatal("removed face still in the person's group")
	}

	pb2, _ := h.s.NameGroup(ctx, h.group(fb), 0, "Bea")
	if err := h.s.MergePeople(ctx, pb2, pa); err != nil {
		t.Fatal(err)
	}
	if h.s.clusters[h.group(fb)].PersonID != pa || h.s.people[pb2] != nil {
		t.Fatal("merge did not move the group")
	}

	// Everything survives a reopen.
	path := h.s.db.Path()
	h.s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.clusters[h.group(fb)].PersonID != pa || len(s2.people) != 1 {
		t.Fatalf("decisions lost on reopen: %+v people=%d", s2.clusters[h.group(fb)].rec(), len(s2.people))
	}
}

// A change of grouping rules regroups everything at startup, keeping who
// the user said people are.
func TestRegroupKeepsNames(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(5, 5))
	ctx := context.Background()
	a := identity(r)
	var fs []int64
	for i := range 5 {
		fs = append(fs, h.photo(day(2020, 1+i), variant(r, a, 0.02), 30))
	}
	pid, err := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	// Pretend the data was grouped by older rules.
	if err := h.s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketMeta).Put(keyGroupingVersion, []byte("1"))
	}); err != nil {
		t.Fatal(err)
	}
	path := h.s.db.Path()
	h.s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	for _, f := range fs {
		if c := s2.clusters[s2.faces[f].ClusterID]; c == nil || c.PersonID != pid {
			t.Fatalf("face %d lost its person after regrouping", f)
		}
	}
}

// The regroup that lost names in a real library: a named group whose faces
// end up in a new group together with many never-named faces of the same
// person. Only named faces vote, so the name survives.
func TestRegroupKeepsNamesWhenMixedWithUnnamed(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(8, 8))
	ctx := context.Background()
	a := identity(r)
	var fs []int64
	for i := range 10 {
		fs = append(fs, h.photo(day(2020, 1+i), variant(r, a, 0.02), 30))
	}
	pid, err := h.s.NameGroup(ctx, h.group(fs[0]), 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	// Simulate older rules having split the person: six of the ten faces
	// sat in a separate group nobody named.
	if err := h.s.db.Update(func(tx *bolt.Tx) error {
		for _, id := range fs[4:] {
			f := *h.s.faces[id]
			f.ClusterID = 999
			if err := put(tx, bucketFaces, f.ID, &f); err != nil {
				return err
			}
		}
		return tx.Bucket(bucketMeta).Put(keyGroupingVersion, []byte("1"))
	}); err != nil {
		t.Fatal(err)
	}
	path := h.s.db.Path()
	h.s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	for _, f := range fs {
		if c := s2.clusters[s2.faces[f].ClusterID]; c == nil || c.PersonID != pid {
			t.Fatalf("face %d lost its person: 4 of 10 faces were named, all agreeing", f)
		}
	}
	if matches, _ := filepath.Glob(path + ".pre-regroup-*"); len(matches) == 0 {
		t.Error("no backup was written before regrouping")
	}
}

func TestNotAFaceStaysOut(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(5, 6))
	ctx := context.Background()
	cat := identity(r)
	f1 := h.photo(day(2020, 1), variant(r, cat, 0.02), 5)
	f2 := h.photo(day(2020, 2), variant(r, cat, 0.02), 5)
	g := h.group(f1)
	if h.group(f2) != g {
		t.Fatal("setup: both faces should share a group")
	}
	img := h.s.faces[f1].ImageID
	if err := h.s.NotAFace(ctx, f1); err != nil {
		t.Fatal(err)
	}
	if h.s.clusters[g].n != 1 || h.group(f1) != 0 {
		t.Fatal("not-a-face still in its group")
	}
	if len(h.s.ImageFaces(ctx, img)) != 0 {
		t.Fatal("not-a-face still shown on its photo")
	}
	if slices.Contains(h.s.NeedPeople(ctx, 100, nil), f1) {
		t.Fatal("not-a-face queued for grouping again")
	}
	// Re-analysing the photo finds the same box again; it stays out.
	path := "1"
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", 1), PHash: 1, Width: 1000, Height: 1000, HasThumb: true,
		Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 2},
		Faces:    []FaceData{{Box: [4]float32{.41, .4, .6, .6}, Score: .9, Embedding: variant(r, cat, 0.02)}},
	}
	if _, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 1, MTime: 1}, d); err != nil {
		t.Fatal(err)
	}
	if len(h.s.ImageFaces(ctx, img)) != 0 {
		t.Fatal("re-analysis brought the not-a-face back")
	}
}

func TestKnownUnknownPhotos(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(7, 8))
	ctx := context.Background()
	ann, bob, cat := identity(r), identity(r), identity(r)
	fa := h.photo(day(2020, 1), variant(r, ann, 0.02), 30)
	fb := h.photo(day(2020, 2), variant(r, bob, 0.02), 30)
	fc := h.photo(day(2020, 3), variant(r, cat, 0.02), 5)
	if _, err := h.s.NameGroup(ctx, h.group(fa), 0, "Ann"); err != nil {
		t.Fatal(err)
	}
	if err := h.s.HideGroup(ctx, h.group(fc), true); err != nil {
		t.Fatal(err)
	}
	img := func(fid int64) int64 { return h.s.faces[fid].ImageID }
	if got := h.s.FaceImageIDs(true); !slices.Equal(got, []int64{img(fa)}) {
		t.Errorf("known photos = %v, want Ann's photo %d", got, img(fa))
	}
	if got := h.s.FaceImageIDs(false); !slices.Equal(got, []int64{img(fb)}) {
		t.Errorf("unknown photos = %v, want only Bob's photo %d (hidden groups count as neither)", got, img(fb))
	}
}

// crowd records a photo with one main face and small faces behind it,
// and groups them all.
func (h *harness) crowd(taken time.Time, main []float32, back ...[]float32) (int64, []int64) {
	h.t.Helper()
	h.n++
	ctx := context.Background()
	path := fmt.Sprint(h.n)
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: path, Size: 1, MTime: 1}}); err != nil {
		h.t.Fatal(err)
	}
	faces := []FaceData{{Box: [4]float32{.3, .3, .7, .7}, Score: .9, Embedding: main}} // 400px
	for i, e := range back {
		x := .05 + float32(i)*.06
		faces = append(faces, FaceData{Box: [4]float32{x, .05, x + .05, .1}, Score: .9, Embedding: e}) // 50px
	}
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", h.n), PHash: uint64(h.n), Width: 1000, Height: 1000, TakenAt: &taken,
		HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1}, Faces: faces,
	}
	imgID, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: path, Size: 1, MTime: 1}, d)
	if err != nil {
		h.t.Fatal(err)
	}
	ids := h.s.facesByImage[imgID]
	for _, fid := range ids {
		if err := h.s.SetAge(ctx, fid, 30); err != nil {
			h.t.Fatal(err)
		}
		if err := h.s.ProcessFace(ctx, fid); err != nil {
			h.t.Fatal(err)
		}
	}
	return ids[0], ids[1:]
}

func TestBackgroundFacesStayOutOfReview(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(9, 10))
	ctx := context.Background()
	son, stranger := identity(r), identity(r)
	var mains, backs []int64
	for i := range 3 {
		m, b := h.crowd(day(2020, 1+i), variant(r, son, 0.02), variant(r, stranger, 0.02))
		mains, backs = append(mains, m), append(backs, b...)
	}
	if h.group(backs[0]) != h.group(backs[1]) {
		t.Fatal("setup: the stranger's faces should group together")
	}
	items, _ := h.s.Review(ctx, 2, 0, 10, 1, ReviewBySize)
	ids := []int64{}
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	if !slices.Contains(ids, h.group(mains[0])) {
		t.Errorf("the main face's group should be in review: %v", ids)
	}
	if slices.Contains(ids, h.group(backs[0])) {
		t.Errorf("a group of background faces is in review: %v", ids)
	}
	if got := h.s.FaceImageIDs(false); len(got) != 3 {
		t.Errorf("$unknown: %d photos, want the 3 with an unnamed main face", len(got))
	}
	pid, _ := h.s.NameGroup(ctx, h.group(mains[0]), 0, "Son")
	if got := h.s.FaceImageIDs(false); len(got) != 0 {
		t.Errorf("$unknown still finds %d photos with only background strangers left", len(got))
	}
	_ = pid
}

func TestOnePersonPerPhoto(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(11, 12))
	twin := identity(r)
	// Two near-identical faces in one photo can't both be the same person.
	h.n++
	ctx := context.Background()
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: "pair", Size: 1, MTime: 1}}); err != nil {
		t.Fatal(err)
	}
	taken := day(2020, 1)
	d := ImageData{
		ContentHash: fmt.Sprintf("%024x", 999), PHash: 999, Width: 1000, Height: 1000, TakenAt: &taken,
		HasThumb: true, Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1},
		Faces: []FaceData{
			{Box: [4]float32{.1, .3, .4, .6}, Score: .9, Embedding: variant(r, twin, 0.01)},
			{Box: [4]float32{.6, .3, .9, .6}, Score: .9, Embedding: variant(r, twin, 0.01)},
		},
	}
	imgID, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: "pair", Size: 1, MTime: 1}, d)
	if err != nil {
		t.Fatal(err)
	}
	ids := h.s.facesByImage[imgID]
	for _, fid := range ids {
		h.s.SetAge(ctx, fid, 30)
		h.s.ProcessFace(ctx, fid)
	}
	if h.group(ids[0]) == h.group(ids[1]) {
		t.Fatal("two faces of one photo in the same group")
	}
}

func TestAutoFiledAndUndone(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(13, 14))
	ctx := context.Background()
	ann := identity(r)
	// Ann in 2010 and again in 2020: outside one group's time window, but
	// the same face, so the later group is filed under her automatically.
	f1 := h.photo(day(2010, 1), variant(r, ann, 0.02), 30)
	h.photo(day(2010, 2), variant(r, ann, 0.02), 30)
	pid, _ := h.s.NameGroup(ctx, h.group(f1), 0, "Ann")
	f2 := h.photo(day(2020, 1), variant(r, ann, 0.02), 40)
	c := h.s.clusters[h.group(f2)]
	if h.group(f2) == h.group(f1) || c.PersonID != pid || !c.Auto {
		t.Fatalf("later group: person %d auto %v, want filed under Ann automatically", c.PersonID, c.Auto)
	}
	h.photo(day(2020, 3), variant(r, ann, 0.02), 40) // 2 faces: big enough for review
	if !c.inReview(2) {
		t.Fatal("an automatically filed group should wait in review for confirmation")
	}
	// "Not Ann" sticks: the group isn't filed again as it grows.
	if err := h.s.RejectGroup(ctx, c.id, pid); err != nil {
		t.Fatal(err)
	}
	f3 := h.photo(day(2020, 2), variant(r, ann, 0.02), 40)
	if h.group(f3) != c.id || c.PersonID != 0 || c.Auto {
		t.Fatalf("after Not Ann: person %d auto %v", c.PersonID, c.Auto)
	}
}

func TestTagFaceIsJustThatFace(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(15, 16))
	ctx := context.Background()
	ann := identity(r)
	f1 := h.photo(day(2020, 1), variant(r, ann, 0.02), 30)
	f2 := h.photo(day(2020, 2), variant(r, ann, 0.02), 30)
	f3 := h.photo(day(2020, 3), variant(r, ann, 0.02), 30)
	g := h.group(f1)
	if h.group(f2) != g || h.group(f3) != g {
		t.Fatal("setup: one group")
	}
	pid, err := h.s.TagFace(ctx, f1, 0, "Ann")
	if err != nil {
		t.Fatal(err)
	}
	if h.group(f1) == g || h.s.clusters[h.group(f1)].PersonID != pid {
		t.Fatalf("tagged face should be Ann, in a group of its own: group %d (old %d) person %d want %d", h.group(f1), g, h.s.clusters[h.group(f1)].PersonID, pid)
	}
	if h.s.clusters[g].PersonID != 0 {
		t.Fatal("tagging one face named its whole group")
	}
	if !slices.Contains(groupIDs(h.s.Candidates(ctx, pid, 10, 1)), g) {
		t.Error("the face's old group should be an 'Is this also Ann?' candidate")
	}
	// The tag survives regrouping, even though the face regroups with the
	// unnamed rest.
	if err := h.s.db.Update(h.s.regroup); err != nil {
		t.Fatal(err)
	}
	if c := h.s.clusters[h.group(f1)]; c.PersonID != pid {
		t.Fatalf("after regroup the tagged face is person %d, want Ann", c.PersonID)
	}
	// Taking the face out of Ann clears the tag.
	if err := h.s.RemoveFace(ctx, f1); err != nil {
		t.Fatal(err)
	}
	if h.s.faces[f1].Tagged != 0 || h.s.clusters[h.group(f1)].PersonID == pid {
		t.Fatal("removed face still tagged as Ann")
	}
}

func TestSeenFacesAreTagged(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(17, 18))
	ctx := context.Background()
	ann := identity(r)
	f1 := h.photo(day(2020, 1), variant(r, ann, 0.02), 30)
	f2 := h.photo(day(2020, 2), variant(r, ann, 0.02), 30)
	pid, _ := h.s.NameGroup(ctx, h.group(f1), 0, "Ann")
	if err := h.s.MarkSeen(ctx, pid, []int64{f1}); err != nil {
		t.Fatal(err)
	}
	if h.s.faces[f1].Tagged != pid || h.s.faces[f2].Tagged != 0 {
		t.Fatal("only the face the user saw should be tagged")
	}
	if err := h.s.db.Update(h.s.regroup); err != nil {
		t.Fatal(err)
	}
	if h.s.clusters[h.group(f1)].PersonID != pid {
		t.Fatal("seen face lost its person in a regroup")
	}
}

func TestPickedCover(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(19, 20))
	ctx := context.Background()
	ann := identity(r)
	f1 := h.photo(day(2020, 1), variant(r, ann, 0.02), 30)
	f2 := h.photo(day(2020, 2), variant(r, ann, 0.02), 30)
	pid, _ := h.s.NameGroup(ctx, h.group(f1), 0, "Ann")
	cover := func() string {
		for _, p := range h.s.People(ctx) {
			if p.ID == pid {
				return p.Cover
			}
		}
		return ""
	}
	want := h.s.faceView(h.s.faces[f2]).Crop
	if err := h.s.UpdatePerson(ctx, pid, nil, nil, &f2); err != nil {
		t.Fatal(err)
	}
	if got := cover(); got != want {
		t.Fatalf("cover %q, want the picked face %q", got, want)
	}
	// Once the face is no longer theirs, the cover falls back.
	if err := h.s.RemoveFace(ctx, f2); err != nil {
		t.Fatal(err)
	}
	if got := cover(); got == want || got == "" {
		t.Fatalf("cover %q after the picked face was taken out", got)
	}
}

func TestMoveGroupToAnotherPerson(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(21, 22))
	ctx := context.Background()
	a, b := identity(r), identity(r)
	fa := h.photo(day(2020, 1), variant(r, a, 0.02), 30)
	fb1 := h.photo(day(2020, 2), variant(r, b, 0.02), 30)
	h.photo(day(2020, 3), variant(r, b, 0.02), 30)
	maria, _ := h.s.NameGroup(ctx, h.group(fa), 0, "Maria")
	if _, err := h.s.NameGroup(ctx, h.group(fb1), maria, ""); err != nil { // the mistake
		t.Fatal(err)
	}
	h.s.MarkSeen(ctx, maria, []int64{fb1})
	// Split: the second group is another Maria.
	other, err := h.s.NameGroup(ctx, h.group(fb1), 0, "Maria H.")
	if err != nil || other == maria {
		t.Fatalf("new person %d (err %v), want a different person from %d", other, err, maria)
	}
	if h.s.faces[fb1].Tagged != other {
		t.Fatal("the seen face's tag should move with its group")
	}
	if err := h.s.db.Update(h.s.regroup); err != nil {
		t.Fatal(err)
	}
	if h.s.clusters[h.group(fb1)].PersonID != other || h.s.clusters[h.group(fa)].PersonID != maria {
		t.Fatal("after a regroup the two Marias should stay apart")
	}
}

func TestNeverTwiceInAPhoto(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(23, 24))
	ctx := context.Background()
	ann := identity(r)
	f1 := h.photo(day(2020, 1), variant(r, ann, 0.02), 30)
	h.photo(day(2020, 2), variant(r, ann, 0.02), 30)
	pid, _ := h.s.NameGroup(ctx, h.group(f1), 0, "Ann")
	// A photo with Ann and a look-alike: the look-alike may not be filed
	// as Ann, and the photo shows Ann once.
	main, back := h.crowd(day(2020, 3), variant(r, ann, 0.02), variant(r, ann, 0.03))
	if c := h.s.clusters[h.group(back[0])]; c.PersonID == pid {
		t.Fatalf("second face in the photo filed as Ann too (main face person %d)", h.s.clusters[h.group(main)].PersonID)
	}
	n := 0
	for _, v := range h.s.ImageFaces(ctx, h.s.faces[main].ImageID) {
		if v.PersonID == pid {
			n++
		}
	}
	if n > 1 {
		t.Fatalf("Ann shown %d times in one photo", n)
	}
}

func TestAlonePhotos(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(25, 26))
	ann, bob, stranger := identity(r), identity(r), identity(r)
	solo := h.photo(day(2020, 1), variant(r, ann, 0.02), 30)
	withCrowd, _ := h.crowd(day(2020, 2), variant(r, ann, 0.02), variant(r, stranger, 0.02))
	// Two people side by side, both in front.
	ctx := context.Background()
	if _, err := h.s.Discover(ctx, 1, 1, []Discovered{{Path: "pair", Size: 1, MTime: 1}}); err != nil {
		t.Fatal(err)
	}
	taken := day(2020, 3)
	pair, _, err := h.s.RecordFile(ctx, FileRef{FolderID: 1, Path: "pair", Size: 1, MTime: 1}, ImageData{
		ContentHash: fmt.Sprintf("%024x", 777), PHash: 777, Width: 1000, Height: 1000, TakenAt: &taken, HasThumb: true,
		Analysis: analysis.Versions{analysis.CMedia: 1, analysis.CFaces: 1},
		Faces: []FaceData{
			{Box: [4]float32{.1, .3, .4, .6}, Score: .9, Embedding: variant(r, ann, 0.02)},
			{Box: [4]float32{.6, .3, .9, .6}, Score: .9, Embedding: variant(r, bob, 0.02)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, fid := range h.s.facesByImage[pair] {
		h.s.SetAge(ctx, fid, 30)
		h.s.ProcessFace(ctx, fid)
	}
	got := h.s.AloneImageIDs()
	img := func(fid int64) int64 { return h.s.faces[fid].ImageID }
	if !slices.Contains(got, img(solo)) || !slices.Contains(got, img(withCrowd)) {
		t.Errorf("a portrait (with or without strangers far behind) is $alone: %v", got)
	}
	if slices.Contains(got, pair) {
		t.Error("two people side by side are not $alone")
	}
}
