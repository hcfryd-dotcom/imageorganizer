package store

import (
	"cmp"
	"context"
	"errors"
	"slices"

	bolt "go.etcd.io/bbolt"
)

// Relation kinds, as seen from a person: their parent, child, sibling or
// partner.
const (
	KinParent  = "parent"
	KinChild   = "child"
	KinSibling = "sibling"
	KinPartner = "partner"
	// Friend: not family, nothing follows from it; but friends are
	// photographed together, as relatives are.
	KinFriend = "friend"
	// Worked out, not entered: a parent's parent, and the other way.
	KinGrandparent  = "grandparent"
	KinGrandchild   = "grandchild"
	KinHalfSibling  = "half-sibling" // one parent in common: entered, or each with two known
	KinStepParent   = "step-parent"  // a parent's partner
	KinStepChild    = "step-child"
	KinStepSibling  = "step-sibling" // a step-parent's child
	KinAuntUncle    = "aunt-uncle"   // a parent's sibling
	KinNieceNephew  = "niece-nephew"
	KinCousin       = "cousin"         // a parent's sibling's child
	KinParentInLaw  = "parent-in-law"  // a partner's parent
	KinChildInLaw   = "child-in-law"   // a child's partner
	KinSiblingInLaw = "sibling-in-law" // a partner's sibling, a sibling's partner
)

// Relative is someone a person is related to, for the person page.
type Relative struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`               // what they are to the person: parent, child, sibling, partner, grandparent, grandchild
	Word     string `json:"word"`               // the same with their gender, when known: mother, son, …
	Inferred bool   `json:"inferred,omitempty"` // follows from the relations entered
}

var ErrBadRelation = errors.New("unknown relation, or a person related to themselves")

// relatives lists the people pid is related to, entered or worked out,
// and how (what they are to pid). Call with s.mu held.
func (s *Store) relatives(pid int64) map[int64]string { return s.family()[pid] }

// entered is every person's relatives as the user entered them (both
// sides); call with s.mu held.
func (s *Store) entered() map[int64]map[int64]string {
	out := map[int64]map[int64]string{}
	add := func(a, b int64, kind string) {
		if s.people[a] == nil || s.people[b] == nil || a == b {
			return
		}
		if out[a] == nil {
			out[a] = map[int64]string{}
		}
		out[a][b] = kind
	}
	for id, p := range s.people {
		for _, q := range p.Parents {
			add(id, q, KinParent)
			add(q, id, KinChild)
		}
		for _, q := range p.Partners {
			add(id, q, KinPartner)
		}
		for _, q := range p.Siblings {
			add(id, q, KinSibling)
		}
		for _, q := range p.HalfSiblings {
			add(id, q, KinHalfSibling)
		}
		for _, q := range p.Friends {
			add(id, q, KinFriend)
		}
	}
	return out
}

// family is every person's relatives: those entered (parents, partners,
// siblings, friends), and those that follow from them. Siblings of
// siblings are siblings; siblings entered share their parents, as far as
// that leaves no one with three; children of the same parent are siblings,
// or half-siblings when each has two parents and one differs. A parent's
// parent is a grandparent, a parent's partner (not a parent) a
// step-parent and their children step-siblings, a parent's sibling an
// aunt or uncle and their children cousins; a partner's family are
// in-laws. Friends lead to no one. Call
// with s.mu held.
func (s *Store) family() map[int64]map[int64]string {
	in := s.entered()
	// Sibling sets: full siblings entered, who share their parents.
	// (Children of the same parent may be half-siblings: they share no
	// more than that parent.)
	root := map[int64]int64{}
	var find func(int64) int64
	find = func(a int64) int64 {
		if r, ok := root[a]; ok && r != a {
			root[a] = find(r)
			return root[a]
		}
		root[a] = a
		return a
	}
	union := func(a, b int64) { root[find(a)] = find(b) }
	entered := map[int64][]int64{} // person -> parents entered
	for a, rel := range in {
		for b, kind := range rel {
			switch kind {
			case KinSibling:
				union(a, b)
			case KinParent:
				entered[a] = append(entered[a], b)
			}
		}
	}
	for _, ps := range entered {
		slices.Sort(ps)
	}
	sets := map[int64][]int64{}
	for a := range root {
		sets[find(a)] = append(sets[find(a)], a)
	}
	// Parents: those entered, filled up from the sibling set when all of
	// it agrees on two at most (with three or more, some are
	// half-siblings: only what's entered counts).
	parents := map[int64][]int64{}
	for id := range s.people {
		ps := slices.Clone(entered[id])
		if _, ok := root[id]; ok && len(ps) < 2 {
			var all []int64
			for _, m := range sets[find(id)] {
				for _, p := range entered[m] {
					if !slices.Contains(all, p) {
						all = append(all, p)
					}
				}
			}
			if len(all) <= 2 {
				ps = all
			}
		}
		slices.Sort(ps)
		if len(ps) > 0 {
			parents[id] = ps
		}
	}
	children := map[int64][]int64{}
	for id, ps := range parents {
		for _, p := range ps {
			children[p] = append(children[p], id)
		}
	}
	shared := func(a, b int64) int {
		n := 0
		for _, p := range parents[a] {
			if slices.Contains(parents[b], p) {
				n++
			}
		}
		return n
	}
	// Brothers and sisters, full or half.
	sibs := map[int64]map[int64]string{}
	addSib := func(a, b int64, kind string) {
		if a == b {
			return
		}
		if sibs[a] == nil {
			sibs[a] = map[int64]string{}
		}
		if _, ok := sibs[a][b]; !ok || kind == KinSibling {
			sibs[a][b] = kind
		}
	}
	// Half-siblings entered: so are the full siblings of each, to the
	// other's.
	half := map[[2]int64]bool{}
	setOf := func(a int64) []int64 {
		if _, ok := root[a]; ok {
			return sets[find(a)]
		}
		return []int64{a}
	}
	for a, rel := range in {
		for b, kind := range rel {
			if kind == KinHalfSibling {
				for _, x := range setOf(a) {
					for _, y := range setOf(b) {
						half[[2]int64{x, y}], half[[2]int64{y, x}] = true, true
					}
				}
			}
		}
	}
	classify := func(a, b int64) {
		switch n := shared(a, b); {
		case a == b:
		case in[a][b] == KinSibling || in[a][b] == KinHalfSibling:
			addSib(a, b, in[a][b]) // as entered
		case half[[2]int64{a, b}]:
			addSib(a, b, KinHalfSibling)
		case n > 0 && (n == len(parents[a]) || n == len(parents[b])):
			addSib(a, b, KinSibling) // no parent known that differs
		case n > 0:
			addSib(a, b, KinHalfSibling)
		case find(a) == find(b):
			addSib(a, b, KinSibling) // siblings of siblings
		}
	}
	for _, set := range sets {
		for _, a := range set {
			for _, b := range set {
				classify(a, b)
			}
		}
	}
	for k := range half {
		classify(k[0], k[1])
	}
	for _, kids := range children {
		for _, a := range kids {
			for _, b := range kids {
				classify(a, b)
			}
		}
	}
	out := map[int64]map[int64]string{}
	add := func(a, b int64, kind string) {
		if a == b || s.people[a] == nil || s.people[b] == nil {
			return
		}
		if out[a] == nil {
			out[a] = map[int64]string{}
		}
		if _, ok := out[a][b]; !ok {
			out[a][b] = kind
		}
	}
	for a, rel := range in { // entered first: they win over worked out
		for b, kind := range rel {
			add(a, b, kind)
		}
	}
	// Closest first: whoever is two things to someone is the closer one.
	for id, rel := range sibs {
		for q, kind := range rel {
			add(id, q, kind)
		}
	}
	for id, ps := range parents {
		for _, p := range ps {
			add(id, p, KinParent)
			add(p, id, KinChild)
		}
	}
	for id, ps := range parents {
		for _, p := range ps {
			for _, g := range parents[p] {
				add(id, g, KinGrandparent)
				add(g, id, KinGrandchild)
			}
		}
	}
	for id, ps := range parents {
		for _, p := range ps {
			// A parent's partner, not a parent: a step-parent, and their
			// children (sharing no parent) step-siblings.
			for q, kind := range in[p] {
				if kind != KinPartner || slices.Contains(ps, q) {
					continue
				}
				add(id, q, KinStepParent)
				add(q, id, KinStepChild)
				for _, c := range children[q] {
					if shared(id, c) == 0 {
						add(id, c, KinStepSibling)
						add(c, id, KinStepSibling)
					}
				}
			}
			// A parent's brothers and sisters, and their children.
			for u := range sibs[p] {
				add(id, u, KinAuntUncle)
				add(u, id, KinNieceNephew)
				for _, c := range children[u] {
					add(id, c, KinCousin)
					add(c, id, KinCousin)
				}
			}
		}
	}
	// In-laws, through a partner: their parents, brothers and sisters,
	// and those's children (a nephew by marriage is a nephew).
	for a, rel := range in {
		for p, kind := range rel {
			if kind != KinPartner {
				continue
			}
			for _, q := range parents[p] {
				add(a, q, KinParentInLaw)
				add(q, a, KinChildInLaw)
			}
			for sb := range sibs[p] {
				add(a, sb, KinSiblingInLaw)
				add(sb, a, KinSiblingInLaw)
				for _, c := range children[sb] {
					add(a, c, KinNieceNephew)
					add(c, a, KinAuntUncle)
				}
			}
		}
	}
	return out
}

// SetRelation records (on) or removes that b is a's parent, child,
// sibling or partner, on both sides.
func (s *Store) SetRelation(ctx context.Context, a, b int64, kind string, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pa, pb := s.people[a], s.people[b]
	if pa == nil || pb == nil {
		return ErrNotFound
	}
	if a == b {
		return ErrBadRelation
	}
	ua, ub := *pa, *pb
	set := func(list []int64, id int64) []int64 {
		list = slices.DeleteFunc(slices.Clone(list), func(x int64) bool { return x == id })
		if on {
			list = append(list, id)
		}
		return list
	}
	switch kind {
	case KinParent: // b is a's parent
		ua.Parents = set(ua.Parents, b)
		ub.Parents = slices.DeleteFunc(slices.Clone(ub.Parents), func(x int64) bool { return x == a }) // not both ways
	case KinChild: // b is a's child
		ub.Parents = set(ub.Parents, a)
		ua.Parents = slices.DeleteFunc(slices.Clone(ua.Parents), func(x int64) bool { return x == b })
	case KinSibling:
		ua.Siblings, ub.Siblings = set(ua.Siblings, b), set(ub.Siblings, a)
		if on { // one or the other
			ua.HalfSiblings = slices.DeleteFunc(slices.Clone(ua.HalfSiblings), func(x int64) bool { return x == b })
			ub.HalfSiblings = slices.DeleteFunc(slices.Clone(ub.HalfSiblings), func(x int64) bool { return x == a })
		}
	case KinHalfSibling:
		ua.HalfSiblings, ub.HalfSiblings = set(ua.HalfSiblings, b), set(ub.HalfSiblings, a)
		if on {
			ua.Siblings = slices.DeleteFunc(slices.Clone(ua.Siblings), func(x int64) bool { return x == b })
			ub.Siblings = slices.DeleteFunc(slices.Clone(ub.Siblings), func(x int64) bool { return x == a })
		}
	case KinPartner:
		ua.Partners, ub.Partners = set(ua.Partners, b), set(ub.Partners, a)
	case KinFriend:
		ua.Friends, ub.Friends = set(ua.Friends, b), set(ub.Friends, a)
	default:
		return ErrBadRelation
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := put(tx, bucketPeople, ua.ID, &ua); err != nil {
			return err
		}
		return put(tx, bucketPeople, ub.ID, &ub)
	}); err != nil {
		return err
	}
	*pa, *pb = ua, ub
	return nil
}

// relativesView is a person's relatives (from family; entered ones from
// in) for the API, closest first.
func (s *Store) relativesView(rel, in map[int64]string, gender func(int64) string) []Relative {
	var out []Relative
	for id, kind := range rel {
		out = append(out, Relative{ID: id, Name: s.people[id].Name, Kind: kind, Word: kinWord(kind, gender(id)), Inferred: in[id] != kind})
	}
	order := map[string]int{KinPartner: 0, KinParent: 1, KinChild: 2, KinSibling: 3, KinGrandparent: 4, KinGrandchild: 5,
		KinHalfSibling: 3, KinStepParent: 1, KinStepChild: 2, KinStepSibling: 3, KinAuntUncle: 6, KinNieceNephew: 7, KinCousin: 8,
		KinParentInLaw: 9, KinChildInLaw: 9, KinSiblingInLaw: 9, KinFriend: 10}
	slices.SortFunc(out, func(a, b Relative) int {
		// Those entered first, then what follows from them.
		return cmp.Or(cmp.Compare(boolInt(a.Inferred), boolInt(b.Inferred)), cmp.Compare(order[a.Kind], order[b.Kind]), cmp.Compare(a.Name, b.Name))
	})
	return out
}

// mergeRelations moves from's relationships onto into (merging people),
// and points everyone who listed from at into. It returns the people
// records to save. Call with s.mu held, before from is removed.
func (s *Store) mergeRelations(from, into int64) []personRec {
	swap := func(list []int64) ([]int64, bool) {
		if !slices.Contains(list, from) {
			return list, false
		}
		out := slices.DeleteFunc(slices.Clone(list), func(x int64) bool { return x == from })
		if !slices.Contains(out, into) {
			out = append(out, into)
		}
		return out, true
	}
	var out []personRec
	for id, p := range s.people {
		if id == from {
			continue
		}
		u := *p
		var c1, c2, c3, c4, c5 bool
		u.Parents, c1 = swap(u.Parents)
		u.Partners, c2 = swap(u.Partners)
		u.Siblings, c3 = swap(u.Siblings)
		u.Friends, c4 = swap(u.Friends)
		u.HalfSiblings, c5 = swap(u.HalfSiblings)
		if id == into {
			f := s.people[from]
			for _, l := range []struct {
				dst *[]int64
				src []int64
			}{{&u.Parents, f.Parents}, {&u.Partners, f.Partners}, {&u.Siblings, f.Siblings}, {&u.Friends, f.Friends}, {&u.HalfSiblings, f.HalfSiblings}} {
				for _, q := range l.src {
					if q != into && !slices.Contains(*l.dst, q) {
						*l.dst = append(*l.dst, q)
						c1 = true
					}
				}
			}
			// Not their own relative.
			for _, l := range []*[]int64{&u.Parents, &u.Partners, &u.Siblings, &u.Friends, &u.HalfSiblings} {
				*l = slices.DeleteFunc(*l, func(x int64) bool { return x == into })
			}
		}
		if c1 || c2 || c3 || c4 || c5 {
			out = append(out, u)
		}
	}
	return out
}

const (
	kinPhoto      = 0.06 // a face's bonus for a person when a relative of theirs is in the photo
	kinNext       = 0.12 // or right beside the face: couples sit together, children by their parents
	kinSim        = 0.25 // likeness enough for a face (or group) with a relative to be offered
	kinGroupBonus = 0.10 // a group's bonus, at a relative in all its photos
	kinGroupShare = 0.30 // share of a group's photos with a relative, to offer a less alike person
)

// whoIn is who is in a photo by name (confirmed or tagged faces), other
// than face skip. Call with s.mu held.
func (s *Store) whoIn(img, skip int64) map[int64]*faceRec {
	out := map[int64]*faceRec{}
	for _, fid := range s.facesByImage[img] {
		f := s.faces[fid]
		if f == nil || f.ID == skip || f.NotFace || f.Picture {
			continue
		}
		p := f.Tagged
		if c := s.clusters[f.ClusterID]; p == 0 && c != nil && c.PersonID != 0 && !c.Auto {
			p = c.PersonID
		}
		if p != 0 {
			out[p] = f
		}
	}
	return out
}

// beside reports whether two faces in a photo are next to each other:
// about level, and within a couple of face widths.
func beside(a, b *faceRec) bool {
	w := max(a.Box[2]-a.Box[0], b.Box[2]-b.Box[0])
	h := max(a.Box[3]-a.Box[1], b.Box[3]-b.Box[1])
	dx := abs32((a.Box[0]+a.Box[2])/2 - (b.Box[0]+b.Box[2])/2)
	dy := abs32((a.Box[1]+a.Box[3])/2 - (b.Box[1]+b.Box[3])/2)
	return dx <= 2.5*w && dy <= 1.2*h
}

// kinNear is how face f's photo speaks for it being the person with
// relatives rel: one of them in the photo (kinPhoto), or right beside it
// (kinNext); who, and whether beside it. Call with s.mu held.
func (s *Store) kinNear(f *faceRec, rel map[int64]string) (bonus float32, who int64, next bool) {
	for q, of := range s.whoIn(f.ImageID, f.ID) {
		if _, ok := rel[q]; !ok {
			continue
		}
		b := float32(kinPhoto)
		if beside(f, of) {
			b = kinNext
		}
		if b > bonus {
			bonus, who, next = b, q, b == kinNext
		}
	}
	return bonus, who, next
}

// kinWith is kinNear's bonus with the people in the photo, here, known.
func kinWith(f *faceRec, here map[int64]*faceRec, rel map[int64]string) float32 {
	var bonus float32
	for q, of := range here {
		if _, ok := rel[q]; !ok {
			continue
		}
		b := float32(kinPhoto)
		if beside(f, of) {
			b = kinNext
		}
		bonus = max(bonus, b)
	}
	return bonus
}

// kinWhy says who backs a face up: "next to Bob (partner)".
func (s *Store) kinWhy(who int64, next bool, rel map[int64]string) string {
	p := s.people[who]
	if p == nil {
		return ""
	}
	w := "with "
	if next {
		w = "next to "
	}
	return w + p.Name + " (" + kinWord(rel[who], "") + ")"
}

// kinGroup is the share of a group's faces (by photo) with a relative of
// the person in the photo, and why: "with Bob (partner) in 3 of 5
// photos". Call with s.mu held.
func (s *Store) kinGroup(faces []*faceRec, rel map[int64]string) (float32, string) {
	if len(rel) == 0 || len(faces) == 0 {
		return 0, ""
	}
	n := 0
	count := map[int64]int{}
	for _, f := range faces {
		if b, who, _ := s.kinNear(f, rel); b > 0 {
			n++
			count[who]++
		}
	}
	if n == 0 {
		return 0, ""
	}
	var best int64
	for q, c := range count {
		if c > count[best] || c == count[best] && q < best {
			best = q
		}
	}
	return float32(n) / float32(len(faces)), s.kinWhy(best, false, rel) + " in " + itoa(n) + " of " + itoa(len(faces)) + " photos"
}

// FaceSuggestions lists who one face may be, likeliest first: the face
// itself compared with each person (its group may be mixed, and so
// clash with the right person as a whole), leaving out people already
// named in the photo, with its own age checked and relatives of theirs
// in the photo as a help. For the photo viewer.
func (s *Store) FaceSuggestions(ctx context.Context, faceID int64) []Suggestion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f := s.faces[faceID]
	if f == nil || len(f.emb) == 0 {
		return []Suggestion{}
	}
	m := s.newMatcher()
	out := s.faceSuggestions(m, f, s.personCovers(s.facesByGroup()))
	out = out[:min(3, len(out))]
	labelKin(out, m.fam, m.idx.genderOf)
	return out
}

// faceSuggestions is everyone face f may be, the likeliest first, as the
// photo's card offers them; "More of …?" offers a face only to the first
// (cardTop). Call with s.mu held; covers may be nil.
func (s *Store) faceSuggestions(m *matcher, f *faceRec, covers map[int64]string) []Suggestion {
	out := []Suggestion{}
	here := s.whoIn(f.ImageID, f.ID)
	idx, fam := m.idx, m.fam
	likes := m.likes(f)
	by, byOK := s.impliedBirth(f)
	for pid, pi := range idx {
		sc, ok := likes[pid]
		if !ok || m.ruledOut(f, pid, here) {
			continue
		}
		kin, who, next := s.kinNear(f, fam[pid])
		if sc < candidateMin && (kin == 0 || sc < kinSim || !byOK) { // on a relative's account only with a known age
			continue
		}
		score := m.score(f, pid, sc, here)
		if score < batchFloor {
			continue // the age or gender says otherwise
		}
		sg := Suggestion{ID: pid, Name: pi.p.Name, Cover: covers[pid], Score: min(1, score), rank: score}
		if genderMismatch(pi.gender, f) {
			sg.Off = offWord(pi.gender)
		}
		if kin > 0 {
			sg.Why = s.kinWhy(who, next, fam[pid])
		}
		if byOK && !pi.p.Picture {
			if l, ok := pi.ages.looksAt(by + f.Age); ok {
				sg.Looked = l
			}
		}
		out = append(out, sg)
	}
	slices.SortFunc(out, func(a, b Suggestion) int { return cmp.Or(cmp.Compare(b.rank, a.rank), cmp.Compare(a.ID, b.ID)) })
	return out
}

// cardTop is whom the photo's card offers first for face f, looked at on
// its own: 0 if no one. Call with s.mu held.
func (s *Store) cardTop(m *matcher, f *faceRec) int64 {
	if out := s.faceSuggestions(m, f, nil); len(out) > 0 {
		return out[0].ID
	}
	return 0
}
