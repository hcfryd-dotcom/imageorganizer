package store

import (
	"cmp"
	"context"
	"math"
	"slices"
	"time"
)

// Discover (experiment): the people who matter, found before anyone is
// named. A person spans several groups (one per stretch of their life):
// groups fairly alike, with the same implied birth year, are chained into
// one person. The chains seen on the most days (gatherings, not one photo
// session) and over the most years come first: family and friends, not
// strangers at an event.
const (
	chainAlike   = 0.55 // a group this alike to a person's average face, birth year fitting, is them
	chainMinSize = 5    // groups smaller than this are left out (noise, and too many to compare)
	chainShown   = 14   // faces shown per person: one a year, the clearest
	stillYears   = 5    // years a chain must span to call it still
	stillAlike   = 0.65 // mean likeness of its faces that many years apart, for that
	chainShare   = 0.25 // share of its photos a group may share with them, for that
	chainWorth   = 3    // days a person must be photographed on to count in the Discover badge
	chainAlso    = 0.45 // a group this alike to one of a named person's confirmed groups is asked about as them
	chainClear   = 0.75 // a face shown must be this clear (visible: its light spread), not a dark or flat smudge
	chainLook    = 6    // faces of a year looked at for a clear one, the likeliest first
)

// Chain is a person not named yet, across their groups.
type Chain struct {
	Groups []int64    `json:"groups"`
	Faces  []FaceView `json:"faces"` // one a year, oldest first
	Photos int        `json:"photos"`
	Days   int        `json:"days"`
	From   int        `json:"from"` // years
	To     int        `json:"to"`
	Birth  float32    `json:"birth,omitempty"`
	// Person: someone named already whom these groups look like (another
	// stretch of their life): "Is this also …?"
	Person     int64  `json:"person,omitempty"`
	PersonName string `json:"person_name,omitempty"`
	// Still: the very same face over the years, about Age: a picture on
	// the wall, a statue, a poster, more likely than a person.
	Still bool    `json:"still,omitempty"`
	Age   float32 `json:"age,omitempty"`
	Alike float32 `json:"-"` // mean likeness of faces years apart
	// LastAge: how old they are in the last photos of them (by their
	// birth year, or else by how old they look; -1 not known): for
	// Discover to leave out, say, the children one doesn't know.
	LastAge float32 `json:"last_age"`
	// With: the named people they're photographed with most, and on how
	// many days with anyone named: someone often beside people you know
	// is likely someone you know.
	With     []string `json:"with,omitempty"`
	WithDays int      `json:"with_days,omitempty"`
	TopDays  int      `json:"top_days,omitempty"` // days with the one named person most with them
	// Clear: how many of their faces can be made out (big enough, surely
	// a face, not at the back): a cluster of grainy smudges is no one to
	// ask about.
	Clear int `json:"clear"`
	// Maybe: someone named whom most of the faces shown look most like
	// (at another age, say), and how alike on average: offered as an
	// answer.
	Maybe     int64   `json:"maybe,omitempty"`
	MaybeName string  `json:"maybe_name,omitempty"`
	MaybeLike float32 `json:"maybe_like,omitempty"`
	// LooksLike: someone named they look like, but who is in photos with
	// them: likely a relative, not offered as an answer.
	LooksLike string `json:"looks_like,omitempty"`
}

const (
	maybeLike  = 0.35 // average likeness of the faces shown, at least, to suggest someone
	maybeShare = 0.5  // share of the faces shown that must look most like them
)

// chainWith is how many named people a chain lists as company.
const chainWith = 3

// alsoRef is a named person Discover asks "Is this also …?" about.
type alsoRef struct {
	pi *personInfo
	us [][]float32 // their confirmed groups
}

// shares reports whether group c is often in the same photos as the
// person: then it's someone else. A few shared photos are allowed: a
// person's groups hold some stray faces of others (and a question is
// only a question).
func (n alsoRef) shares(c *cluster) bool {
	shared := 0
	for img := range c.imgs {
		if n.pi.imgs[img] {
			shared++
		}
	}
	return float32(shared) > chainShare*float32(len(c.imgs))
}

// chainSet is a person Discover asks about: unnamed groups chained, or one
// group that looks like a person named already (who).
type chainSet struct {
	groups []*cluster
	days   map[int64]bool
	years  map[int]bool
	n      int
	who    *personRec
}

// DiscoverCount is how many people Discover asks about who were
// photographed on chainWorth days or more, and how many relation, date
// and doubtful face questions wait.
func (s *Store) DiscoverCount(ctx context.Context) (people, relations int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sc := range s.discoverChains() {
		if len(sc.days) >= chainWorth {
			people++
		}
	}
	return people, len(s.guessRelations(0, false)) + len(s.doubtfulDates(0)) + len(s.doubtfulFaces(0, doubtOpts{which: doubtBoth}, true))
}

// DiscoverPeople lists the unnamed people who matter most: chains of unnamed
// groups, by the days they're photographed on.
func (s *Store) DiscoverPeople(ctx context.Context, limit int) []Chain {
	return s.discoverPeople(limit, nil)
}

// discoverPeople is DiscoverPeople, of the chains keep keeps (nil: all),
// called with s.mu held for reading and each chain's faces.
func (s *Store) discoverPeople(limit int, keep func(sc chainSet, faces map[int64][]*faceRec) bool) []Chain {
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.discoverChains()
	if keep != nil {
		var groups []*cluster
		for _, sc := range all {
			groups = append(groups, sc.groups...)
		}
		faces := facesOfGroups(s, groups, func(c *cluster) int64 { return c.id })
		all = slices.DeleteFunc(all, func(sc chainSet) bool { return !keep(sc, faces) })
	}
	all = all[:min(len(all), limit)]
	// Everyone named, to say whom a chain looks most like.
	refs := map[int64][][]float32{}
	for pid, pi := range s.personIndex() {
		for _, g := range pi.groups {
			refs[pid] = append(refs[pid], clusterUnit(g))
		}
	}
	var want []*cluster
	for _, sc := range all {
		want = append(want, sc.groups...)
	}
	faces := facesOfGroups(s, want, func(c *cluster) int64 { return c.id })
	out := make([]Chain, 0, len(all))
	for _, sc := range all {
		ch := Chain{Photos: sc.n, Days: len(sc.days)}
		if sc.who != nil {
			ch.Person, ch.PersonName = sc.who.ID, sc.who.Name
		}
		ch.With, ch.WithDays, ch.TopDays = s.chainCompany(sc.groups)
		ch.Clear = s.chainClear(sc.groups, faces)
		var births, nb float32
		// The clearest face of each year; background faces only when
		// there are no others (someone always at the back).
		cands := map[int][]*faceRec{}
		score := map[*faceRec]float32{}
		for _, c := range sc.groups {
			ch.Groups = append(ch.Groups, c.id)
			if b, _, ok := c.birth(); ok {
				births += b * float32(c.n)
				nb += float32(c.n)
			}
		}
		for _, bg := range []bool{false, true} {
			for _, c := range sc.groups {
				for _, f := range faces[c.id] {
					im := s.images[f.ImageID]
					if im == nil || f.bg && !bg {
						continue
					}
					y := 0 // undated
					if im.TakenAt != nil {
						y = time.Unix(*im.TakenAt, 0).UTC().Year()
					}
					score[f] = s.frontal(f) * min(1, s.facePx(f)/matchPx) * f.Score
					cands[y] = append(cands[y], f)
				}
			}
			if len(cands) > 0 {
				break
			}
		}
		// The clearest face of each year that is clear: a dark or flat
		// one (a photo of a print, a face in shadow) can't be judged; a
		// year with only such faces is left out.
		best := map[int]*faceRec{}
		for y, fs := range cands {
			slices.SortFunc(fs, func(a, b *faceRec) int { return cmp.Or(cmp.Compare(score[b], score[a]), cmp.Compare(a.ID, b.ID)) })
			for _, f := range fs[:min(len(fs), chainLook)] {
				if s.visible(f) >= chainClear {
					best[y] = f
					break
				}
			}
		}
		if len(best) > 1 {
			delete(best, 0) // an undated face only when there's nothing else
		}
		if nb > 0 {
			ch.Birth = births / nb
		}
		ch.Alike, ch.Age, ch.Still = s.stillness(sc.groups, faces)
		var years []int
		for y := range best {
			years = append(years, y)
		}
		slices.Sort(years)
		if len(years) > 0 && years[0] != 0 {
			ch.From, ch.To = years[0], years[len(years)-1]
		}
		switch {
		case ch.Birth > 0 && ch.To > 0:
			ch.LastAge = float32(ch.To) - ch.Birth
		case ch.Age > 0:
			ch.LastAge = ch.Age
		default:
			ch.LastAge = -1
		}
		step := (len(years) + chainShown - 1) / chainShown
		shown := map[*faceRec]bool{}
		days := map[int64]bool{}
		for i, y := range years {
			if i%step == 0 {
				shown[best[y]] = true
				days[takenDay(s.images[best[y].ImageID])] = true
			}
		}
		// Few years (a baby, someone seen for a while): more clear faces,
		// from other days, a year at a time, so there's enough to tell
		// them by, and to confirm.
		for more := true; more && len(shown) < chainShown; {
			more = false
			for _, y := range years {
				for _, f := range cands[y][:min(len(cands[y]), chainLook*3)] {
					d := takenDay(s.images[f.ImageID])
					if shown[f] || days[d] || s.visible(f) < chainClear {
						continue
					}
					shown[f], days[d], more = true, true, true
					break
				}
				if len(shown) >= chainShown {
					break
				}
			}
		}
		var picked []*faceRec
		for f := range shown {
			picked = append(picked, f)
		}
		taken := func(f *faceRec) int64 {
			if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil {
				return *im.TakenAt
			}
			return 0
		}
		slices.SortFunc(picked, func(a, b *faceRec) int { return cmp.Or(cmp.Compare(taken(a), taken(b)), cmp.Compare(a.ID, b.ID)) })
		for _, f := range picked {
			ch.Faces = append(ch.Faces, s.faceView(f))
		}
		ch.Maybe, ch.MaybeName, ch.MaybeLike = s.chainMaybe(picked, refs)
		out = append(out, ch)
	}
	return out
}

// chainClear counts the faces of groups that can be made out: big
// enough, surely a face (the detector's confidence), not at the back.
// Measured without reading crops. Call with s.mu held.
func (s *Store) chainClear(groups []*cluster, faces map[int64][]*faceRec) int {
	n := 0
	for _, c := range groups {
		for _, f := range faces[c.id] {
			if !f.bg && f.Score >= clearDet && s.facePx(f) >= clearPx {
				n++
			}
		}
	}
	return n
}

const (
	clearPx  = 60   // face width in pixels, at least, to be made out
	clearDet = 0.70 // the face detector's confidence, at least
)

// chainMaybe is the named person most of faces fs look most like, and
// their average likeness to them; 0 if none clearly. Call with s.mu held.
func (s *Store) chainMaybe(fs []*faceRec, refs map[int64][][]float32) (int64, string, float32) {
	if len(fs) == 0 || len(refs) == 0 {
		return 0, "", 0
	}
	votes := map[int64]int{}
	sums := map[int64]float32{}
	for _, f := range fs {
		var best int64
		top := float32(-1)
		u := unitVec(f.emb)
		for pid, rs := range refs {
			if p := s.people[pid]; p == nil {
				continue
			}
			b, _ := bestRef(u, rs)
			sums[pid] += b
			if b > top || b == top && pid < best {
				best, top = pid, b
			}
		}
		votes[best]++
	}
	var who int64
	for pid, n := range votes {
		if n > votes[who] || n == votes[who] && pid < who {
			who = pid
		}
	}
	like := sums[who] / float32(len(fs))
	if who == 0 || float32(votes[who]) < maybeShare*float32(len(fs)) || like < maybeLike {
		return 0, "", 0
	}
	return who, s.people[who].Name, like
}

// chainCompany is who a chain's groups are photographed with: the named
// people on the most days with them (chainWith of them), the days with
// anyone named, and the days with the one most with them. Call with s.mu
// held.
func (s *Store) chainCompany(groups []*cluster) ([]string, int, int) {
	mine := map[int64]bool{}
	for _, c := range groups {
		mine[c.id] = true
	}
	days := map[int64]map[int64]bool{} // person -> days together
	any := map[int64]bool{}
	for _, c := range groups {
		for img := range c.imgs {
			im := s.images[img]
			d := takenDay(im)
			if d < 0 {
				continue
			}
			for pid, f := range s.whoIn(img, 0) {
				if mine[f.ClusterID] || s.people[pid] == nil || s.people[pid].Picture {
					continue
				}
				if days[pid] == nil {
					days[pid] = map[int64]bool{}
				}
				days[pid][d] = true
				any[d] = true
			}
		}
	}
	ids := make([]int64, 0, len(days))
	for pid := range days {
		ids = append(ids, pid)
	}
	slices.SortFunc(ids, func(a, b int64) int { return cmp.Or(cmp.Compare(len(days[b]), len(days[a])), cmp.Compare(a, b)) })
	var names []string
	for _, pid := range ids[:min(len(ids), chainWith)] {
		names = append(names, s.people[pid].Name)
	}
	top := 0
	if len(ids) > 0 {
		top = len(days[ids[0]])
	}
	return names, len(any), top
}

// discoverChains is every person Discover could ask about, most days
// first. Call with s.mu held.
func (s *Store) discoverChains() []chainSet {
	var cs []*cluster
	for _, c := range s.clusters {
		if c.PersonID == 0 && !c.Hidden && c.n >= chainMinSize && c.fg > 0 {
			cs = append(cs, c)
		}
	}
	// Biggest groups first; each joins the person (so far) whose average
	// face it is most like, enough, with a fitting birth year, or starts
	// one. Against the whole person rather than any one group, so no chain
	// of in-between groups can drift from one person to the next.
	slices.SortFunc(cs, func(a, b *cluster) int { return cmp.Or(cmp.Compare(b.n, a.n), cmp.Compare(a.id, b.id)) })
	type person struct {
		sum, unit    []float32
		birth, w, ag float64
		groups       []*cluster
	}
	// The people named already come first: a group as alike to one of
	// their confirmed groups (the one nearest in age, most likely) as to a
	// person's average face, their age fitting and not in their photos, is
	// likely them at another time of life.
	type named = alsoRef
	var known []named
	for _, pi := range s.confirmedIndex() {
		if pi.p.Picture || len(pi.groups) == 0 {
			continue
		}
		n := named{pi: pi}
		for _, g := range pi.groups {
			n.us = append(n.us, clusterUnit(g))
		}
		known = append(known, n)
	}
	seeded := make([][]*cluster, len(known))
	var people []*person
	for _, c := range cs {
		u := clusterUnit(c)
		cb, ca, cok := c.birth()
		kb, kSim := -1, float32(chainAlso)
		ks := make([]float32, len(known))
		parallel(len(known), func(k int) { ks[k], _ = bestRef(u, known[k].us) })
		for k, n := range known {
			if ks[k] < kSim || slices.Contains(c.Rejected, n.pi.p.ID) || !n.pi.ages.fits(cb, ca, cok) || n.shares(c) {
				continue
			}
			kb, kSim = k, ks[k]
		}
		if kb >= 0 {
			seeded[kb] = append(seeded[kb], c)
			continue
		}
		best, bestSim := -1, float32(chainAlike)
		sims := make([]float32, len(people))
		parallel(len(people), func(k int) { sims[k] = dot(u, people[k].unit) })
		for k, p := range people {
			if sims[k] < bestSim {
				continue
			}
			if p.w > 0 && cok && !birthCompatible(float32(p.birth/p.w), float32(p.ag/p.w), true, cb, ca, true) {
				continue
			}
			best, bestSim = k, sims[k]
		}
		if best < 0 {
			people = append(people, &person{sum: make([]float32, len(u))})
			best = len(people) - 1
		}
		p := people[best]
		for k, x := range c.sum {
			p.sum[k] += x
		}
		if cok {
			p.birth += float64(cb) * float64(c.n)
			p.ag += float64(ca) * float64(c.n)
			p.w += float64(c.n)
		}
		p.unit = unitVec(p.sum)
		p.groups = append(p.groups, c)
	}
	chains := map[int][]*cluster{}
	for k, p := range people {
		chains[k] = p.groups
	}
	var all []chainSet
	add := func(g []*cluster, who *personRec) {
		sc := chainSet{groups: g, days: map[int64]bool{}, years: map[int]bool{}, who: who}
		for _, c := range g {
			sc.n += c.n
			for d := range c.days {
				sc.days[d] = true
				sc.years[time.Unix(d*86400, 0).UTC().Year()] = true
			}
		}
		all = append(all, sc)
	}
	for _, g := range chains {
		add(g, nil)
	}
	// One question per group: a yes for some faces shown mustn't file
	// groups not shown.
	for k, gs := range seeded {
		for _, g := range gs {
			add([]*cluster{g}, known[k].pi.p)
		}
	}
	slices.SortFunc(all, func(a, b chainSet) int {
		return cmp.Or(cmp.Compare(len(b.days), len(a.days)), cmp.Compare(len(b.years), len(a.years)), cmp.Compare(b.n, a.n))
	})
	return all
}

// stillness is how alike a chain's faces stay across years: the mean
// likeness of faces five or more years apart (a person changes; a picture
// on the wall is the same picture), and their mean apparent age.
func (s *Store) stillness(gs []*cluster, faces map[int64][]*faceRec) (alike, age float32, still bool) {
	type pt struct {
		t float64
		u []float32
	}
	var ps []pt
	var ages float64
	for _, c := range gs {
		for _, f := range faces[c.id] {
			im := s.images[f.ImageID]
			if f.Age < 0 || im == nil || im.TakenAt == nil || scanned(im) || len(f.emb) == 0 || f.bg {
				continue
			}
			ps = append(ps, pt{float64(*im.TakenAt) / (365.25 * 86400), unitVec(f.emb)})
			ages += float64(f.Age)
		}
	}
	if len(ps) < 10 {
		return 0, 0, false
	}
	age = float32(ages / float64(len(ps)))
	var sum float64
	var n int
	for i := range ps {
		for j := i + 1; j < len(ps) && n < 4000; j++ {
			if math.Abs(ps[i].t-ps[j].t) >= stillYears {
				sum += float64(dot(ps[i].u, ps[j].u))
				n++
			}
		}
	}
	if n < 5 {
		return 0, age, false
	}
	alike = float32(sum / float64(n))
	return alike, age, alike >= stillAlike
}
