package store

import (
	"cmp"
	"context"
	"runtime"
	"slices"
	"strings"
	"sync"
)

const (
	matchPeople = 3    // people a group's faces are compared with, by likeness to the group
	matchFloor  = 0.15 // a group's likeness to a person's average to look closer
	// A face turned away or looking down is hard to judge by eye: the
	// matcher ranks by likeness × (matchPose + (1-matchPose) × frontal).
	matchPose = 0.3
	// So is a small one: ranked by likeness × (matchSize + (1-matchSize) ×
	// min(1, width / matchPx)) too.
	matchSize = 0.5
	matchPx   = 100
	// And so is a blurry one, which the face detector is unsure of: × the
	// detector's confidence, from 0 at matchDet0 to 1 at matchDet1.
	matchDet0 = 0.50
	matchDet1 = 0.80
	// And a dark or washed-out one, measured on its crop: × the face's
	// luminance spread from matchSD0 (left out) to matchSD1 (fine).
	matchSD0  = 6
	matchSD1  = 25
	matchLook = 5  // candidates measured on their crops, per face asked for
	likeTop   = 10 // the known faces most alike, of which the clearest is shown
)

// FaceMatch is an unknown face and who it most likely is, for the face
// matcher: one face, one decision.
type FaceMatch struct {
	Face   FaceView   `json:"face"`
	Person int64      `json:"person"`
	Name   string     `json:"name"`
	Score  float32    `json:"score"`
	Like   FaceView   `json:"like"`             // the person's known face it looks most like (in their closest group)
	Others []MatchAlt `json:"others,omitempty"` // the next likeliest people
	Why    string     `json:"why,omitempty"`    // a relative of theirs beside it: "next to Bob (partner)"
}

// MatchAlt is another person an unknown face may be.
type MatchAlt struct {
	Person int64   `json:"person"`
	Name   string  `json:"name"`
	Score  float32 `json:"score"`
	Kin    string  `json:"kin,omitempty"` // the suggested person's sibling ("brother"), offered whatever the age says
}

type matchRef struct {
	pid  int64
	pi   *personInfo
	mean []float32   // average of the groups, to narrow the search
	refs [][]float32 // each confirmed group's centroid
}

// FaceMatches ranks the faces in unnamed groups by how surely they are
// someone already named, best first. A group can hold several people, so
// each face is scored on its own (against the people its group is most
// like), with its own age checked; faces the user said are not that person
// are passed over for the next likeliest. Faces looking at the camera come
// first, and big ones: they are the ones a person can tell apart. Faces
// put off ("skip") come last, the longest put off first.
func (s *Store) FaceMatches(ctx context.Context, limit int, o MatchOptions) []FaceMatch {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.faceMatches(limit, nil, o)
}

// MatchOptions narrow and order the matcher's faces.
type MatchOptions struct {
	// MinAge, MaxAge: the unknown face's apparent age (0: no bound). With
	// a bound, faces whose age isn't known are left out.
	MinAge, MaxAge float32
	// Sort: "" best first; "young" or "old": by the face's apparent age;
	// "new" or "oldphoto": by when the photo was taken.
	Sort string
}

// keep reports whether a face passes the age bounds.
func (o MatchOptions) keep(f *faceRec) bool {
	if o.MinAge <= 0 && o.MaxAge <= 0 {
		return true
	}
	if f.AgeVersion == 0 || f.Age < 0 {
		return false
	}
	return (o.MinAge <= 0 || f.Age >= o.MinAge) && (o.MaxAge <= 0 || f.Age <= o.MaxAge)
}

// order compares two faces for a chosen sort (0: equal, or best first).
func (s *Store) matchOrder(sort string, a, b *faceRec) int {
	age := func(f *faceRec) float32 {
		if f.AgeVersion == 0 || f.Age < 0 {
			return 999 // unknown: last either way
		}
		return f.Age
	}
	taken := func(f *faceRec) int64 {
		if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil {
			return *im.TakenAt
		}
		return 0
	}
	switch sort {
	case "young":
		return cmp.Compare(age(a), age(b))
	case "old":
		ab, bb := age(a), age(b)
		if ab == 999 || bb == 999 {
			return cmp.Compare(ab, bb)
		}
		return cmp.Compare(bb, ab)
	case "new":
		return cmp.Compare(taken(b), taken(a))
	case "oldphoto":
		ta, tb := taken(a), taken(b)
		if ta == 0 || tb == 0 {
			return cmp.Compare(tb, ta) // undated last
		}
		return cmp.Compare(ta, tb)
	}
	return 0
}

// faceMatches is FaceMatches, for the faces in the photos in only (all
// when nil); call with s.mu held.
func (s *Store) faceMatches(limit int, only map[int64]bool, o MatchOptions) []FaceMatch {
	idx := s.personIndex()
	var people []*matchRef
	for pid, pi := range idx {
		m := &matchRef{pid: pid, pi: pi}
		for _, g := range pi.groups {
			if !g.Auto {
				m.refs = append(m.refs, clusterUnit(g))
			}
		}
		if len(m.refs) == 0 {
			continue
		}
		m.mean = make([]float32, len(m.refs[0]))
		for _, r := range m.refs {
			for k, x := range r {
				m.mean[k] += x
			}
		}
		m.mean = unitVec(m.mean)
		people = append(people, m)
	}
	var cs []*cluster
	if only != nil { // just the groups with a face in those photos
		seen := map[int64]bool{}
		for img := range only {
			for _, fid := range s.facesByImage[img] {
				if f := s.faces[fid]; f != nil && !seen[f.ClusterID] {
					seen[f.ClusterID] = true
					if c := s.clusters[f.ClusterID]; c != nil && c.PersonID == 0 && !c.Hidden && c.n > 0 {
						cs = append(cs, c)
					}
				}
			}
		}
	} else {
		for _, c := range s.clusters {
			if c.PersonID == 0 && !c.Hidden && c.n > 0 {
				cs = append(cs, c)
			}
		}
	}
	// Each group's likeliest people: by their average first, then by their
	// closest group.
	cands := make([][]*matchRef, len(cs))
	parallel(len(cs), func(i int) {
		c := cs[i]
		u := clusterUnit(c)
		type ps struct {
			m *matchRef
			s float32
		}
		var top []ps
		for _, m := range people {
			if slices.Contains(c.Rejected, m.pid) {
				continue
			}
			if d := dot(u, m.mean); d >= matchFloor {
				top = append(top, ps{m, d})
			}
		}
		slices.SortFunc(top, func(a, b ps) int { return cmp.Compare(b.s, a.s) })
		for _, t := range top[:min(len(top), matchPeople)] {
			if best, _ := bestRef(u, t.m.refs); best >= faceGroupMin {
				cands[i] = append(cands[i], t.m)
			}
		}
	})
	want := map[int64][]*matchRef{}
	for i, c := range cs {
		if len(cands[i]) > 0 {
			want[c.id] = cands[i]
		}
	}
	// Relatives cling together: a face in a photo with someone named is
	// also compared with that person's family.
	fam := s.family()
	byPid := make(map[int64]*matchRef, len(people))
	for _, m := range people {
		byPid[m.pid] = m
	}
	kinImg := map[int64]bool{}
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 && !c.Auto && len(fam[c.PersonID]) > 0 {
			kinImg[f.ImageID] = true
		}
	}
	var fs []*faceRec
	for _, f := range s.faces {
		if only != nil && !only[f.ImageID] || !o.keep(f) {
			continue
		}
		c := s.clusters[f.ClusterID]
		if c == nil || f.NotFace || f.Picture || f.Tagged != 0 || f.bg || len(f.emb) == 0 {
			continue
		}
		if want[f.ClusterID] != nil || kinImg[f.ImageID] && c.PersonID == 0 && !c.Hidden && c.n > 0 {
			fs = append(fs, f)
		}
	}
	type pick struct {
		m     *matchRef
		score float32 // likeness
		kin   float32 // a relative of theirs in the photo
		why   string
	}
	picks := make([][]pick, len(fs))
	rank := make([]float32, len(fs))
	parallel(len(fs), func(i int) {
		f := fs[i]
		u := unitVec(f.emb)
		by, byOK := s.impliedBirth(f)
		ms := slices.Clone(want[f.ClusterID])
		for q := range s.whoIn(f.ImageID, f.ID) {
			for p := range fam[q] {
				if m := byPid[p]; m != nil && !slices.Contains(ms, m) {
					ms = append(ms, m)
				}
			}
		}
		for _, m := range ms {
			if slices.Contains(f.Not, m.pid) || m.pi.imgs[f.ImageID] {
				continue // said not them, or they are already in the photo
			}
			if c := s.clusters[f.ClusterID]; slices.Contains(c.Rejected, m.pid) {
				continue
			}
			sc, _ := bestRef(u, m.refs)
			if byOK && !m.pi.ages.fitsLike(by, f.Age, true, sc) {
				continue
			}
			kin, who, next := s.kinNear(f, fam[m.pid])
			if sc >= faceCandMin || kin > 0 && sc >= kinSim && byOK { // on a relative's account only with a known age
				pk := pick{m: m, score: sc, kin: kin}
				if kin > 0 {
					pk.why = s.kinWhy(who, next, fam[m.pid])
				}
				if genderMismatch(m.pi.gender, f) {
					pk.kin -= genderMiss // ranked lower, not ruled out
					pk.why = strings.TrimPrefix(pk.why+"; "+offWord(m.pi.gender), "; ")
				}
				picks[i] = append(picks[i], pk)
			}
		}
		slices.SortFunc(picks[i], func(a, b pick) int { return cmp.Compare(b.score+b.kin, a.score+a.kin) })
		if len(picks[i]) > 0 {
			pose := matchPose + (1-matchPose)*s.frontal(f)
			size := matchSize + (1-matchSize)*min(1, s.facePx(f)/matchPx)
			det := min(1, max(0, (f.Score-matchDet0)/(matchDet1-matchDet0)))
			rank[i] = (picks[i][0].score + picks[i][0].kin) * pose * size * det
		}
	})
	order := make([]int, 0, len(fs))
	for i := range fs {
		if len(picks[i]) > 0 {
			order = append(order, i)
		}
	}
	slices.SortFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(fs[a].Skipped, fs[b].Skipped), s.matchOrder(o.Sort, fs[a], fs[b]),
			cmp.Compare(rank[b], rank[a]), cmp.Compare(fs[a].ID, fs[b].ID))
	})
	// The likeliest are measured on their crops (cached), and smudges
	// too dark or faint to judge go.
	order = order[:min(len(order), matchLook*limit)]
	vis := make([]float32, len(fs))
	parallel(len(order), func(k int) {
		vis[order[k]] = s.visible(fs[order[k]])
	})
	order = slices.DeleteFunc(order, func(i int) bool { return vis[i] == 0 })
	slices.SortStableFunc(order, func(a, b int) int {
		return cmp.Or(cmp.Compare(fs[a].Skipped, fs[b].Skipped), s.matchOrder(o.Sort, fs[a], fs[b]),
			cmp.Compare(rank[b]*vis[b], rank[a]*vis[a]))
	})
	order = order[:min(len(order), limit)]
	// The known face shown with each is one the user confirmed themselves
	// (tagged, or seen when naming its group), never just one filed under
	// the person with its group: those can be wrong.
	confirmed := map[int64][]*faceRec{}
	for _, i := range order {
		confirmed[picks[i][0].m.pid] = nil
	}
	for _, f := range s.faces {
		if _, want := confirmed[f.Tagged]; want && f.Tagged != 0 {
			if c := s.clusters[f.ClusterID]; c != nil && c.PersonID == f.Tagged {
				confirmed[f.Tagged] = append(confirmed[f.Tagged], f)
			}
		}
	}
	out := make([]FaceMatch, 0, len(order))
	for _, i := range order {
		p := picks[i][0]
		fm := FaceMatch{Face: s.faceView(fs[i]), Person: p.m.pid, Name: p.m.pi.p.Name, Score: p.score, Why: p.why}
		if like := s.likeFace(unitVec(fs[i].emb), confirmed[p.m.pid]); like != nil {
			fm.Like = s.faceView(like)
		}
		for _, o := range picks[i][1:] {
			fm.Others = append(fm.Others, MatchAlt{Person: o.m.pid, Name: o.m.pi.p.Name, Score: o.score})
		}
		// Brothers and sisters (half ones too) are mixed up most: offer them too, even
		// where the age estimate rules them out (it can read a child years
		// off).
		f := fs[i]
		for q, kind := range fam[p.m.pid] {
			m := byPid[q]
			if kind != KinSibling && kind != KinHalfSibling || m == nil || slices.ContainsFunc(fm.Others, func(o MatchAlt) bool { return o.Person == q }) ||
				slices.Contains(f.Not, q) || m.pi.imgs[f.ImageID] {
				continue
			}
			sc, _ := bestRef(unitVec(f.emb), m.refs)
			fm.Others = append(fm.Others, MatchAlt{Person: q, Name: m.pi.p.Name, Score: sc, Kin: kinWord(kind, m.pi.gender)})
		}
		out = append(out, fm)
	}
	return out
}

// likeFace picks, of a person's confirmed faces, the one to show next to
// unknown face u: of the ones most like it, the clearest.
func (s *Store) likeFace(u []float32, group []*faceRec) *faceRec {
	type cand struct {
		f *faceRec
		d float32
	}
	var cs []cand
	for _, o := range group {
		if len(o.emb) > 0 && !o.NotFace && !o.Picture && !s.twiceInPhoto(o) {
			cs = append(cs, cand{o, dot(u, unitVec(o.emb))})
		}
	}
	slices.SortFunc(cs, func(a, b cand) int { return cmp.Compare(b.d, a.d) })
	var best *faceRec
	bestScore := float32(-1)
	for _, c := range cs[:min(len(cs), likeTop)] {
		// Likeness first; a clear face wins over a slightly more alike smudge.
		if sc := c.d * (0.5 + 0.5*s.visible(c.f)); sc > bestScore {
			best, bestScore = c.f, sc
		}
	}
	return best
}

// twiceInPhoto reports whether a face's person (by its group) is also
// another face in the same photo: one of the two is filed wrongly, so it
// can't stand for them.
func (s *Store) twiceInPhoto(f *faceRec) bool {
	c := s.clusters[f.ClusterID]
	if c == nil || c.PersonID == 0 {
		return false
	}
	for _, fid := range s.facesByImage[f.ImageID] {
		o := s.faces[fid]
		if o == nil || o.ID == f.ID || o.NotFace || o.Picture {
			continue
		}
		if oc := s.clusters[o.ClusterID]; o.Tagged == c.PersonID || oc != nil && oc.PersonID == c.PersonID {
			return true
		}
	}
	return false
}

// visible is how well a face can be made out on its crop, 0 (a dark or
// faint smudge) to 1; 1 when the crop can't be read.
func (s *Store) visible(f *faceRec) float32 {
	q := s.quality(f)
	if !q.ok {
		return 1
	}
	if q.sd < matchSD0 {
		return 0
	}
	return min(1, 0.3+0.7*(q.sd-matchSD0)/(matchSD1-matchSD0))
}

// bestRef is the best cosine similarity of unit vector u to any of refs,
// and which one.
func bestRef(u []float32, refs [][]float32) (float32, int) {
	best, at := float32(-1), -1
	for j, r := range refs {
		if d := dot(u, r); d > best {
			best, at = d, j
		}
	}
	return best, at
}

// readChunk is how many faces background work scores per hold of the
// library: an answer saved meanwhile waits for one chunk (a tenth of a
// second or so), not for the whole run.
const readChunk = 4000

// readSteps are how background work reads the library: read runs fn with
// the library readable, run is parallel a chunk at a time, each chunk
// with the library readable. Answers saved in between may leave what was
// read a little stale; the work must allow for that. held: the caller
// holds the library already (all of it is then read at once).
func (s *Store) readSteps(held bool) (read func(func()), run func(int, func(int))) {
	if held {
		return func(fn func()) { fn() }, parallel
	}
	read = func(fn func()) {
		s.mu.RLock()
		defer s.mu.RUnlock()
		fn()
	}
	run = func(n int, fn func(int)) {
		for start := 0; start < n; start += readChunk {
			read(func() { parallel(min(n, start+readChunk)-start, func(i int) { fn(start + i) }) })
		}
	}
	return read, run
}

// parallel runs fn(0..n-1) on all cores.
func parallel(n int, fn func(int)) {
	var wg sync.WaitGroup
	next := make(chan int, 256)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				fn(i)
			}
		}()
	}
	for i := range n {
		next <- i
	}
	close(next)
	wg.Wait()
}
