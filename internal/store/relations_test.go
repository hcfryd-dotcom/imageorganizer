package store

import (
	"context"
	"math/rand/v2"
	"testing"
)

// Relationships are kept on both sides, and survive merging people.
func TestRelations(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(121, 122))
	ctx := context.Background()
	name := func(n string) int64 {
		t.Helper()
		pid, err := h.s.NameGroup(ctx, h.group(h.photo(day(2010, len(n)), identity(r), 40)), 0, n)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	alice, carol, bob, carol2 := name("Alice"), name("Carol"), name("Bob"), name("Carol II")
	if err := h.s.SetRelation(ctx, carol, alice, KinParent, true); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SetRelation(ctx, alice, bob, KinPartner, true); err != nil {
		t.Fatal(err)
	}
	h.s.mu.RLock()
	got := h.s.relatives(alice)
	h.s.mu.RUnlock()
	if got[carol] != KinChild || got[bob] != KinPartner {
		t.Errorf("Alice's relatives %v", got)
	}
	if err := h.s.SetRelation(ctx, alice, alice, KinSibling, true); err == nil {
		t.Error("someone made their own sibling")
	}
	// Carol II turns out to be Carol: her mother stays her mother.
	if err := h.s.SetRelation(ctx, carol2, bob, KinParent, true); err != nil {
		t.Fatal(err)
	}
	if err := h.s.MergePeople(ctx, carol2, carol); err != nil {
		t.Fatal(err)
	}
	h.s.mu.RLock()
	got = h.s.relatives(carol)
	h.s.mu.RUnlock()
	if got[alice] != KinParent || got[bob] != KinParent || len(got) != 2 {
		t.Errorf("after merging: Carol's relatives %v", got)
	}
	if err := h.s.SetRelation(ctx, alice, bob, KinPartner, false); err != nil {
		t.Fatal(err)
	}
	h.s.mu.RLock()
	got = h.s.relatives(bob)
	h.s.mu.RUnlock()
	if _, ok := got[alice]; ok {
		t.Error("removed partner still a partner")
	}
}

// A face beside someone's partner is offered as them, even when only
// somewhat alike, and says why; relatives suggested together say how
// they are related.
func TestRelativesBeside(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(131, 132))
	ctx := context.Background()
	alice, bob, carol := identity(r), identity(r), identity(r)
	var b1, k1, h1 int64
	for i := range 4 {
		b, k := h.side(day(2000, 1+i), variant(r, alice, 0.02), variant(r, bob, 0.02), 60, 62)
		hf := h.photo(day(2000, 10+i), variant(r, carol, 0.02), 45)
		if i == 0 {
			b1, k1, h1 = b, k, hf
		}
	}
	pb, _ := h.s.NameGroup(ctx, h.group(b1), 0, "Alice")
	pk, _ := h.s.NameGroup(ctx, h.group(k1), 0, "Bob")
	ph, _ := h.s.NameGroup(ctx, h.group(h1), 0, "Carol")
	if err := h.s.SetRelation(ctx, pb, pk, KinPartner, true); err != nil {
		t.Fatal(err)
	}
	if err := h.s.SetRelation(ctx, ph, pb, KinParent, true); err != nil {
		t.Fatal(err)
	}
	// Alice in 2010, in sunglasses: only somewhat alike, beside Bob.
	odd := variant(r, alice, 0.14)
	fb, _ := h.side(day(2010, 1), odd, variant(r, bob, 0.02), 70, 72)
	h.s.FileFaces(ctx) // Bob beside her, filed under him
	sim := dot(unitVec(h.s.faces[fb].emb), clusterUnit(h.s.clusters[h.group(b1)]))
	if sim >= faceCandMin || sim < kinSim {
		t.Fatalf("test setup: %.2f alike, want between %.2f and %.2f", sim, kinSim, faceCandMin)
	}
	var found *FaceMatch
	for _, m := range h.s.FaceMatches(ctx, 50, MatchOptions{}) {
		if m.Face.ID == fb {
			found = &m
		}
	}
	if found == nil || found.Person != pb || found.Why != "next to Bob (partner)" {
		t.Fatalf("match for the face beside Bob: %+v", found)
	}
	var offered bool
	for _, v := range h.s.FaceCandidates(ctx, pb, 50) {
		if v.ID == fb && v.Why == "next to Bob (partner)" {
			offered = true
		}
	}
	if !offered {
		t.Error("not offered on Alice's page")
	}
	// Mother and daughter suggested together say so.
	h.s.mu.RLock()
	defer h.s.mu.RUnlock()
	c := h.s.clusters[h.group(fb)]
	sg := h.s.suggest(c, []*faceRec{h.s.faces[fb]}, nil, h.s.personIndex(), h.s.family())
	if len(sg) == 0 || sg[0].ID != pb || sg[0].Why != "with Bob (partner) in 1 of 1 photos" {
		t.Errorf("suggestions %+v, want Alice first, with Bob", sg)
	}
	// Mother and daughter suggested together say how they're related.
	two := []Suggestion{{ID: pb, Name: "Alice"}, {ID: ph, Name: "Carol"}}
	labelKin(two, h.s.family(), func(int64) string { return "" })
	if two[0].Kin != "Carol’s parent" || two[1].Kin != "Alice’s child" {
		t.Errorf("kin labels %q, %q", two[0].Kin, two[1].Kin)
	}
}

// Relations that follow from the ones entered: siblings of siblings,
// children of the same parent, shared parents, grandparents.
func TestInferredRelations(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(141, 142))
	ctx := context.Background()
	name := func(n string) int64 {
		t.Helper()
		pid, err := h.s.NameGroup(ctx, h.group(h.photo(day(2010, 1+len(n)), identity(r), 40)), 0, n)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	a, b, c, d, mum, gran := name("A"), name("Bb"), name("Ccc"), name("Dddd"), name("Mum..."), name("Gran...")
	rel := func(x, y int64, kind string) {
		t.Helper()
		if err := h.s.SetRelation(ctx, x, y, kind, true); err != nil {
			t.Fatal(err)
		}
	}
	rel(a, b, KinSibling) // b is a's sibling
	rel(a, c, KinSibling)
	rel(d, mum, KinParent) // d's parent is mum
	rel(a, mum, KinParent)
	rel(mum, gran, KinParent)
	h.s.mu.RLock()
	fam, in := h.s.family(), h.s.entered()
	view := h.s.relativesView(fam[b], in[b], func(int64) string { return "" })
	h.s.mu.RUnlock()
	for _, c := range []struct {
		x, y int64
		want string
		why  string
	}{
		{b, c, KinSibling, "siblings of siblings"},
		{d, b, KinSibling, "children of the same parent, and their siblings"},
		{c, mum, KinParent, "siblings share parents"},
		{mum, d, KinChild, "entered, seen from the parent"},
		{b, gran, KinGrandparent, "a parent's parent"},
		{gran, c, KinGrandchild, "and the other way"},
	} {
		if got := fam[c.x][c.y]; got != c.want {
			t.Errorf("%s: got %q, want %q", c.why, got, c.want)
		}
	}
	for _, v := range view {
		if v.ID == a && v.Inferred || v.ID == c && !v.Inferred {
			t.Errorf("b's relative %s: inferred %v", v.Name, v.Inferred)
		}
	}
}

// A match always offers the suggested person's siblings, whatever the age
// estimate says: siblings are the people mixed up most.
func TestMatchOffersSiblings(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(171, 172))
	ctx := context.Background()
	older, younger := identity(r), identity(r)
	var o1, y1 int64
	for i := range 4 {
		o := h.photo(day(2015, 1+i), variant(r, older, 0.02), 6)
		y := h.photo(day(2020, 1+i), variant(r, younger, 0.02), 6)
		if i == 0 {
			o1, y1 = o, y
		}
	}
	po, _ := h.s.NameGroup(ctx, h.group(o1), 0, "Older")
	py, _ := h.s.NameGroup(ctx, h.group(y1), 0, "Younger")
	if err := h.s.SetRelation(ctx, po, py, KinSibling, true); err != nil {
		t.Fatal(err)
	}
	// The older one at 8, a bit unlike: offered as them, and their
	// younger sibling (who'd be 3, so the age rules them out) as well.
	f := h.photo(day(2017, 20), variant(r, older, 0.1), 8)
	for _, m := range h.s.FaceMatches(ctx, 50, MatchOptions{}) {
		if m.Face.ID != f {
			continue
		}
		if m.Person != po {
			t.Fatalf("suggested %s", m.Name)
		}
		for _, o := range m.Others {
			if o.Person == py && o.Kin == "sibling" {
				return
			}
		}
		t.Fatalf("sibling not offered: %+v", m.Others)
	}
	t.Fatal("face not in Match")
}

// Two people photographed side by side on many days are asked about, as
// partners when ages don't say otherwise; once answered, not again.
func TestGuessRelations(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(141, 142))
	ctx := context.Background()
	alice, bob, carol := identity(r), identity(r), identity(r)
	var b1, k1, h1 int64
	for i := range 10 {
		b, k := h.side(day(2000, 1+i), variant(r, alice, 0.02), variant(r, bob, 0.02), 60, 62)
		if i == 0 {
			b1, k1 = b, k
		}
	}
	for i := range 20 {
		if f := h.photo(day(2000, 40+i), variant(r, carol, 0.02), 45); i == 0 {
			h1 = f
		}
	}
	pb, _ := h.s.NameGroup(ctx, h.group(b1), 0, "Alice")
	pk, _ := h.s.NameGroup(ctx, h.group(k1), 0, "Bob")
	h.s.NameGroup(ctx, h.group(h1), 0, "Carol")
	gs := h.s.GuessRelations(ctx, 10)
	if len(gs) != 1 || gs[0].Kind != KinPartner || gs[0].Days != 10 || gs[0].Image == 0 {
		t.Fatalf("guesses %+v", gs)
	}
	if !((gs[0].A == pb && gs[0].B == pk) || (gs[0].A == pk && gs[0].B == pb)) {
		t.Fatalf("guessed about %d and %d", gs[0].A, gs[0].B)
	}
	if err := h.s.SetAsked(ctx, pk, pb, true); err != nil {
		t.Fatal(err)
	}
	if gs := h.s.GuessRelations(ctx, 10); len(gs) != 0 {
		t.Errorf("asked again: %+v", gs)
	}
}

// An entered relation the ages can't square with is asked again: a parent
// born after the child, a son born to a ten-year-old, siblings 30 years
// apart.
func TestAgeWrong(t *testing.T) {
	for _, c := range []struct {
		kind         string
		elder, young float32
		want         bool
	}{
		{KinParent, 1958, 1981, true},
		{KinParent, 1958, 1962, false}, // ages unsure: let it be
		{KinChild, 1958, 1981, false},
		{KinChild, 1975, 1980, true},
		{KinSibling, 1950, 1980, true},
		{KinSibling, 1950, 1962, false},
		{KinPartner, 0, 1980, false}, // age unknown
	} {
		if got := ageWrong(c.kind, RelGuess{ABirth: c.elder, BBirth: c.young}); got != c.want {
			t.Errorf("%s %v/%v: wrong %v, want %v", c.kind, c.elder, c.young, got, c.want)
		}
	}
}

// People have partners one after another: with a partner known, someone
// photographed as a pair with them years before is asked about as an
// earlier partner; someone with them in the same years is a clash.
func TestGuessPartnersOverTime(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(151, 152))
	ctx := context.Background()
	dan, ex, now, other := identity(r), identity(r), identity(r), identity(r)
	var v1, v2, e1, n1, o1 int64
	for i := range 10 {
		v, e := h.side(day(2000, 1+i), variant(r, dan, 0.02), variant(r, ex, 0.02), 30, 29)
		w, n := h.side(day(2010, 1+i), variant(r, dan, 0.02), variant(r, now, 0.02), 40, 38)
		_, o := h.side(day(2010, 1+i), variant(r, dan, 0.02), variant(r, other, 0.02), 40, 41)
		if i == 0 {
			v1, v2, e1, n1, o1 = v, w, e, n, o
		}
	}
	pv, _ := h.s.NameGroup(ctx, h.group(v1), 0, "Dan")
	if h.group(v2) != h.group(v1) {
		h.s.NameGroup(ctx, h.group(v2), pv, "") // Dan ten years on
	}
	h.s.NameGroup(ctx, h.group(e1), 0, "Ex")
	pn, _ := h.s.NameGroup(ctx, h.group(n1), 0, "Now")
	h.s.NameGroup(ctx, h.group(o1), 0, "Other")
	if err := h.s.SetRelation(ctx, pv, pn, KinPartner, true); err != nil {
		t.Fatal(err)
	}
	got := map[string]RelGuess{}
	for _, g := range h.s.GuessRelations(ctx, 20) {
		got[g.AName+"+"+g.BName], got[g.BName+"+"+g.AName] = g, g
	}
	if g, ok := got["Dan+Ex"]; !ok || g.Kind != KinPartner || g.Then != "before" || g.ThenOf != "Dan" || g.ThenWith != "Now" || g.Clash != "" {
		t.Errorf("earlier partner: %+v (found %v)", g, ok)
	}
	if g, ok := got["Dan+Other"]; !ok || g.Kind == KinPartner && g.Clash == "" {
		t.Errorf("a partner at the same time as Now, no clash: %+v", g)
	}
}

// Friends are kept on both sides, and nothing follows from them: a
// friend's parent is no one's grandparent, a friend's friend no friend.
func TestFriends(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(161, 162))
	ctx := context.Background()
	name := func(n string) int64 {
		t.Helper()
		pid, err := h.s.NameGroup(ctx, h.group(h.photo(day(2010, len(n)), identity(r), 12)), 0, n)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	kid, pal, mum, other := name("Kid"), name("Pal"), name("Mum"), name("Otherkid")
	for _, rel := range []struct {
		a, b int64
		kind string
	}{{kid, pal, KinFriend}, {pal, other, KinFriend}, {pal, mum, KinParent}} {
		if err := h.s.SetRelation(ctx, rel.a, rel.b, rel.kind, true); err != nil {
			t.Fatal(err)
		}
	}
	h.s.mu.RLock()
	fam := h.s.family()
	h.s.mu.RUnlock()
	if fam[kid][pal] != KinFriend || fam[pal][kid] != KinFriend {
		t.Errorf("friends: %v / %v", fam[kid], fam[pal])
	}
	if _, ok := fam[kid][mum]; ok {
		t.Errorf("a friend's mother is related: %v", fam[kid])
	}
	if _, ok := fam[kid][other]; ok {
		t.Errorf("a friend's friend is a friend: %v", fam[kid])
	}
	if err := h.s.SetRelation(ctx, pal, kid, KinFriend, false); err != nil {
		t.Fatal(err)
	}
	h.s.mu.RLock()
	defer h.s.mu.RUnlock()
	if _, ok := h.s.family()[kid][pal]; ok {
		t.Error("still friends after removing it")
	}
}

// Only the closest ties are entered; the rest follow. Mum has Ann with
// Dad1 and Ben with Dad2, who is now her partner and has Cat from before:
// Ann and Ben are half-siblings, Dad2 is Ann's stepfather and Cat her
// stepsister. Mum's sister Sis has Dan: Sis is their aunt, Dan their
// cousin. Nobody gets a third parent.
func TestBlendedFamily(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(171, 172))
	ctx := context.Background()
	n := 0
	name := func(nm string) int64 {
		t.Helper()
		n++
		pid, err := h.s.NameGroup(ctx, h.group(h.photo(day(2010, n), identity(r), 30)), 0, nm)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	mum, dad1, dad2, ann, ben, cat, sis, dan, ex, gran := name("Mum"), name("Dad1"), name("Dad2"), name("Ann"), name("Ben"), name("Cat"), name("Sis"), name("Dan"), name("Ex"), name("Gran")
	rel := func(x, y int64, kind string) {
		t.Helper()
		if err := h.s.SetRelation(ctx, x, y, kind, true); err != nil {
			t.Fatal(err)
		}
	}
	rel(ann, mum, KinParent)
	rel(ann, dad1, KinParent)
	rel(ben, mum, KinParent)
	rel(ben, dad2, KinParent)
	rel(cat, dad2, KinParent)
	rel(cat, ex, KinParent) // without her, Cat and Ben would be full siblings: no parent known differs
	rel(mum, dad2, KinPartner)
	rel(mum, sis, KinSibling)
	rel(dan, sis, KinParent)
	rel(mum, gran, KinParent)
	h.s.mu.RLock()
	fam := h.s.family()
	h.s.mu.RUnlock()
	for _, c := range []struct {
		x, y int64
		want string
		why  string
	}{
		{ann, ben, KinHalfSibling, "one parent in common"},
		{ann, dad2, KinStepParent, "mum's partner"},
		{dad2, ann, KinStepChild, "and the other way"},
		{ann, cat, KinStepSibling, "the stepfather's daughter"},
		{ben, cat, KinHalfSibling, "same father, mothers differ"},
		{ann, sis, KinAuntUncle, "mum's sister"},
		{sis, ben, KinNieceNephew, "her sister's son"},
		{ann, dan, KinCousin, "children of sisters"},
		{dan, ben, KinCousin, "and the other way"},
		{ann, dad1, KinParent, "entered"},
		{dad2, sis, KinSiblingInLaw, "his partner's sister"},
		{sis, dad2, KinSiblingInLaw, "her sister's partner"},
		{dad2, gran, KinParentInLaw, "his partner's mother"},
		{gran, dad2, KinChildInLaw, "her daughter's partner"},
		{dad2, dan, KinNieceNephew, "his partner's nephew"},
		{sis, gran, KinParent, "siblings share their parents"},
	} {
		if got := fam[c.x][c.y]; got != c.want {
			t.Errorf("%s: got %q, want %q", c.why, got, c.want)
		}
	}
	for _, x := range []int64{ann, ben, cat} {
		p := 0
		for _, k := range fam[x] {
			if k == KinParent {
				p++
			}
		}
		if p > 2 {
			t.Errorf("%d has %d parents: %v", x, p, fam[x])
		}
	}
}

// Family through others, however distant (second cousins, a partner's
// family, a partner's sibling's partner), are never asked about: their
// links say how they're related. Friends join no one.
func TestLinked(t *testing.T) {
	known := map[int64]map[int64]string{}
	rel := func(a, b int64, kind, back string) {
		for _, x := range [][2]int64{{a, b}, {b, a}} {
			if known[x[0]] == nil {
				known[x[0]] = map[int64]string{}
			}
		}
		known[a][b], known[b][a] = kind, back
	}
	// 1 and 2 are siblings; 3 is 1's grandchild, 4 is 2's grandchild.
	rel(5, 1, KinParent, KinChild)
	rel(3, 5, KinParent, KinChild)
	rel(6, 2, KinParent, KinChild)
	rel(4, 6, KinParent, KinChild)
	rel(1, 2, KinSibling, KinSibling)
	rel(7, 3, KinPartner, KinPartner)  // 7 is 3's partner
	rel(8, 9, KinSibling, KinSibling)  // 8 and 9 are siblings, unrelated to the rest
	rel(10, 8, KinPartner, KinPartner) // 10 is 8's partner
	rel(11, 9, KinPartner, KinPartner) // 11 is 9's partner
	rel(12, 3, KinFriend, KinFriend)   // 12 is 3's friend
	for _, c := range []struct {
		a, b int64
		want bool
		why  string
	}{
		{3, 4, true, "second cousins"},
		{7, 4, true, "a partner's second cousin"},
		{10, 11, true, "partners of siblings"},
		{7, 8, false, "strangers"},
		{12, 4, false, "a friend joins no one"},
	} {
		if got := linked(known, c.a, c.b); got != c.want {
			t.Errorf("%s: linked %v, want %v", c.why, got, c.want)
		}
	}
}

// A parent's partner, with only that one parent entered, counts as a
// stepparent but may be the other parent: asked. With both parents
// entered, not.
func TestGuessMaybeParent(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(181, 182))
	ctx := context.Background()
	mum, pat, kid := identity(r), identity(r), identity(r)
	var m1, p1, k1 int64
	for i := range 10 {
		k, m := h.side(day(2010, 1+i), variant(r, kid, 0.02), variant(r, mum, 0.02), 5, 35)
		_, p := h.side(day(2010, 11+i), variant(r, kid, 0.02), variant(r, pat, 0.02), 5, 37)
		if i == 0 {
			m1, p1, k1 = m, p, k
		}
	}
	pm, _ := h.s.NameGroup(ctx, h.group(m1), 0, "Mum")
	pp, _ := h.s.NameGroup(ctx, h.group(p1), 0, "Pat")
	pk, _ := h.s.NameGroup(ctx, h.group(k1), 0, "Kid")
	for _, r := range []struct {
		a, b int64
		kind string
	}{{pk, pm, KinParent}, {pm, pp, KinPartner}} {
		if err := h.s.SetRelation(ctx, r.a, r.b, r.kind, true); err != nil {
			t.Fatal(err)
		}
	}
	asked := func() bool {
		for _, g := range h.s.GuessRelations(ctx, 20) {
			if g.A == pp && g.B == pk && g.Kind == KinChild {
				return true
			}
		}
		return false
	}
	if !asked() {
		t.Errorf("Pat not asked about as Kid's parent: %+v", h.s.GuessRelations(ctx, 20))
	}
	other, _ := h.s.NameGroup(ctx, h.group(h.photo(day(2000, 1), identity(r), 30)), 0, "Dad")
	if err := h.s.SetRelation(ctx, pk, other, KinParent, true); err != nil {
		t.Fatal(err)
	}
	if asked() {
		t.Error("asked about a stepparent, both parents entered")
	}
}

// Four sisters with the same parents, and a half-brother sharing only
// their father: entered as one sister's half-brother, he's all four's,
// and his mother is none of theirs, though theirs isn't in the library.
func TestHalfSiblingEntered(t *testing.T) {
	h := newHarness(t)
	r := rand.New(rand.NewPCG(191, 192))
	ctx := context.Background()
	n := 0
	name := func(nm string) int64 {
		t.Helper()
		n++
		pid, err := h.s.NameGroup(ctx, h.group(h.photo(day(2010, n), identity(r), 30)), 0, nm)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	s1, s2, s3, s4, bro, hisMum, dad, kid := name("S1"), name("S2"), name("S3"), name("S4"), name("Bro"), name("HisMum"), name("Dad"), name("Kid")
	rel := func(x, y int64, kind string) {
		t.Helper()
		if err := h.s.SetRelation(ctx, x, y, kind, true); err != nil {
			t.Fatal(err)
		}
	}
	for _, s := range []int64{s2, s3, s4} {
		rel(s1, s, KinSibling)
	}
	rel(s1, bro, KinSibling)     // first entered as a brother,
	rel(s1, bro, KinHalfSibling) // then put right: replaces it
	rel(bro, hisMum, KinParent)
	rel(bro, dad, KinParent)
	rel(s2, dad, KinParent)
	rel(kid, s3, KinParent)
	h.s.mu.RLock()
	fam := h.s.family()
	h.s.mu.RUnlock()
	for _, c := range []struct {
		x, y int64
		want string
		why  string
	}{
		{s1, bro, KinHalfSibling, "entered"},
		{s4, bro, KinHalfSibling, "her sister's half-brother"},
		{s1, s4, KinSibling, "sisters"},
		{s4, dad, KinParent, "sisters share their father"},
		{kid, bro, KinAuntUncle, "a mother's half-brother is an uncle"},
		{kid, dad, KinGrandparent, "grandfather"},
	} {
		if got := fam[c.x][c.y]; got != c.want {
			t.Errorf("%s: got %q, want %q", c.why, got, c.want)
		}
	}
	for _, s := range []int64{s1, s2, s3, s4} {
		if k, ok := fam[s][hisMum]; ok {
			t.Errorf("his mother is a sister's %s", k)
		}
	}
	if k, ok := fam[kid][hisMum]; ok {
		t.Errorf("his mother is the niece's %s", k)
	}
}
