package store

import (
	"cmp"
	"context"
	"math"
	"slices"
	"strconv"
	"time"
)

// Discover is one queue of questions, the ones teaching the most first:
// what answering is worth is about how many faces it names, by itself or
// by what is then filed automatically (TestEvalStartOver).
const (
	queueBatch        = 32   // faces in a batch of "More of …?": the best matches, quick to decide
	queueBatchLook    = 100  // faces a batch could ask about in time, which "Who is this?" leaves to it
	queueBatchMin     = 3    // a batch worth fewer faces than this isn't asked
	queueLikely       = 0.30 // a face's likeness to the person from which it counts towards a batch's worth,
	queueSure         = 0.60 // fully from here
	queueKin          = 0.50 // worth of a relation, per photo of the two together (relatives help name each other),
	queueSureAt       = 2    // times its confidence over this (RelGuess.Confidence: about 0.5 doubtful to 4 sure),
	queueSureMin      = 0.25 // within these
	queueSureMax      = 2
	queueDoubt        = 10  // worth of a doubtful face: a wrong face teaches more wrong ones
	queueDatePhoto    = 0.5 // worth of dating a photo, per face in it (their age then reads right)
	queueSessionPhoto = 1   // worth of a day of copied prints, per photo it dates (their faces can then be named)
	queuePicture      = 3   // worth of a picture on the wall, per photo it's in (its age misleads, and it's filed as them)
	queueBirthFace    = 0.1 // worth of a birth year, per face of theirs (each one's age is judged by it)
	queueBirthAdult   = 0.3 // and a third of that, known only by grown-up faces (their age counts for little)
	queueWithDay      = 20  // worth of someone not named yet, per day they're photographed with the same person named
	queueFew          = 30  // confirmed faces under which someone's batch is asked first
	queueFewMax       = 10  // and worth at most this many times more
	queueFewGain      = 3   // how steeply: 3× at a third of queueFew, 1× at queueFew
	queueShot         = 20  // "Same person?": a face confirmed wrong misleads everything learnt from it
	queueWho          = 200 // "Who is this?" chains looked at, the most days first
	queueClearMin     = 5   // clear faces someone not named yet needs, to be asked about
)

// QueueItem is one Discover question: exactly one of the kinds is set.
type QueueItem struct {
	Key      string       `json:"key"`
	Kind     string       `json:"kind"`  // who, more, picture, birth, relation, shot, face, session, date
	Value    float32      `json:"value"` // about how many faces answering names
	Who      *Chain       `json:"who,omitempty"`
	More     *Batch       `json:"more,omitempty"`
	Picture  *Picture     `json:"picture,omitempty"`
	Birth    *Birth       `json:"birth,omitempty"`
	Relation *RelGuess    `json:"relation,omitempty"`
	Face     *FaceDoubt   `json:"face,omitempty"`
	Session  *DateSession `json:"session,omitempty"`
	Date     *DateDoubt   `json:"date,omitempty"`
	Shot     *ShotPair    `json:"shot,omitempty"`
}

// Batch is "More of …?": a known person's likeliest unnamed faces, best
// first, for the user to tick off the wrong ones and accept the rest.
type Batch struct {
	Person int64      `json:"person"`
	Name   string     `json:"name"`
	Cover  string     `json:"cover"`
	Faces  []FaceView `json:"faces"`
	// Confirmed: their faces confirmed so far; few, and the batch is
	// asked first, to know them by.
	Confirmed int `json:"confirmed"`
}

// Picture is "A picture of … on the wall?": a face of theirs that keeps
// one age over the years.
type Picture struct {
	Person int64     `json:"person"`
	Name   string    `json:"name"`
	Set    FrozenSet `json:"set"`
}

// Birth is "When was … born?": what their confirmed faces say, and a few
// of them to go by.
type Birth struct {
	Person int64  `json:"person"`
	Name   string `json:"name"`
	Est    int    `json:"est"` // the year their faces say
	// Rough: said by grown-up faces only, whose ages read loosely (and
	// young): a guess, not a year to pick.
	Rough bool       `json:"rough,omitempty"`
	Faces []FaceView `json:"faces"`
}

// DiscoverQueue lists the questions Discover asks, the most worth first.
func (s *Store) DiscoverQueue(ctx context.Context) []QueueItem {
	var out []QueueItem
	// More of them.
	s.mu.RLock()
	covers := s.personCovers(s.facesByGroup())
	var known []*personRec
	for _, p := range s.people {
		known = append(known, p)
	}
	m := s.newMatcher() // once for everyone: answers made meanwhile only change the order
	confirmed := map[int64]int{}
	for _, f := range s.faces {
		if f.Tagged != 0 && !f.Picture {
			confirmed[f.Tagged]++
		}
	}
	s.mu.RUnlock()
	inBatch := map[int64]bool{} // faces some batch asks about
	for _, p := range known {
		if ctx.Err() != nil {
			return nil
		}
		if m.idx[p.ID] == nil || len(m.idx[p.ID].groups) == 0 {
			continue // no faces of theirs yet
		}
		if b := s.birthQuestion(p.ID); b != nil {
			n := 0
			for _, g := range m.idx[p.ID].groups {
				n += g.n
			}
			v := queueBirthFace * float32(n)
			if b.Rough {
				v *= queueBirthAdult // a grown-up's age counts for little against a face
			}
			out = append(out, QueueItem{Key: "birth-" + strconv.FormatInt(p.ID, 10), Kind: "birth", Value: v, Birth: b})
		}
		for _, set := range s.FrozenFor(ctx, p.ID) {
			out = append(out, QueueItem{Key: "picture-" + strconv.FormatInt(set.Faces[0].ID, 10), Kind: "picture",
				Value: queuePicture * float32(set.Photos), Picture: &Picture{Person: p.ID, Name: p.Name, Set: set}})
		}
		s.mu.RLock()
		fs := s.faceCandidates(m, p.ID, queueBatchLook)
		s.mu.RUnlock()
		look := fs
		fs = fs[:min(len(fs), queueBatch)]
		var worth float32
		for _, f := range fs {
			worth += min(1, max(0, (f.Score-queueLikely)/(queueSure-queueLikely)))
		}
		if worth < queueBatchMin {
			continue
		}
		for _, f := range look {
			inBatch[f.ID] = true
		}
		slices.SortStableFunc(fs, func(a, b FaceView) int { return cmp.Compare(b.Score, a.Score) })
		// Someone known by a few faces gains the most from more: worth
		// more the fewer they are (each confirmed face then teaches more
		// what they look like, and the filing follows).
		n := confirmed[p.ID]
		worth *= min(queueFewMax, max(1, queueFewGain*float32(queueFew)/float32(max(n, 1))-(queueFewGain-1)))
		out = append(out, QueueItem{Key: "more-" + strconv.FormatInt(p.ID, 10), Kind: "more", Value: worth,
			More: &Batch{Person: p.ID, Name: p.Name, Cover: covers[p.ID], Faces: fs, Confirmed: n}})
	}
	// Who is this? People not named yet, seen on several days. A group
	// like someone named is asked about too, unless their batch above asks
	// about most of its faces: else it would be asked about nowhere (a
	// relative looking like them, held as someone else, never filed).
	elses := map[int64]bool{} // groups like someone named, but in photos with them
	keep := func(sc chainSet, faces map[int64][]*faceRec) bool {
		if len(sc.days) < chainWorth || s.chainClear(sc.groups, faces) < queueClearMin {
			return false // a few days only, or no face to make out
		}
		if sc.who == nil {
			return true
		}
		n, in := 0, 0
		for _, c := range sc.groups {
			for _, f := range faces[c.id] {
				n++
				if inBatch[f.ID] {
					in++
				}
			}
		}
		if 2*in >= n {
			return false
		}
		if fs := faces[sc.groups[0].id]; len(fs) > 0 && m.someoneElse(fs[0], sc.who.ID) {
			elses[sc.groups[0].id] = true
		}
		return true
	}
	for _, ch := range s.discoverPeople(queueWho, keep) {
		if ch.Days < chainWorth || ch.Clear < queueClearMin {
			continue // a few days only, or no face to make out
		}
		ch := ch
		if ch.Person != 0 {
			// Like someone named: offered as the answer, unless they're
			// in photos together.
			if elses[ch.Groups[0]] {
				ch.LooksLike = ch.PersonName
				if ch.Maybe == ch.Person {
					ch.Maybe, ch.MaybeName, ch.MaybeLike = 0, "", 0
				}
			} else if ch.Maybe == 0 {
				ch.Maybe, ch.MaybeName = ch.Person, ch.PersonName
			}
			ch.Person, ch.PersonName = 0, ""
		}
		// Worth their clear faces (grainy smudges teach nothing), and much
		// more for each day beside the same person named: likely family.
		// The days beside them count by how much of the cluster is clear:
		// a cluster of smudges near someone is mostly not one person.
		clear := float32(ch.Clear) / float32(max(ch.Photos, 1))
		out = append(out, QueueItem{Key: "who-" + strconv.FormatInt(ch.Groups[0], 10), Kind: "who",
			Value: float32(ch.Clear) + queueWithDay*float32(ch.TopDays)*float32(math.Sqrt(float64(min(clear, 1)))), Who: &ch})
	}
	for _, g := range s.GuessRelations(ctx, 0) {
		g := g
		// Worth the photos it helps name, by how sure the guess is: a pair
		// together often but at odds with the family entered drops.
		sure := min(queueSureMax, max(queueSureMin, g.Confidence/queueSureAt))
		out = append(out, QueueItem{Key: "rel-" + strconv.FormatInt(g.AID, 10) + "-" + strconv.FormatInt(g.BID, 10), Kind: "relation",
			Value: queueKin * float32(g.Photos) * sure, Relation: &g})
	}
	// Faces confirmed as two people at one place in near-identical shots:
	// one is likely confirmed wrong, and every confirmed face teaches.
	for _, sp := range s.ShotConflicts(ctx) {
		sp := sp
		out = append(out, QueueItem{Key: "shot-" + strconv.FormatInt(sp.A.ID, 10) + "-" + strconv.FormatInt(sp.B.ID, 10), Kind: "shot", Value: queueShot, Shot: &sp})
	}
	for _, d := range s.DoubtfulFaces(ctx, 0) {
		d := d
		out = append(out, QueueItem{Key: "face-" + strconv.FormatInt(d.Face.ID, 10), Kind: "face", Value: queueDoubt, Face: &d})
	}
	// Wrong dates: a session of scanning at once, the photos in it then
	// not asked one by one.
	inSession := map[string]bool{}
	for _, ds := range s.DateSessions(ctx) {
		ds := ds
		inSession[ds.Day+"|"+ds.Camera] = true
		out = append(out, QueueItem{Key: "session-" + ds.Day + "-" + ds.Camera, Kind: "session",
			Value: queueSessionPhoto * float32(ds.Dated+len(ds.Others)), Session: &ds})
	}
	for _, d := range s.DoubtfulDates(ctx, 0) {
		if inSession[s.sessionKey(d.Image)] {
			continue
		}
		d := d
		out = append(out, QueueItem{Key: "date-" + strconv.FormatInt(d.Image, 10), Kind: "date",
			Value: queueDatePhoto * float32(len(d.Faces)), Date: &d})
	}
	// Who's who first: people not named yet, how the named are related, and
	// pictures of them. A baseline of people keeps faces from going to
	// whoever named looks most like them, and relatives help name each
	// other. Then the batches of more faces, and the rest, by worth.
	first := func(it QueueItem) int {
		// Photos dated before someone in them was born: their faces can't
		// be named until the date is put right.
		unborn := it.Kind == "session" || it.Kind == "date" && it.Date.Unborn != ""
		few := it.Kind == "more" && it.More.Confirmed < queueFew
		return boolInt(few || it.Kind == "who" || it.Kind == "shot" || it.Kind == "relation" || it.Kind == "picture" || it.Kind == "birth" || unborn)
	}
	slices.SortStableFunc(out, func(a, b QueueItem) int {
		return cmp.Or(cmp.Compare(first(b), first(a)), cmp.Compare(b.Value, a.Value), cmp.Compare(a.Key, b.Key))
	})
	return out
}

// birthQuestion is "When was pid born?", once enough of their faces are
// confirmed to say, and the year isn't entered nor said to be unknown.
func (s *Store) birthQuestion(pid int64) *Birth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.people[pid]
	if p == nil || p.Picture || p.BirthYear != 0 || p.BirthUnknown {
		return nil
	}
	var fs []*faceRec
	var bs []float32
	for _, f := range s.faces {
		if f.Tagged != pid || f.Picture {
			continue
		}
		if by, ok := s.impliedBirth(f); ok && s.facePx(f) >= dateMinPx {
			fs, bs = append(fs, f), append(bs, by)
		}
	}
	if len(fs) < ageConfirmed {
		return nil
	}
	// A child's age reads to a year or two, a grown-up's loosely: from
	// their childhood faces, when there are some.
	var young []float32
	for i, f := range fs {
		if f.Age < ageChild {
			young = append(young, bs[i])
		}
	}
	sorted, rough := young, false
	if len(young) < birthYoung {
		sorted, rough = slices.Clone(bs), true
	}
	slices.Sort(sorted)
	b := &Birth{Person: pid, Name: p.Name, Est: int(math.Floor(float64(sorted[len(sorted)/2]))), Rough: rough}
	// A few to go by, spread over the years: the youngest ones first
	// (a child's age reads closest).
	slices.SortFunc(fs, func(a, c *faceRec) int { return cmp.Or(cmp.Compare(a.Age, c.Age), cmp.Compare(a.ID, c.ID)) })
	step := max(1, len(fs)/birthShown)
	for i := 0; i < len(fs) && len(b.Faces) < birthShown; i += step {
		b.Faces = append(b.Faces, s.faceView(fs[i]))
	}
	return b
}

const (
	birthShown = 6 // faces a birth question shows
	birthYoung = 3 // childhood faces to say the year by, else it's rough
)

// sessionKey is the day and camera a photo is from, as DateSessions
// keys them.
func (s *Store) sessionKey(img int64) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	im := s.images[img]
	if im == nil || im.TakenAt == nil {
		return ""
	}
	return time.Unix(*im.TakenAt, 0).UTC().Format(time.DateOnly) + "|" + im.CameraMake + " " + im.CameraModel
}
