package store

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"sync"
	"time"
)

// How surely a face is someone, worked out the same way wherever it's
// used: filing faces by themselves, the batches Discover offers, "Is this
// really them?", and who a face in a photo may be. The percentage shown is
// Score; Sure says whether it's filed by itself, and Hold why not.

// Match is how surely a face is a person.
type Match struct {
	Person int64
	Like   float32 // likeness to their faces: their closest group (a face's own group without it)
	Kin    float32 // a relative of theirs in the photo: kinPhoto, beside it kinNext
	Event  float32 // them in another photo of the occasion: eventBonus
	Shot   float32 // the face at its place in a near-identical shot: them (shotBonus), or someone else (−shotBonus)
	Cost   float32 // what age and gender say against
	Score  float32 // Like + Kin + Event + Shot − Cost, 0–1: the percentage shown
	Next   int64   // the next likeliest person, 0 if none
	Lead   float32 // Score over theirs
	Sure   bool    // near certain: filed by itself
	Hold   string  // why it isn't: "looks 45; they looked ~36 then"
}

// matcher compares faces with everyone's groups. Call its methods with
// s.mu held, as when it was made.
type matcher struct {
	s    *Store
	idx  personIndex
	fam  map[int64]map[int64]string
	refs []fileRef
	// rivals of each person: unnamed groups that are in photos with them,
	// so someone else not named yet: a friend seen over the years, or
	// someone seen fifty times at one event (a knight at a tournament). A face is
	// only sure to be the person if it's clearly more like them than
	// like these: else, before that someone is named, their faces would
	// go to whoever named looks most like them.
	rivals map[int64][]fileRef
	// events: each photo's occasion (photoEvents), and who is in the
	// occasion's photos: someone in the photos just before or after is
	// likely in this one too.
	events   map[int64]int64
	evPeople map[int64]map[int64]int
	// cooc: the groups each group shares rivalShared photos or more
	// with: different people, two faces in one photo being two people.
	cooc map[int64][]*cluster
	// mine: each person's groups, as compared with other groups.
	mine map[int64][][]float32
	// units: the unit centroid of each group a check compares (made
	// once here, not for every face).
	units map[int64][]float32
	// shots: each face's partners in near-identical shots (shots.go).
	shots map[int64][]*faceRec
	elses sync.Map // [group, person] -> someoneElse's answer, as worked out
}

const (
	occasionGap = time.Hour // photos this close, from one camera, are one occasion (a party over hours: a chain of them)
	eventBonus  = 0.05      // the person in another photo of the occasion
)

const (
	rivalShared = 2    // photos two groups must share to be two people,
	rivalShare  = 0.10 // and this share of the smaller one's photos
	rivalFloor  = 0.30 // likeness to the person a rival must have, to be compared with faces
)

// fileRef is one of a person's groups, as compared with faces.
type fileRef struct {
	pid int64
	g   *cluster
	u   []float32
}

// newMatcher indexes everyone's groups. Call with s.mu held.
func (s *Store) newMatcher() *matcher {
	m := &matcher{s: s, idx: s.personIndex(), fam: s.family(), shots: s.shotPartners()}
	for pid, pi := range m.idx {
		for _, g := range pi.groups {
			m.refs = append(m.refs, fileRef{pid, g, clusterUnit(g)})
		}
	}
	m.events, m.evPeople = s.photoEvents(occasionGap), map[int64]map[int64]int{}
	for pid, pi := range m.idx {
		for img := range pi.imgs {
			ev := m.events[img]
			if m.evPeople[ev] == nil {
				m.evPeople[ev] = map[int64]int{}
			}
			m.evPeople[ev][pid]++
		}
	}
	m.mine = map[int64][][]float32{}
	for _, r := range m.refs {
		m.mine[r.pid] = append(m.mine[r.pid], r.u)
	}
	shared := map[[2]int64]int{}
	for _, ids := range s.facesByImage {
		var gs []int64
		for _, id := range ids {
			if f := s.faces[id]; f != nil && f.ClusterID != 0 && !f.NotFace && !f.Picture {
				gs = append(gs, f.ClusterID)
			}
		}
		for i, a := range gs {
			for _, b := range gs[i+1:] {
				if a != b {
					shared[[2]int64{min(a, b), max(a, b)}]++
				}
			}
		}
	}
	m.cooc = map[int64][]*cluster{}
	m.units = map[int64][]float32{}
	for k, n := range shared {
		if n >= rivalShared {
			a, b := s.clusters[k[0]], s.clusters[k[1]]
			if a != nil && b != nil && together(n, len(a.imgs), len(b.imgs)) {
				m.cooc[a.id] = append(m.cooc[a.id], b)
				m.cooc[b.id] = append(m.cooc[b.id], a)
				for _, c := range []*cluster{a, b} {
					if m.units[c.id] == nil {
						m.units[c.id] = clusterUnit(c)
					}
				}
			}
		}
	}
	m.rivals = map[int64][]fileRef{}
	for _, c := range s.clusters {
		if c.PersonID != 0 || c.Hidden || c.n < chainMinSize {
			continue
		}
		var u []float32
		for pid, pi := range m.idx {
			shared := 0
			for img := range c.imgs {
				if pi.imgs[img] {
					shared++
				}
			}
			if shared < rivalShared || !together(shared, len(c.imgs), len(pi.imgs)) {
				continue
			}
			if u == nil {
				u = clusterUnit(c)
			}
			// Only someone at all like them can be the closer match to a
			// face that is like them; a group very like them as a whole is
			// them, the odd stray face aside (a person's faces split over
			// groups), not someone else.
			if b, _ := bestRef(u, m.mine[pid]); b >= rivalFloor && b < rivalSelf {
				m.rivals[pid] = append(m.rivals[pid], fileRef{0, c, u})
			}
		}
	}
	return m
}

// rival is how alike face f is to the person's rivals: the likeliest one
// (its own group without it).
func (m *matcher) rival(f *faceRec, pid int64) float32 {
	best := float32(-1)
	for _, r := range m.rivals[pid] {
		if r.g.id == f.ClusterID {
			if r.g.n >= 2 {
				best = max(best, looLike(f, r.g))
			}
			continue
		}
		best = max(best, f.like(r.u))
	}
	return best
}

// looLike is face f's likeness to its own group without it.
func looLike(f *faceRec, c *cluster) float32 {
	rest := make([]float32, len(c.sum))
	for k := range rest {
		rest[k] = c.sum[k] - f.emb[k]
	}
	return f.like(unitVec(rest))
}

// Variables, for TestEvalStartOver to try others (off: 2, never). From no
// names, a few named a round: treating rivals this like the person as
// them filed 3% more faces, fewer wrong; filing faces of unread age from
// 0.65 filed 4% more, three times as many wrong (so it's off).
var (
	rivalSelf     float32 = 0.70 // a rival group this like the person as a whole is them
	ageUnknownSim float32 = 2    // a face whose age can't be read is filed from this score
)

// elseStrength is how strongly a group filed under someone else must be
// known to be them, in the photos it shares with a face's group, for
// someoneElse to pass it by (a variable for evals). Refiling the library
// from the user's confirmations alone (2026-10-10), against the old names:
// passing none by filed 32847 agreeing, 156 not; only those confirmed
// there (StrengthNamed) 32950 and 168; any (StrengthFiled) 33251 and 163.
// Faces filed by themselves are mostly right, and rechecked every pass.
var elseStrength = StrengthFiled

// together reports whether two sets of photos (na and nb of them) that
// share n are of two people photographed together: in a real part of the
// smaller one's photos, not a few strays (a group of someone holds the odd
// face of whoever stood beside them).
func together(n, na, nb int) bool {
	return float32(n) >= rivalShare*float32(min(na, nb))
}

// groupLike is how alike group g is to person pid: 1 if it's theirs.
func (m *matcher) groupLike(g *cluster, pid int64) float32 {
	if g.PersonID == pid {
		return 1
	}
	u := m.units[g.id]
	if u == nil {
		u = clusterUnit(g)
	}
	b, _ := bestRef(u, m.mine[pid])
	return b
}

// someoneElse reports whether face f's unnamed group is someone other
// than person pid: in photos with a group at least as like them (or
// theirs), so that one is them, if anyone. A group filed under someone
// else doesn't count, as surely as elseStrength asks: a look-alike
// brother there is him, not them.
func (m *matcher) someoneElse(f *faceRec, pid int64) bool {
	c := m.s.clusters[f.ClusterID]
	if c == nil || c.PersonID != 0 || len(m.mine[pid]) == 0 || len(m.cooc[c.id]) == 0 {
		return false
	}
	// The same for every face of the group: worked out once.
	k := [2]int64{c.id, pid}
	if v, ok := m.elses.Load(k); ok {
		return v.(bool)
	}
	out := false
	mine := m.groupLike(c, pid)
	for _, h := range m.cooc[c.id] {
		if h.PersonID != 0 && h.PersonID != pid && m.s.sharedStrength(c, h) >= elseStrength {
			continue // someone else there, as the user said: whoever they look like, they aren't pid
		}
		if m.groupLike(h, pid) >= mine {
			out = true
			break
		}
	}
	m.elses.Store(k, out)
	return out
}

// likes is each person's likeness to face f: their closest group, f's own
// group without f (so a filed face doesn't vouch for itself).
func (m *matcher) likes(f *faceRec) map[int64]float32 {
	c := m.s.clusters[f.ClusterID]
	out := map[int64]float32{}
	for _, r := range m.refs {
		sim := float32(-1)
		if r.g == c {
			if c.n > 1 {
				sim = looLike(f, c)
			}
		} else {
			sim = f.like(r.u)
		}
		if b, ok := out[r.pid]; !ok || sim > b {
			out[r.pid] = sim
		}
	}
	return out
}

// ruledOut reports whether face f can't be person pid: the user said so,
// or they're someone else in the photo already.
func (m *matcher) ruledOut(f *faceRec, pid int64, here map[int64]*faceRec) bool {
	c := m.s.clusters[f.ClusterID]
	return slices.Contains(f.Not, pid) || c != nil && slices.Contains(c.Rejected, pid) ||
		here[pid] != nil && (c == nil || c.PersonID != pid)
}

// Age and gender lower the score rather than rule a face out: an age
// within the person's leeway costs nothing, beyond it the more the further
// off (children's ages tell look-alike siblings apart, so theirs costs
// more; adults' ages read rough); a face reading as the other gender costs
// genderCost.
const (
	ageCostAdult = 0.30 // per leeway beyond the leeway, for an adult
	ageCostChild = 2.50 // for a child (either the face or the person then)
	ageCostMax   = 0.60
	ageChild     = 18
	genderCost   = 0.10
)

// cost is how much face f's age and gender count against person pid, and
// in words.
func (m *matcher) cost(f *faceRec, pid int64) (float32, string) {
	pi := m.idx[pid]
	if pi == nil {
		return 0, ""
	}
	var cost float32
	var why string
	if by, ok := m.s.impliedBirth(f); ok && f.Age >= 0 && pi.ages.ok {
		var off, tol, young float32
		looked, known := pi.ages.looksAt(by + f.Age)
		if known {
			off, tol, young = abs32(f.Age-looked), birthTolerance(max(f.Age, looked)), min(f.Age, looked)
		} else {
			off, tol, young = abs32(by-pi.ages.birth), birthTolerance(max(f.Age, pi.ages.age)), min(f.Age, pi.ages.age)
		}
		if excess := off/tol - 1; excess > 0 {
			rate := float32(ageCostAdult)
			if young < ageChild {
				rate = ageCostChild
			}
			cost = min(ageCostMax, rate*excess)
			why = fmt.Sprintf("looks %d", int(math.Round(float64(f.Age))))
			if known {
				why += fmt.Sprintf("; %s looked ~%d then", pi.p.Name, int(math.Round(float64(looked))))
			} else {
				why += fmt.Sprintf("; %s born ~%d", pi.p.Name, int(math.Round(float64(pi.ages.birth))))
			}
		}
	}
	if genderMismatch(pi.gender, f) {
		cost += genderCost
		why = strings.TrimPrefix(why+"; "+offWord(pi.gender), "; ")
	}
	return cost, why
}

// score is how surely face f is person pid, likeness given: likeness, a
// relative of theirs in the photo, them in another photo of the occasion,
// less what age and gender say against
// (unclamped, for comparing; Score shows it clamped).
func (m *matcher) score(f *faceRec, pid int64, like float32, here map[int64]*faceRec) float32 {
	c, _ := m.cost(f, pid)
	return like + kinWith(f, here, m.fam[pid]) + m.event(f, pid) + m.shot(f, pid) - c
}

// event is eventBonus if person pid is in another photo of the occasion
// face f's photo is from.
func (m *matcher) event(f *faceRec, pid int64) float32 {
	n := m.evPeople[m.events[f.ImageID]][pid]
	if pi := m.idx[pid]; pi != nil && pi.imgs[f.ImageID] {
		n-- // this photo itself
	}
	if n > 0 {
		return eventBonus
	}
	return 0
}

// best is the likeliest person for face f, by score, 0 if none.
func (m *matcher) best(f *faceRec, likes map[int64]float32) int64 {
	here := m.s.whoIn(f.ImageID, f.ID)
	var who int64
	top := float32(-1)
	for pid, like := range likes {
		if m.ruledOut(f, pid, here) {
			continue
		}
		if sc := m.score(f, pid, like, here); sc > top || sc == top && pid < who {
			who, top = pid, sc
		}
	}
	return who
}

// match is how surely face f is person pid, given everyone's likeness to
// it (likes). A face filed under them already is kept on a slightly lower
// bar than a new one is filed on, so it doesn't come and go.
func (m *matcher) match(f *faceRec, pid int64, likes map[int64]float32) Match {
	s := m.s
	here := s.whoIn(f.ImageID, f.ID)
	mt := Match{Person: pid, Like: max(likes[pid], 0), Kin: kinWith(f, here, m.fam[pid]), Event: m.event(f, pid), Shot: m.shot(f, pid)}
	cost, costWhy := m.cost(f, pid)
	mt.Cost = cost
	sc := mt.Like + mt.Kin + mt.Event + mt.Shot - cost
	mt.Score = min(1, max(0, sc))
	next := float32(0)
	for q, like := range likes {
		if q == pid || m.ruledOut(f, q, here) {
			continue
		}
		if o := m.score(f, q, like, here); o > next || o == next && q < mt.Next {
			mt.Next, next = q, o
		}
	}
	mt.Lead = sc - next
	c := s.clusters[f.ClusterID]
	filed := c != nil && c.PersonID == pid
	need, lead, rivalLead := fileSim, float32(fileLead), float32(fileLead)
	if filed {
		// Kept on a lower bar than filed on, so faces don't come and go:
		// against someone not named yet, just still ahead (a group mixing
		// a few of their faces with someone else's would otherwise unfile
		// what the next pass files again).
		need, lead, rivalLead = fileKeep, fileKeepLead, 0
	}
	// Ahead of anyone seen with them not named yet: shown as the lead
	// when it's what holds the face back.
	rival := m.rival(f, pid)
	behindRival := sc-rival < rivalLead
	if behindRival || sc-rival < mt.Lead && mt.Lead >= lead {
		mt.Lead, mt.Next, next = sc-rival, 0, rival
	}
	_, aged := s.impliedBirth(f)
	switch {
	case m.idx[pid] == nil:
		mt.Hold = "no faces of theirs yet"
	case slices.Contains(f.Not, pid):
		mt.Hold = "you said it isn’t them"
	case m.ruledOut(f, pid, here):
		mt.Hold = "they’re in this photo already"
	case m.someoneElse(f, pid):
		mt.Hold = "it’s with a face more like them in other photos: someone else"
	case sc < need && costWhy != "" && sc+cost >= need:
		mt.Hold = costWhy
	case sc < need:
		mt.Hold = "not alike enough to be sure"
	case behindRival:
		mt.Hold = fmt.Sprintf("as like someone not named yet, seen with them (%d%%)", pct(next))
	case mt.Next != 0 && mt.Lead < lead:
		mt.Hold = fmt.Sprintf("almost as like %s (%d%%)", s.people[mt.Next].Name, pct(next))
	case !filed && s.facePx(f) < filePx:
		mt.Hold = "too small a face to be sure"
	case (!aged || f.Age < 0) && sc < ageUnknownSim:
		mt.Hold = "its age can’t be read"
	default:
		mt.Sure = true
	}
	return mt
}

// pct is a likeness as a whole percentage, at most 100.
func pct(x float32) int { return int(math.Round(float64(min(1, max(0, x)) * 100))) }
