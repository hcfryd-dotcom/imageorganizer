package store

import (
	"cmp"
	"context"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	guessDays      = 8    // days two people must be photographed together, to ask about them
	guessFewDays   = 4    // or this many, for people seldom photographed:
	guessFewShare  = 0.40 // on that share of the rarer one's days
	guessFewLift   = 10   // and that many times more often than chance
	guessHomeDays  = 12   // or this many with someone on so many days
	guessHomeLift  = 3    // that the lift can be no more than this,
	guessHomeShare = 0.30 // on that share of the rarer one's days
	guessShare     = 0.20 // share of the rarer one's days the other must be on too
	guessLift      = 1.5  // and that many times more often than chance (the household is on everything)
	guessParent    = 17   // years between birth years, at least, for parent and child
	guessGrand     = 42   // years between, at least, for grandparent and grandchild
	guessCouple    = 13   // years between, at most, for partners or siblings
	guessKidAge    = 16   // age of the elder at shared photos below which a pair are siblings, not partners
	guessKidShare  = 0.30 // share of shared days with the elder that young, to say siblings
	guessDuo       = 10   // photos of just the two, side by side often, for those on everything
	guessBeside    = 0.25 // share of their photos side by side, for that
	guessDuoShare  = 0.10 // share of their photos with just the two of them, to say partners
	guessClash     = 0.5  // confidence kept by a guess at odds with the family entered
	guessOverlap   = 0.5  // share of a pair's days within someone's time with a partner known, to say they can't be partners
	guessWrong     = 2    // and gained by one putting right what's entered
	guessOlder     = 8    // years younger, at least, that a parent entered is to be wrong
	guessYounger   = 10   // years older a parent must be, at least, else wrong
	guessApart     = 25   // years apart, at least, for siblings or partners entered to be wrong
)

// RelGuess is a question for the user: how are these two related? Kind is
// the best guess of what B is to A (parent, child, sibling, partner,
// grandparent, grandchild), from how often they're photographed together,
// side by side, and their ages.
type RelGuess struct {
	A, B       int64       `json:"-"`
	AName      string      `json:"a_name"`
	BName      string      `json:"b_name"`
	AID        int64       `json:"a"`
	BID        int64       `json:"b"`
	Kind       string      `json:"kind"`
	Days       int         `json:"days"`       // days photographed together
	Photos     int         `json:"photos"`     // photos together
	Beside     int         `json:"beside"`     // photos side by side
	Duo        int         `json:"duo"`        // photos of just the two of them
	Share      float32     `json:"share"`      // share of the rarer one's days the other is on
	Lift       float32     `json:"lift"`       // how much more often than chance
	ABirth     float32     `json:"a_birth"`    // estimated birth years, 0 unknown
	BBirth     float32     `json:"b_birth"`    //
	FirstYear  int         `json:"first_year"` // first and last year together
	LastYear   int         `json:"last_year"`  //
	FaceA      int64       `json:"-"`
	FaceB      int64       `json:"-"`
	Faces      [2]FaceView `json:"faces"` // a photo of the two of them, side by side if any: A's face, B's
	Image      int64       `json:"image"`
	Confidence float32     `json:"confidence"` // how sure we are they're related at all, for the order
	AGender    string      `json:"a_gender,omitempty"`
	BGender    string      `json:"b_gender,omitempty"`
	// Clash: why the guess doesn't square with the family entered so far
	// ("Kid has two parents already"). Asked anyway, later: the family
	// entered may be wrong, or not all of it.
	Clash string `json:"clash,omitempty"`
	Admit bool   `json:"-"` // asked about (guessRelations(all) lists others too)
	// Entered: what B was entered as to A, when their ages say that's
	// wrong (a father born after his daughter): asked to put it right.
	Entered string `json:"entered,omitempty"`
	// Then: for partners, "before" or "after" ThenOf's partner ThenWith:
	// people have partners one after another.
	Then     string `json:"then,omitempty"`
	ThenOf   string `json:"then_of,omitempty"`
	ThenWith string `json:"then_with,omitempty"`
}

// linked reports whether a and b are family already, however distant:
// joined by parents, siblings and partners (by blood or marriage), whose
// links say how they're related. Friends join no one.
func linked(known map[int64]map[int64]string, a, b int64) bool {
	seen := map[int64]bool{a: true}
	todo := []int64{a}
	for len(todo) > 0 && len(seen) < 4096 {
		x := todo[0]
		todo = todo[1:]
		for q, k := range known[x] {
			if k == KinFriend || seen[q] {
				continue
			}
			if q == b {
				return true
			}
			seen[q] = true
			todo = append(todo, q)
		}
	}
	return false
}

// parentsOf counts the parents known of p.
func parentsOf(known map[int64]map[int64]string, p int64) int {
	n := 0
	for _, k := range known[p] {
		if k == KinParent {
			n++
		}
	}
	return n
}

// meanDay is the middle of a set of days.
func meanDay(days map[int64]bool) float64 {
	var sum float64
	for d := range days {
		sum += float64(d)
	}
	return sum / float64(max(len(days), 1))
}

// GuessRelations lists pairs of named people who are likely related, but
// not known to be (entered, worked out, or answered "not related"), most
// likely first, with a guess of how.
func (s *Store) GuessRelations(ctx context.Context, limit int) []RelGuess {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.guessRelations(limit, false)
}

// guessRelations is GuessRelations; all also lists the pairs already
// known (to measure it). Call with s.mu held.
func (s *Store) guessRelations(limit int, all bool) []RelGuess {
	type pair struct{ a, b int64 }
	type stat struct {
		days           map[int64]bool
		young          int // shared days with the elder under guessKidAge
		photos, beside int
		duo            int
		first, last    int64
		face           [2]int64
		img            int64
		best           float32 // how good the shown photo is
	}
	birth := map[int64]float32{}
	for id, p := range s.people {
		if p.Picture {
			continue
		}
		if b, _, ok := s.personBirth(p); ok {
			birth[id] = b
		}
	}
	days := map[int64]map[int64]bool{} // person -> days photographed
	allDays := map[int64]bool{}
	pairs := map[pair]*stat{}
	for img, ids := range s.facesByImage {
		im := s.images[img]
		d := takenDay(im)
		if d < 0 || scanned(im) {
			continue
		}
		allDays[d] = true
		who := s.whoIn(img, 0)
		named := 0
		for _, fid := range ids {
			if f := s.faces[fid]; f != nil && !f.NotFace && !f.Picture {
				named++
			}
		}
		for p := range who {
			if s.people[p] == nil || s.people[p].Picture {
				continue
			}
			if days[p] == nil {
				days[p] = map[int64]bool{}
			}
			days[p][d] = true
		}
		for a, fa := range who {
			for b, fb := range who {
				if a >= b || s.people[a] == nil || s.people[b] == nil || s.people[a].Picture || s.people[b].Picture {
					continue
				}
				st := pairs[pair{a, b}]
				if st == nil {
					st = &stat{days: map[int64]bool{}, first: math.MaxInt64}
					pairs[pair{a, b}] = st
				}
				if !st.days[d] {
					st.days[d] = true
					if ba, okA := birth[a]; okA {
						if bb, okB := birth[b]; okB {
							year := float32(1970) + float32(d)/365.25
							if year-min(ba, bb) < guessKidAge {
								st.young++
							}
						}
					}
				}
				st.photos++
				next := beside(fa, fb)
				if next {
					st.beside++
				}
				if named == 2 {
					st.duo++
				}
				st.first, st.last = min(st.first, d), max(st.last, d)
				// The photo to show: the two side by side, big.
				q := min(fa.Box[2]-fa.Box[0], fb.Box[2]-fb.Box[0]) * fa.Score * fb.Score
				if next {
					q *= 2
				}
				if q > st.best {
					st.best, st.face, st.img = q, [2]int64{fa.ID, fb.ID}, img
				}
			}
		}
	}
	known, in, est := s.family(), s.entered(), s.genderEstimates()
	var out []RelGuess
	for pr, st := range pairs {
		a, b := pr.a, pr.b
		k, isKnown := known[a][b]
		_, ent := in[a][b]
		// A parent's partner with one parent known may be the other: asked
		// whether they are, though they count as a stepparent so far.
		maybeParent := false
		if isKnown && !ent && !all {
			child := a
			if k == KinStepChild {
				child = b
			}
			if maybeParent = (k == KinStepParent || k == KinStepChild) && parentsOf(known, child) < 2; !maybeParent {
				continue // follows from what's entered: fixed there
			}
		}
		if !isKnown && !all && linked(known, a, b) {
			continue // family already, through others: their links say how
		}
		pa, pb := s.people[a], s.people[b]
		if slices.Contains(pa.Asked, b) || slices.Contains(pb.Asked, a) {
			continue
		}
		n := len(st.days)
		if n < guessFewDays && !(all && evalAllDays) {
			continue
		}
		da, db := len(days[a]), len(days[b])
		share := float32(n) / float32(min(da, db))
		// Chance: if b were on random days of the years a is
		// photographed, how many of a's days would they share?
		lift := float32(n) * float32(len(allDays)) / float32(da) / float32(db)
		// The most it can be: with someone on most days (the household),
		// no one is together with them much more often than chance.
		most := float32(len(allDays)) / float32(max(da, db))
		pairShot := st.duo >= guessDuo && float32(st.beside) >= guessBeside*float32(st.photos)
		ok := n >= guessDays && share >= guessShare && lift >= guessLift || // together more often than chance
			n >= guessDays && pairShot || // the household: photographed as a pair, side by side
			share >= guessFewShare && lift >= guessFewLift || // seldom photographed, but then together
			n >= guessHomeDays && most < guessHomeLift && share >= guessHomeShare // often with someone on everything
		if !ok && !isKnown && !all {
			continue
		}
		g := RelGuess{Admit: ok, A: a, B: b, AID: a, BID: b, AName: pa.Name, BName: pb.Name, Days: n, Photos: st.photos,
			Beside: st.beside, Duo: st.duo, Share: share, Lift: lift, ABirth: birth[a], BBirth: birth[b],
			FirstYear: time.Unix(st.first*86400, 0).UTC().Year(), LastYear: time.Unix(st.last*86400, 0).UTC().Year(),
			FaceA: st.face[0], FaceB: st.face[1], Image: st.img}
		// The elder first: "Is Alice Kid's mother?"
		if g.ABirth != 0 && g.BBirth != 0 && g.BBirth < g.ABirth {
			g.A, g.B, g.AID, g.BID, g.AName, g.BName, g.ABirth, g.BBirth, g.FaceA, g.FaceB =
				b, a, b, a, pb.Name, pa.Name, g.BBirth, g.ABirth, g.FaceB, g.FaceA
		}
		ga, _ := genderOf(s.people[g.A], est[g.A])
		gb, _ := genderOf(s.people[g.B], est[g.B])
		g.AGender, g.BGender = ga, gb
		g.Kind = guessKind(g, float32(st.young)/float32(n))
		if maybeParent && g.Kind != KinChild {
			continue
		}
		if isKnown && ent && !all {
			// Entered already: asked again only when their ages say otherwise.
			if k := in[g.A][g.B]; ageWrong(k, g) {
				g.Entered = k
			} else {
				continue
			}
		}
		g.Clash = s.clash(known, g)
		if g.Kind == KinPartner {
			// One partner at a time: against the partners known of either,
			// when were these two together?
		partners:
			for _, x := range []int64{g.A, g.B} {
				for q, k := range known[x] {
					if k != KinPartner || q == g.A || q == g.B {
						continue
					}
					o := pairs[pair{min(x, q), max(x, q)}]
					if o == nil {
						continue // never photographed together: nothing to go by
					}
					in := 0
					for d := range st.days {
						if d >= o.first && d <= o.last {
							in++
						}
					}
					g.ThenOf, g.ThenWith = s.people[x].Name, s.people[q].Name
					switch {
					case float32(in) > guessOverlap*float32(n):
						g.Clash = fmt.Sprintf("%s was with %s then", g.ThenOf, g.ThenWith)
						g.Then = ""
						break partners
					case meanDay(st.days) < meanDay(o.days):
						g.Then = "before"
					default:
						g.Then = "after"
					}
				}
			}
		}
		if g.Kind == KinGrandchild && !all {
			// Not asked: grandparents follow from the parents entered,
			// which are asked about themselves.
			continue
		}
		g.Faces = [2]FaceView{s.faceView(s.faces[g.FaceA]), s.faceView(s.faces[g.FaceB])}
		// Confidence: together often, beside each other, on their own.
		g.Confidence = share * float32(math.Log1p(float64(n))) * (1 + float32(st.beside)/float32(st.photos)) *
			(1 + float32(st.duo)/float32(st.photos))
		if g.Clash != "" {
			g.Confidence *= guessClash
		}
		if g.Entered != "" {
			g.Confidence *= guessWrong
		}
		out = append(out, g)
	}
	// Someone's latest partner first: one partner guessed at a time, the
	// earlier ones asked about once that's settled.
	latest := map[int64]int{}
	for _, g := range out {
		if g.Kind == KinPartner && g.Then == "" && g.Clash == "" {
			latest[g.A], latest[g.B] = max(latest[g.A], g.LastYear), max(latest[g.B], g.LastYear)
		}
	}
	for i, g := range out {
		if g.Kind == KinPartner && g.Then == "" && g.Clash == "" && (g.LastYear < latest[g.A] || g.LastYear < latest[g.B]) {
			out[i].Confidence *= guessClash
		}
	}
	slices.SortFunc(out, func(x, y RelGuess) int {
		return cmp.Or(cmp.Compare(y.Confidence, x.Confidence), cmp.Compare(x.A, y.A), cmp.Compare(x.B, y.B))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// clash says why a guess doesn't square with the family known, "" when it
// does: no third parent (partners: by when, in guessRelations), siblings with parents of their
// own share one, grandparents come through a parent.
func (s *Store) clash(known map[int64]map[int64]string, g RelGuess) string {
	of := func(p int64, kind string) []int64 {
		var ids []int64
		for q, k := range known[p] {
			if k == kind && q != g.A && q != g.B {
				ids = append(ids, q)
			}
		}
		slices.Sort(ids)
		return ids
	}
	names := func(ids []int64) string {
		var ns []string
		for _, id := range ids {
			if p := s.people[id]; p != nil {
				ns = append(ns, p.Name)
			}
		}
		return strings.Join(ns, " and ")
	}
	switch g.Kind {
	case KinChild:
		if ps := of(g.B, KinParent); len(ps) >= 2 {
			return fmt.Sprintf("%s has two parents already: %s", g.BName, names(ps))
		}
	case KinSibling:
		pa, pb := of(g.A, KinParent), of(g.B, KinParent)
		if len(pa) > 0 && len(pb) > 0 && !slices.ContainsFunc(pa, func(q int64) bool { return slices.Contains(pb, q) }) {
			return fmt.Sprintf("their parents differ: %s, and %s", names(pa), names(pb))
		}
	case KinGrandchild:
		// Through one of B's parents, unless all of theirs are known.
		ps := of(g.B, KinParent)
		if len(ps) == 0 || known[g.A][g.B] == KinGrandchild {
			return ""
		}
		for _, p := range ps {
			if len(of(p, KinParent)) < 2 {
				return ""
			}
		}
		return fmt.Sprintf("%s’s parents %s have both their parents already", g.BName, names(ps))
	}
	return ""
}

// ageWrong reports whether B entered as kind to A (A the elder) can't be,
// by their ages: a parent younger than their child, a child born to a
// ten-year-old, a brother 30 years apart.
func ageWrong(kind string, g RelGuess) bool {
	if g.ABirth == 0 || g.BBirth == 0 {
		return false
	}
	gap := g.BBirth - g.ABirth // B younger by
	switch kind {
	case KinParent, KinGrandparent:
		return gap >= guessOlder
	case KinChild, KinGrandchild:
		return gap < guessYounger
	case KinSibling, KinPartner:
		return gap >= guessApart
	}
	return false
}

// guessKind is the likeliest relation, what B is to A (A the elder when
// both ages are known), from their age gap; young is the share of their
// days together with the elder still a child.
func guessKind(g RelGuess, young float32) string {
	if g.ABirth == 0 || g.BBirth == 0 {
		return KinPartner
	}
	gap := g.BBirth - g.ABirth
	switch {
	case gap >= guessGrand:
		return KinGrandchild
	case gap >= guessParent:
		return KinChild
	case gap <= guessCouple && young >= guessKidShare:
		return KinSibling
	case gap <= guessCouple:
		// Adults of an age: partners are photographed as a pair, and
		// are (mostly) a man and a woman; brothers and sisters less so.
		duo := float32(g.Duo) / float32(max(g.Photos, 1))
		next := float32(g.Beside) / float32(max(g.Photos, 1))
		if (g.AGender != "" && g.AGender == g.BGender) || (duo < guessDuoShare && next < guessBeside) {
			return KinSibling
		}
		return KinPartner
	}
	// In between: an older sibling or a young parent; together as
	// children says siblings.
	if young >= guessKidShare {
		return KinSibling
	}
	return KinChild
}

// SetAsked records that the user answered how a and b are related (not
// family, or a way not kept as a relation), so they aren't asked about
// again; on false forgets it.
func (s *Store) SetAsked(ctx context.Context, a, b int64, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pa, pb := s.people[a], s.people[b]
	if pa == nil || pb == nil {
		return ErrNotFound
	}
	if a == b {
		return ErrBadRelation
	}
	ua := *pa
	ua.Asked = slices.DeleteFunc(slices.Clone(ua.Asked), func(x int64) bool { return x == b })
	if on {
		ua.Asked = append(ua.Asked, b)
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketPeople, ua.ID, &ua) }); err != nil {
		return err
	}
	*pa = ua
	return nil
}

// evalAllDays lets guessRelations(all) list pairs on any number of days (to measure).
var evalAllDays bool
