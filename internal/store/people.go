package store

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

// People recognition, built from match groups.
//
// Faces are filed into match groups: faces that look alike *and* imply the
// same birth year (capture date minus apparent age). A person is one or
// more groups. Siblings photographed at the same age can look nearly
// identical, but their implied birth years differ by their age gap, so they
// land in different groups and are never offered as each other.
//
// The flow: name an unknown group, then answer "Is this also <name>?" for
// the most similar other groups. New photos join an existing group when
// both face and birth year agree, and so belong to that group's person
// automatically; anything less certain starts a small new group that shows
// up as a candidate.

var bucketGroups = []byte("groups")

// Tunables for ArcFace cosine similarity.
const (
	clusterJoin     = 0.45 // similarity to a group centroid to join it
	candidateMin    = 0.30 // group-to-person similarity to offer "is this also"
	singleCandidate = 0.40 // stricter for one-face groups, which are noisy
	// autoAssign is the group-to-person similarity at which a group is
	// filed under a person without asking. Measured on a real library
	// (eval_test.go, hiding each named group's name): always right, and
	// a stranger matched someone this well in under 1% of cases.
	autoAssign = 0.60
)

// birthTolerance is how far implied birth years may differ for the same
// person at a given apparent age: age estimates are tighter for children.
func birthTolerance(age float32) float32 { return 2 + 0.15*age }

// cluster is a match group: running sums for its centroid and implied birth
// year, plus the persisted user decisions about it.
type cluster struct {
	id       int64
	sum      []float32
	nrm      float64 // length of sum, kept up to date
	n        int
	birthSum float64
	birthN   int
	ageSum   float64
	timeSum  float64 // capture years, for the group's time window
	timeN    int
	imgs     map[int64]int32 // photos with a face in the group: a person appears once per photo
	days     map[int64]int32 // days with a photo in the group: on several days, it's someone you know
	fg       int             // faces that aren't background faces

	PersonID int64   // the person this group is, 0 if unknown
	Auto     bool    // PersonID was filed automatically, not confirmed by the user
	Rejected []int64 // people the user said this group is not
	Hidden   bool    // strangers, statues, pets: never shown or suggested
	// SkippedAt is the group's size when the user skipped it in review;
	// it returns to the inbox once it has grown by half again.
	SkippedAt int
}

// groupRec is what is persisted about a group: only user decisions; the
// statistics are rebuilt from the faces at load.
type groupRec struct {
	ID        int64   `json:"id"`
	PersonID  int64   `json:"person,omitempty"`
	Auto      bool    `json:"auto,omitempty"`
	Rejected  []int64 `json:"not,omitempty"`
	Hidden    bool    `json:"hidden,omitempty"`
	SkippedAt int     `json:"skipped,omitempty"`
}

func (c *cluster) rec() groupRec {
	return groupRec{ID: c.id, PersonID: c.PersonID, Auto: c.Auto, Rejected: c.Rejected, Hidden: c.Hidden, SkippedAt: c.SkippedAt}
}

// clusterAdd and clusterRemove keep a group's running sums: centroid,
// implied birth year, apparent age and capture time.
func (s *Store) clusterAdd(c *cluster, f *faceRec) { s.clusterUpdate(c, f, 1) }

func (s *Store) clusterRemove(c *cluster, f *faceRec) { s.clusterUpdate(c, f, -1) }

func (s *Store) clusterUpdate(c *cluster, f *faceRec, sign int) {
	if c.sum == nil {
		c.sum = make([]float32, len(f.emb))
	}
	sg := float32(sign)
	for i, x := range f.emb {
		c.sum[i] += sg * x
	}
	c.n += sign
	c.nrm = norm(c.sum)
	if c.imgs == nil {
		c.imgs = map[int64]int32{}
	}
	if !f.Picture { // a picture of someone doesn't take their place in the photo
		if c.imgs[f.ImageID] += int32(sign); c.imgs[f.ImageID] <= 0 {
			delete(c.imgs, f.ImageID)
		}
	}
	if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil {
		if c.days == nil {
			c.days = map[int64]int32{}
		}
		d := *im.TakenAt / 86400
		if c.days[d] += int32(sign); c.days[d] <= 0 {
			delete(c.days, d)
		}
	}
	if sign > 0 {
		f.bg = s.isBackground(f) // remembered: the photo's other faces may change before removal
	}
	if !f.bg {
		c.fg += sign
	}
	if by, ok := s.impliedBirth(f); ok {
		c.birthSum += float64(sign) * float64(by)
		c.birthN += sign
		c.ageSum += float64(sign) * float64(f.Age)
	}
	if t, ok := s.captureYear(f); ok {
		c.timeSum += float64(sign) * float64(t)
		c.timeN += sign
	}
}

// captureYear is the fractional year the face's photo was taken.
func (s *Store) captureYear(f *faceRec) (float32, bool) {
	im := s.images[f.ImageID]
	if im == nil || im.TakenAt == nil || f.Picture {
		return 0, false
	}
	t := time.Unix(*im.TakenAt, 0).UTC()
	return float32(t.Year()) + float32(t.YearDay())/366, true
}

// isBackground reports whether a face is a passer-by: much smaller than
// the main face of its photo (when small, or among 4+ faces: spectators
// behind someone), behind one face that clearly stands out among 8+
// (an audience), small in a crowd, or tiny on its own. Such faces are still matched to known people, but
// never make review cards of their own. Measured on a real library
// (eval_test.go, TestEvalFaceSize): 58% of the unnamed faces, 5% of named.
func (s *Store) isBackground(f *faceRec) bool {
	im := s.images[f.ImageID]
	if im == nil {
		return false
	}
	w := f.Box[2] - f.Box[0]
	px := w * float32(max(im.Width, im.Height))
	var widths []float32
	for _, id := range s.facesByImage[f.ImageID] {
		if o := s.faces[id]; o != nil && !o.NotFace {
			widths = append(widths, o.Box[2]-o.Box[0])
		}
	}
	if len(widths) == 0 {
		return px < 40
	}
	slices.Sort(widths)
	crowd, largest, median := len(widths), widths[len(widths)-1], widths[len(widths)/2]
	smaller := w < 0.4*largest // well behind the main face
	// An audience: one face clearly in front of many similar ones.
	audience := crowd >= 8 && largest >= 1.8*median && w < 0.7*largest
	return px < 40 || (px < 100 && (crowd >= 10 || smaller)) || (crowd >= 4 && smaller) || audience
}

func (c *cluster) when() (float32, bool) {
	if c.timeN == 0 {
		return 0, false
	}
	return float32(c.timeSum / float64(c.timeN)), true
}

// timeWindow is how far (in years) a photo may be from a group's typical
// capture time and still join it. Faces change fast in childhood, so a
// group of a child spans little time; an adult's face is stable for years.
// This also keeps look-alike siblings apart when age estimates for small
// children are off.
func timeWindow(age float32) float32 { return 1 + 0.15*age }

func (c *cluster) birth() (birth, age float32, ok bool) {
	if c.birthN == 0 {
		return 0, 0, false
	}
	return float32(c.birthSum / float64(c.birthN)), float32(c.ageSum / float64(c.birthN)), true
}

func norm(v []float32) float64 {
	var s float64
	for _, x := range v {
		s += float64(x) * float64(x)
	}
	return math.Sqrt(s)
}

// sim is the cosine similarity between a group centroid and a vector.
func (c *cluster) sim(v []float32) float32 {
	n := c.nrm * norm(v)
	if n == 0 {
		return 0
	}
	var d float64
	for i, x := range c.sum {
		d += float64(x) * float64(v[i])
	}
	return float32(d / n)
}

// birthCompatible reports whether two implied birth years could be the
// same person; unknown birth years are always compatible.
func birthCompatible(b1, a1 float32, ok1 bool, b2, a2 float32, ok2 bool) bool {
	if !ok1 || !ok2 {
		return true
	}
	return abs32(b1-b2) <= birthTolerance(max(a1, a2))
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// impliedBirth returns capture year (fractional) minus apparent age.
func (s *Store) impliedBirth(f *faceRec) (float32, bool) {
	im := s.images[f.ImageID]
	if f.AgeVersion == 0 || f.Age < 0 || im == nil || im.TakenAt == nil || scanned(im) || f.Picture {
		return 0, false
	}
	t := time.Unix(*im.TakenAt, 0).UTC()
	return float32(t.Year()) + float32(t.YearDay())/366 - f.Age, true
}

func (s *Store) clusterOf(id int64) *cluster {
	c := s.clusters[id]
	if c == nil {
		c = &cluster{id: id}
		s.clusters[id] = c
		s.nextCluster = max(s.nextCluster, id)
	}
	return c
}

// rebuildClusters recomputes group statistics from the faces (at load).
func (s *Store) rebuildClusters() {
	s.clusters = map[int64]*cluster{}
	for _, f := range s.faces {
		if f.ClusterID != 0 {
			s.clusterAdd(s.clusterOf(f.ClusterID), f)
		}
	}
}

func (s *Store) loadGroups(tx *bolt.Tx) error {
	b, err := tx.CreateBucketIfNotExists(bucketGroups)
	if err != nil {
		return err
	}
	return b.ForEach(func(_, v []byte) error {
		var g groupRec
		if err := json.Unmarshal(v, &g); err != nil {
			return err
		}
		c := s.clusterOf(g.ID)
		c.PersonID, c.Auto, c.Rejected, c.Hidden, c.SkippedAt = g.PersonID, g.Auto, g.Rejected, g.Hidden, g.SkippedAt
		return nil
	})
}

// ---------- the people stage ----------

// NeedPeople returns aged faces (any faces, without an age model) that
// have no match group yet.
func (s *Store) NeedPeople(ctx context.Context, n int, skip map[int64]bool) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []int64
	for id, f := range s.faces {
		if f.ClusterID == 0 && (f.AgeVersion > 0 || s.NoAgeModel) && !f.NotFace && !skip[id] && !s.isJunk(f) {
			out = append(out, id)
		}
	}
	// In id order, so groups grow the same way on every run.
	slices.Sort(out)
	return out[:min(n, len(out))]
}

// ProcessFace files a face into the best matching group, or a new one.
func (s *Store) ProcessFace(ctx context.Context, faceID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.faces[faceID]
	if !ok || f.ClusterID != 0 || f.NotFace || s.isJunk(f) {
		return nil
	}
	gid := s.bestCluster(f)
	rec := *f
	rec.ClusterID = gid
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, rec.ID, &rec) }); err != nil {
		return err
	}
	f.ClusterID = gid
	c := s.clusterOf(gid)
	s.clusterAdd(c, f)
	if changed := s.autoAssign([]*cluster{c}); len(changed) > 0 {
		return s.db.Update(func(tx *bolt.Tx) error { return putGroup(tx, changed[0]) })
	}
	return nil
}

// recheckAuto un-files automatically filed groups that turn out to share
// photos with the person's confirmed faces. Call at load, in a write
// transaction.
func (s *Store) recheckAuto(tx *bolt.Tx) error {
	idx := s.confirmedIndex()
	n := 0
	for _, c := range s.clusters {
		if !c.Auto || c.PersonID == 0 {
			continue
		}
		if pi := idx[c.PersonID]; pi != nil && pi.clashes(c) {
			c.PersonID, c.Auto = 0, false
			if err := putGroup(tx, c); err != nil {
				return err
			}
			n++
		}
	}
	if n > 0 {
		slog.Info("un-filed groups that share photos with the person", "groups", n)
	}
	return nil
}

// autoAssignPerson is autoAssign after person pid gained a confirmed
// group: only their score can have risen, so unnamed groups are compared
// with their groups alone (in parallel), and the few that pass are checked
// against everyone as usual.
func (s *Store) autoAssignPerson(pid int64) []*cluster {
	minSim := s.autoMinLocked()
	if minSim == 0 {
		return nil // automatic filing is off
	}
	idx := s.confirmedIndex()
	pi := idx[pid]
	if pi == nil || len(pi.groups) == 0 {
		return nil
	}
	unit := clusterUnit
	refs := make([][]float32, len(pi.groups))
	for i, g := range pi.groups {
		refs[i] = unit(g)
	}
	var cs []*cluster
	for _, c := range s.clusters {
		if c.PersonID == 0 && !c.Hidden && c.n > 0 {
			cs = append(cs, c)
		}
	}
	pass := make([]bool, len(cs))
	var wg sync.WaitGroup
	next := make(chan int, 256)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				u := unit(cs[i])
				for _, r := range refs {
					var d float32
					for k, x := range u {
						d += x * r[k]
					}
					if d >= minSim {
						pass[i] = true
						break
					}
				}
			}
		}()
	}
	for i := range cs {
		next <- i
	}
	close(next)
	wg.Wait()
	var passed []*cluster
	for i, c := range cs {
		if pass[i] {
			passed = append(passed, c)
		}
	}
	return s.autoAssign(passed)
}

// autoAssign files unnamed groups under a person when they look like one
// of the person's confirmed groups at least autoAssign alike (and the
// birth year fits, and the user hasn't said no). Only confirmed groups
// count, so one automatic mistake can't pull in more. It returns the
// groups it changed, for the caller to persist.
func (s *Store) autoAssign(cs []*cluster) []*cluster {
	minSim := s.autoMinLocked()
	if minSim == 0 {
		return nil // automatic filing is off
	}
	idx := s.confirmedIndex()
	var changed []*cluster
	for _, c := range cs {
		if c.PersonID != 0 || c.Hidden || c.n == 0 {
			continue
		}
		if pid, score := idx.best(c); pid != 0 && score >= minSim {
			// Filing without asking: the age must fit strictly (suggestions
			// allow a very alike adult more, see fitsLike).
			if cb, ca, cok := c.birth(); !idx[pid].ages.fits(cb, ca, cok) {
				continue
			}
			c.PersonID, c.Auto = pid, true
			changed = append(changed, c)
		}
	}
	return changed
}

// bestCluster picks the unnamed group the face looks like, whose implied
// birth year agrees and whose photos are from the same period of life, or
// allocates a new group id. Named groups hold only faces the user saw.
func (s *Store) bestCluster(f *faceRec) int64 { return s.bestClusterOf(f, 0) }

// bestClusterOf is bestCluster among person pid's groups (0: unnamed
// ones). The cheap date checks run before the embedding comparison.
func (s *Store) bestClusterOf(f *faceRec, pid int64) int64 {
	by, aged := s.impliedBirth(f)
	ft, timed := s.captureYear(f)
	best, bestSim := int64(0), float32(clusterJoin)
	for id, c := range s.clusters {
		if c.n == 0 || c.PersonID != pid || c.imgs[f.ImageID] > 0 {
			continue // empty, someone else's, or already has a face from this photo
		}
		cb, ca, cok := c.birth()
		if !birthCompatible(by, f.Age, aged, cb, ca, cok) {
			continue // looks alike, but born at a different time: a sibling
		}
		if ct, ok := c.when(); ok && timed {
			age := f.Age
			if !aged {
				age = ca // unknown face age: use the group's
				if !cok {
					age = 0 // no age at all: be strict
				}
			}
			if abs32(ft-ct) > timeWindow(age) {
				continue // same look, different period of life
			}
		}
		if sim := c.simNorm(f.emb, f.en); sim >= bestSim {
			best, bestSim = id, sim
		}
	}
	if best == 0 {
		s.nextCluster++
		best = s.nextCluster
	}
	return best
}

// placeConfirmed moves face f, which the user said is person pid, into
// the person's group it fits (or a new one of theirs), and tags it. It
// changes memory only and returns the group; the caller stores both. Call
// with s.mu held for writing.
func (s *Store) placeConfirmed(f *faceRec, pid int64) *cluster {
	f.Tagged = pid
	if c := s.clusters[f.ClusterID]; c != nil && c.PersonID == pid {
		return c
	}
	if old := s.clusters[f.ClusterID]; old != nil {
		s.clusterRemove(old, f)
	}
	f.ClusterID = 0
	c := s.clusterOf(s.bestClusterOf(f, pid))
	c.PersonID, c.Auto, c.Hidden = pid, false, false
	c.Rejected = slices.DeleteFunc(slices.Clone(c.Rejected), func(id int64) bool { return id == pid })
	f.ClusterID = c.id
	s.clusterAdd(c, f)
	return c
}

// groupingVersion identifies the grouping rules; when it changes, all
// faces are regrouped once at startup (see regroup).
const groupingVersion = 8

var keyGroupingVersion = []byte("grouping_version")

// regroupIfNeeded regroups all faces when the grouping rules changed.
// Call during load, with the write transaction.
func (s *Store) regroupIfNeeded(tx *bolt.Tx) error {
	meta := tx.Bucket(bucketMeta)
	v, _ := strconv.Atoi(string(meta.Get(keyGroupingVersion)))
	if v >= groupingVersion {
		return nil
	}
	if len(s.faces) > 0 {
		// Regrouping rewrites every group: keep a copy of the database as
		// it was, so user decisions can always be recovered.
		backup := tx.DB().Path() + ".pre-regroup-" + time.Now().Format("20060102-150405")
		if err := tx.CopyFile(backup, 0o600); err != nil {
			return fmt.Errorf("backup before regrouping: %w", err)
		}
		slog.Info("database backed up before regrouping", "path", backup)
		if v < startOverBelow {
			if err := s.startOver(tx, v); err != nil {
				return err
			}
		}
		if err := s.regroup(tx); err != nil {
			return err
		}
	}
	return meta.Put(keyGroupingVersion, []byte(strconv.Itoa(groupingVersion)))
}

// startOverBelow: a library grouped under an older version than this
// starts over, with no face named (see startOver).
const startOverBelow = 8

// startOver forgets everything about who is who but people's names (and
// which "people" are paintings or statues): no
// face is named (each keeps who it first was as Was, a record, never a
// hint), and family, birth years, genders, pictures chosen, "not them",
// strangers hidden and pictures on the wall are gone. Detections said not
// to be faces, and photo dates, stay: they aren't about people. Naming
// starts from Discover, with faces the user sees. from is the grouping
// version the library had. Call before regroup, in its transaction.
func (s *Store) startOver(tx *bolt.Tx, from int) error {
	n := 0
	for _, f := range s.faces {
		if f.Tagged != 0 {
			if f.Was == 0 && s.people[f.Tagged] != nil {
				f.Was = f.Tagged
			}
			n++
		}
		// Before version 5 a named group's faces were all the user's
		// naming; since, only tagged ones are (the rest were filed).
		if c := s.clusters[f.ClusterID]; from < 5 && f.Was == 0 && f.Tagged == 0 && c != nil && c.PersonID != 0 && !c.Auto && s.people[c.PersonID] != nil {
			f.Was = c.PersonID
		}
		f.Tagged, f.Not, f.Picture, f.Live, f.Skipped = 0, nil, false, false, 0
	}
	for _, c := range s.clusters {
		c.PersonID, c.Auto, c.Hidden, c.Rejected, c.SkippedAt = 0, false, false, nil, 0
	}
	for id, p := range s.people {
		// A painting or statue stays one: a fact about a thing, not who's who.
		np := &personRec{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, Picture: p.Picture}
		s.people[id] = np
		if err := put(tx, bucketPeople, id, np); err != nil {
			return err
		}
	}
	slog.Info("starting over: only names kept", "were tagged", n, "people", len(s.people))
	return nil
}

// regroup rebuilds every match group with the current rules, processing
// faces in capture order, and carries the user's decisions over. Only the
// faces the user saw are named: they go into groups of their person's.
// The rest are grouped unnamed; a face whose old group was named keeps
// that as a hint (Was) for Discover. Hidden and rejected decisions carry
// over by majority: a decision covering at least half of a new group, and
// 3 faces or a fifth of it.
func (s *Store) regroup(tx *bolt.Tx) error {
	start := time.Now()
	type decision struct {
		hidden   bool
		rejected []int64
	}
	before := map[int64]decision{} // face id -> its old group's decisions
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil {
			before[f.ID] = decision{c.Hidden, c.Rejected}
			if c.PersonID != 0 && !c.Auto && f.Tagged == 0 && f.Was == 0 && s.people[c.PersonID] != nil {
				f.Was = c.PersonID
			}
		}
	}

	// Rebuild the unnamed groups from scratch, oldest photos first.
	var order, tagged []*faceRec
	for _, f := range s.faces {
		f.ClusterID = 0
		switch {
		case f.AgeVersion == 0 && !s.NoAgeModel || f.NotFace || s.isJunk(f):
		case f.Tagged != 0 && s.people[f.Tagged] != nil:
			tagged = append(tagged, f)
		default:
			f.Tagged = 0
			order = append(order, f)
		}
	}
	g := newGrouper(s, currentRules)
	g.run(order) // sorts order by capture time
	s.clusters, s.nextCluster = g.byID, int64(len(g.list))
	for _, f := range order {
		f.ClusterID = g.of[f.ID].id
	}

	// Carry hiding and rejections over by majority.
	members := map[int64][]*faceRec{}
	for _, f := range order {
		members[f.ClusterID] = append(members[f.ClusterID], f)
	}
	hidden := 0
	for gid, fs := range members {
		c := s.clusters[gid]
		hide, rejects := 0, map[int64]int{}
		for _, f := range fs {
			d := before[f.ID]
			if d.hidden {
				hide++
			}
			for _, r := range d.rejected {
				rejects[r]++
			}
		}
		enough := func(n int) bool { return (n >= 3 || n*5 >= len(fs)) && n*2 >= len(fs) }
		if enough(hide) {
			c.Hidden = true
			hidden++
		}
		for pid, n := range rejects {
			if enough(n) {
				c.Rejected = append(c.Rejected, pid)
			}
		}
	}

	// The faces the user saw, into their person's groups, oldest first
	// (so the groups follow them through life).
	slices.SortFunc(tagged, func(a, b *faceRec) int {
		ta, _ := s.captureYear(a)
		tb, _ := s.captureYear(b)
		return cmp.Or(cmp.Compare(ta, tb), cmp.Compare(a.ID, b.ID))
	})
	for _, f := range tagged {
		s.placeConfirmed(f, f.Tagged)
	}

	for _, f := range s.faces {
		if err := put(tx, bucketFaces, f.ID, f); err != nil {
			return err
		}
	}
	if err := tx.DeleteBucket(bucketGroups); err != nil && !errors.Is(err, bolt.ErrBucketNotFound) {
		return err
	}
	if _, err := tx.CreateBucket(bucketGroups); err != nil {
		return err
	}
	for _, c := range s.clusters {
		if c.PersonID != 0 || c.Hidden || len(c.Rejected) > 0 {
			if err := putGroup(tx, c); err != nil {
				return err
			}
		}
	}
	slog.Info("regrouped faces", "faces", len(order), "groups", len(members), "confirmed", len(tagged),
		"hidden", hidden, "took", time.Since(start).Round(time.Millisecond))
	return nil
}

// ---------- views ----------

// Person is the API view of a person.
type Person struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	BirthYear int     `json:"birth_year,omitempty"`
	EstBirth  float32 `json:"est_birth,omitempty"`
	Picture   bool    `json:"picture,omitempty"` // a painting, statue or poster
	// With a birth year: how old the person looks at the latest age with
	// enough photos ("at 50 looks 36").
	LooksAt    float32    `json:"looks_at,omitempty"`
	LooksAge   float32    `json:"looks_age,omitempty"`
	Groups     int        `json:"groups"`
	Faces      int        `json:"faces"`
	Photos     int        `json:"photos"`
	Cover      string     `json:"cover"`       // face crop key
	CoverImage int64      `json:"cover_image"` // the photo it is from
	Family     []Relative `json:"family,omitempty"`
	Gender     string     `json:"gender,omitempty"`     // as entered
	EstGender  string     `json:"est_gender,omitempty"` // estimated from their adult faces
}

// FaceView is the API view of a face.
type FaceView struct {
	ID       int64      `json:"id"`
	ImageID  int64      `json:"image"`
	Crop     string     `json:"crop"`
	Box      [4]float32 `json:"box"`
	Age      float32    `json:"age"`
	Taken    *int64     `json:"taken,omitempty"`
	Group    int64      `json:"group,omitempty"`
	PersonID int64      `json:"person,omitempty"`
	Eyes     float32    `json:"eyes,omitempty"` // how open the eyes are; 0 not judged, -1 couldn't be
	Person   string     `json:"person_name,omitempty"`
	Odd      bool       `json:"odd,omitempty"`   // among a group's least alike faces, in a sample
	Bg       bool       `json:"bg,omitempty"`    // a background face (isBackground)
	Score    float32    `json:"score,omitempty"` // likeness to a person, in FaceCandidates
	// Twice: its group is filed under this person, who is another face in
	// the same photo too (photo viewer): one of the two is wrong.
	Twice     int64    `json:"twice,omitempty"`
	TwiceName string   `json:"twice_name,omitempty"`
	Company   []string `json:"company,omitempty"` // their family photographed that day, in FaceCandidates
	Why       string   `json:"why,omitempty"`     // a relative of theirs beside it, in FaceCandidates
	Hold      string   `json:"hold,omitempty"`    // why it isn't filed under them by itself (Match), in FaceCandidates
	// Picture: a picture of the person (on the wall); PersonPicture: the
	// person is a painting or statue; MaybePicture: someone already in the photo
	// whom this face looks like, but not at their age at the time, so it
	// may be a picture of them (photo viewer only).
	Picture       bool `json:"picture,omitempty"`
	PersonPicture bool `json:"person_picture,omitempty"`
	// Auto: filed under the person automatically (FileFaces), not
	// confirmed by the user.
	Auto         bool  `json:"auto,omitempty"`
	MaybePicture int64 `json:"maybe_picture,omitempty"`
	// Strength is how strongly it's known to be them: "checked" (confirmed
	// alone), "named" (with others), "filed" (by itself).
	Strength string `json:"strength,omitempty"`
}

// GroupView is a match group with sample faces.
type GroupView struct {
	ID       int64   `json:"id"`
	Size     int     `json:"size"`
	EstBirth float32 `json:"est_birth,omitempty"`
	PersonID int64   `json:"person,omitempty"`
	// PersonName comes along so a card shows it even for someone named
	// after the page loaded its list of people.
	PersonName string     `json:"person_name,omitempty"`
	Auto       bool       `json:"auto,omitempty"`  // filed automatically
	Off        string     `json:"off,omitempty"`   // its faces read as the other gender than the person's: "reads as a man"
	Score      float32    `json:"score,omitempty"` // similarity, for candidates
	Faces      []FaceView `json:"faces"`
	Maybe      *Person    `json:"maybe,omitempty"` // likely person, for unknown groups
}

func (s *Store) faceView(f *faceRec) FaceView {
	v := FaceView{ID: f.ID, ImageID: f.ImageID, Box: f.Box, Age: f.Age, Group: f.ClusterID, Bg: s.passerBy(f), Eyes: f.Eyes}
	if c := s.clusters[f.ClusterID]; c != nil && c.Hidden {
		v.Bg = true // a stranger: shown faint, like a passer-by
	}
	if im := s.images[f.ImageID]; im != nil {
		v.Crop = im.Hash + "_" + strconv.Itoa(f.Index)
		if im.Rotate != 0 {
			v.Crop += "?r=" + strconv.Itoa(im.Rotate) // crops were redone upright
		}
		v.Taken = im.TakenAt
	}
	v.Picture = f.Picture
	if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 {
		v.PersonID = c.PersonID
		v.Auto = f.Tagged != c.PersonID // filed automatically, not seen
		_, st := s.faceStrength(f)
		v.Strength = st.String()
		if p := s.people[c.PersonID]; p != nil {
			v.Person = p.Name
			v.PersonPicture = p.Picture
		}
	}
	return v
}

// notSomeone reports whether a face doesn't count as a person in its
// photo: a picture of someone, or a painting or statue.
func (s *Store) notSomeone(f *faceRec) bool {
	if f.Picture {
		return true
	}
	c := s.clusters[f.ClusterID]
	return c != nil && c.PersonID != 0 && s.people[c.PersonID] != nil && s.people[c.PersonID].Picture
}

// groupFaces returns a group's faces, best-looking (large, confident) first.
func (s *Store) groupFaces(gid int64) []*faceRec {
	var fs []*faceRec
	for _, f := range s.faces {
		if f.ClusterID == gid {
			fs = append(fs, f)
		}
	}
	slices.SortFunc(fs, func(a, b *faceRec) int {
		return cmp.Or(cmp.Compare((b.Box[2]-b.Box[0])*b.Score, (a.Box[2]-a.Box[0])*a.Score), cmp.Compare(a.ID, b.ID))
	})
	return fs
}

func (s *Store) groupView(c *cluster, faces []*faceRec, sample int) GroupView {
	g := GroupView{ID: c.id, Size: c.n, PersonID: c.PersonID, Auto: c.Auto}
	if p := s.people[c.PersonID]; p != nil {
		g.PersonName = p.Name
	}
	if b, _, ok := c.birth(); ok {
		g.EstBirth = b
	}
	for i, f := range s.sampleFaces(c, faces, sample) {
		v := s.faceView(f)
		v.Odd = i >= sample-sample/2 && len(faces) > sample
		g.Faces = append(g.Faces, v)
	}
	return g
}

// sampleFaces picks n faces to show for a group: the clearest half, and
// the half that looks least like the group. Group decisions are made from
// these, so the doubtful faces must be among them.
func (s *Store) sampleFaces(c *cluster, faces []*faceRec, n int) []*faceRec {
	if len(faces) <= n {
		return faces
	}
	best := n - n/2
	rest := slices.Clone(faces[best:])
	slices.SortFunc(rest, func(a, b *faceRec) int {
		return cmp.Or(cmp.Compare(c.sim(a.emb), c.sim(b.emb)), cmp.Compare(a.ID, b.ID))
	})
	return append(slices.Clone(faces[:best]), rest[:n/2]...)
}

// facesByGroup indexes faces by group in one pass.
func (s *Store) facesByGroup() map[int64][]*faceRec {
	m := map[int64][]*faceRec{}
	for _, f := range s.faces {
		if f.ClusterID != 0 {
			m[f.ClusterID] = append(m[f.ClusterID], f)
		}
	}
	for _, fs := range m {
		slices.SortFunc(fs, func(a, b *faceRec) int {
			return cmp.Or(cmp.Compare((b.Box[2]-b.Box[0])*b.Score, (a.Box[2]-a.Box[0])*a.Score), cmp.Compare(a.ID, b.ID))
		})
	}
	return m
}

// facesOfGroups is facesByGroup for a few groups only.
func facesOfGroups[T any](s *Store, items []T, id func(T) int64) map[int64][]*faceRec {
	want := map[int64]bool{}
	for _, it := range items {
		want[id(it)] = true
	}
	m := map[int64][]*faceRec{}
	for _, f := range s.faces {
		if want[f.ClusterID] {
			m[f.ClusterID] = append(m[f.ClusterID], f)
		}
	}
	for _, fs := range m {
		slices.SortFunc(fs, func(a, b *faceRec) int {
			return cmp.Or(cmp.Compare((b.Box[2]-b.Box[0])*b.Score, (a.Box[2]-a.Box[0])*a.Score), cmp.Compare(a.ID, b.ID))
		})
	}
	return m
}

// personBirth is the known birth year, or the face-weighted mean of the
// person's groups' implied birth years.
func (s *Store) personBirth(p *personRec) (birth, age float32, ok bool) {
	if p.BirthYear != 0 {
		return float32(p.BirthYear) + 0.5, 0, true
	}
	// From the faces the user confirmed, once there are enough: faces
	// filed by themselves mustn't move it (see ageModels).
	var sum, ages float64
	var n int
	for _, f := range s.faces {
		if f.Tagged != p.ID {
			continue
		}
		if by, ok := s.impliedBirth(f); ok {
			sum += float64(by)
			ages += float64(f.Age)
			n++
		}
	}
	if n >= ageConfirmed {
		return float32(sum / float64(n)), float32(ages / float64(n)), true
	}
	sum, ages, n = 0, 0, 0
	for _, c := range s.clusters {
		if c.PersonID == p.ID && c.birthN > 0 {
			sum += c.birthSum
			ages += c.ageSum
			n += c.birthN
		}
	}
	if n == 0 {
		return 0, 0, false
	}
	return float32(sum / float64(n)), float32(ages / float64(n)), true
}

// People lists named people, most photographed first.
func (s *Store) People(ctx context.Context) []Person {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byGroup := s.facesByGroup()
	fam, in, est := s.family(), s.entered(), s.genderEstimates()
	gender := func(pid int64) string {
		if q := s.people[pid]; q != nil {
			g, _ := genderOf(q, est[pid])
			return g
		}
		return ""
	}
	out := make([]Person, 0, len(s.people))
	for _, p := range s.people {
		v := Person{ID: p.ID, Name: p.Name, BirthYear: p.BirthYear, Picture: p.Picture, Family: s.relativesView(fam[p.ID], in[p.ID], gender),
			Gender: p.Gender, EstGender: est[p.ID]}
		if b, _, ok := s.personBirth(p); ok && !p.Picture {
			v.EstBirth = b
		}
		if p.BirthYear != 0 && !p.Picture {
			v.LooksAt, v.LooksAge = s.looksNow(p)
		}
		photos := map[int64]bool{}
		var cover *faceRec
		var coverGroup int
		for id, c := range s.clusters {
			if c.PersonID != p.ID {
				continue
			}
			v.Groups++
			v.Faces += c.n
			fs := byGroup[id]
			for _, f := range fs {
				photos[f.ImageID] = true
			}
			// Cover: the best face of the person's biggest group.
			if len(fs) > 0 && c.n > coverGroup {
				cover, coverGroup = fs[0], c.n
			}
		}
		v.Photos = len(photos)
		if f := s.pickedCover(p); f != nil {
			cover = f
		} else if f := s.autoCover(p, byGroup); f != nil {
			cover = f
		}
		if cover != nil {
			v.Cover = s.faceView(cover).Crop
			v.CoverImage = cover.ImageID
		}
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b Person) int { return cmp.Or(cmp.Compare(b.Photos, a.Photos), strings.Compare(a.Name, b.Name)) })
	return out
}

// UnknownGroups lists groups without a person (and not hidden) with at
// least minSize faces, biggest first, each with a likely person if one
// stands out.
func (s *Store) UnknownGroups(ctx context.Context, minSize, limit, sample int) []GroupView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byGroup := s.facesByGroup()
	var cs []*cluster
	for _, c := range s.clusters {
		if c.PersonID == 0 && !c.Hidden && c.n >= minSize {
			cs = append(cs, c)
		}
	}
	slices.SortFunc(cs, func(a, b *cluster) int { return cmp.Or(cmp.Compare(b.n, a.n), cmp.Compare(a.id, b.id)) })
	idx := s.personIndex()
	out := make([]GroupView, 0, min(limit, len(cs)))
	for _, c := range cs[:min(limit, len(cs))] {
		g := s.groupView(c, byGroup[c.id], sample)
		if pid, score := idx.best(c); pid != 0 && score >= 0.45 {
			p := s.people[pid]
			g.Maybe = &Person{ID: p.ID, Name: p.Name}
		}
		out = append(out, g)
	}
	return out
}

// clusterUnit is a group's centroid scaled to length 1, so similarities
// to many others are plain dot products.
func clusterUnit(c *cluster) []float32 {
	n := float32(c.nrm)
	u := make([]float32, len(c.sum))
	if n > 0 {
		for i, x := range c.sum {
			u[i] = x / n
		}
	}
	return u
}

// groupSim is the cosine similarity of two group centroids.
func groupSim(a, b *cluster) float32 {
	n := norm(a.sum) * norm(b.sum)
	if n == 0 {
		return 0
	}
	var d float64
	for i, x := range a.sum {
		d += float64(x) * float64(b.sum[i])
	}
	return float32(d / n)
}

// personIndex holds each named person's groups and birth year, gathered in
// one pass so scoring many groups against people stays linear.
type personIndex map[int64]*personInfo

type personInfo struct {
	p      *personRec
	groups []*cluster
	imgs   map[int64]bool // photos with a face in their groups
	strict bool           // any shared photo clashes (for automatic filing)
	ages   ageModel
	gender string // as entered, else estimated from their faces; "" unknown
}

func (s *Store) personIndex() personIndex { return s.indexPeople(false) }

// confirmedIndex is personIndex with only the groups the user confirmed.
func (s *Store) confirmedIndex() personIndex { return s.indexPeople(true) }

func (s *Store) indexPeople(confirmedOnly bool) personIndex {
	idx := personIndex{}
	for id, p := range s.people {
		idx[id] = &personInfo{p: p}
	}
	for _, c := range s.clusters {
		if pi := idx[c.PersonID]; pi != nil && c.n > 0 && !(confirmedOnly && (c.Auto || c.n < 2)) {
			pi.groups = append(pi.groups, c)
		}
	}
	s.ageModels(idx)
	for _, pi := range idx {
		if pi.p.Picture {
			pi.ages = ageModel{} // a painting never ages: any age fits
		}
	}
	for _, pi := range idx {
		pi.strict = confirmedOnly // used for automatic filing
		pi.imgs = map[int64]bool{}
		for _, g := range pi.groups {
			for img := range g.imgs {
				pi.imgs[img] = true
			}
		}
	}
	return idx
}

// clashes reports whether group c keeps turning up in the same photos as
// the person: then it is someone else (a person appears once per photo),
// however alike they look, like two children of the same age who are
// often photographed together.
// One shared photo in a big group is tolerated, as a stray face.
func (pi *personInfo) clashes(c *cluster) bool {
	shared := 0
	for img := range c.imgs {
		if pi.imgs[img] {
			shared++
		}
	}
	if pi.strict {
		return shared > 0 // a guess must never put someone twice in a photo
	}
	return shared >= 3 || shared*10 > len(c.imgs)
}

// score is how much group c looks like person pid: its best similarity to
// any of the person's groups. ok is false if c can't be them (rejected, or
// born at a different time).
func (idx personIndex) score(c *cluster, pid int64) (float32, bool) {
	pi := idx[pid]
	if pi == nil || slices.Contains(c.Rejected, pid) || pi.clashes(c) {
		return 0, false
	}
	best := float32(-1)
	for _, o := range pi.groups {
		best = max(best, groupSim(c, o))
	}
	if cb, ca, cok := c.birth(); !pi.ages.fitsLike(cb, ca, cok, best) {
		return 0, false
	}
	return best, best > -1
}

func (idx personIndex) best(c *cluster) (int64, float32) {
	var best int64
	bestScore := float32(-1)
	for pid := range idx {
		if score, ok := idx.score(c, pid); ok && score > bestScore {
			best, bestScore = pid, score
		}
	}
	return best, bestScore
}

// Candidates returns unknown groups most likely to be the person, for
// "Is this also <name>?", best first.
func (s *Store) Candidates(ctx context.Context, personID int64, limit, sample int) []GroupView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type cand struct {
		c     *cluster
		score float32
	}
	idx := s.personIndex()
	pi := idx[personID]
	if pi == nil {
		return nil
	}
	refs := make([][]float32, len(pi.groups))
	for i, g := range pi.groups {
		refs[i] = clusterUnit(g)
	}
	var cs []*cluster
	for _, c := range s.clusters {
		if c.PersonID == 0 && !c.Hidden && c.n > 0 {
			cs = append(cs, c)
		}
	}
	// The same rules as idx.score, with the person's centroids prepared
	// once and the groups scored in parallel.
	scores := make([]float32, len(cs))
	var wg sync.WaitGroup
	next := make(chan int, 256)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				c := cs[i]
				scores[i] = -1
				if slices.Contains(c.Rejected, personID) || pi.clashes(c) {
					continue
				}
				u := clusterUnit(c)
				best := float32(-1)
				for _, r := range refs {
					var d float32
					for k, x := range u {
						d += x * r[k]
					}
					best = max(best, d)
				}
				if cb, ca, cok := c.birth(); pi.ages.fitsLike(cb, ca, cok, best) {
					scores[i] = best
				}
			}
		}()
	}
	for i := range cs {
		next <- i
	}
	close(next)
	wg.Wait()
	var cands []cand
	for i, c := range cs {
		if score := scores[i]; score >= candidateMin && (c.n > 1 || score >= singleCandidate) {
			cands = append(cands, cand{c, score})
		}
	}
	slices.SortFunc(cands, func(a, b cand) int { return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.c.id, b.c.id)) })
	cands = cands[:min(limit, len(cands))]
	byGroup := facesOfGroups(s, cands, func(cd cand) int64 { return cd.c.id })
	out := make([]GroupView, 0, len(cands))
	for _, cd := range cands {
		g := s.groupView(cd.c, byGroup[cd.c.id], sample)
		g.Score = cd.score
		out = append(out, g)
	}
	return out
}

// PersonGroups returns the groups that make up a person, biggest first.
func (s *Store) PersonGroups(ctx context.Context, personID int64, sample int) []GroupView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byGroup := s.facesByGroup()
	gender := ""
	if p := s.people[personID]; p != nil {
		gender, _ = genderOf(p, s.genderEstimates()[personID])
	}
	var out []GroupView
	for _, c := range s.clusters {
		if c.PersonID == personID && c.n > 0 {
			g := s.groupView(c, byGroup[c.id], sample)
			// A group that clearly reads as the other gender was likely
			// filed under the wrong person.
			if genderMismatch(gender, byGroup[c.id]...) {
				g.Off = offWord(gender)
			}
			out = append(out, g)
		}
	}
	slices.SortFunc(out, func(a, b GroupView) int { return cmp.Or(cmp.Compare(b.Size, a.Size), cmp.Compare(a.ID, b.ID)) })
	return out
}

// GroupFaces returns all faces of a group, oldest photo first.
func (s *Store) GroupFaces(ctx context.Context, groupID int64) []FaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FaceView
	for _, f := range s.groupFaces(groupID) {
		out = append(out, s.faceView(f))
	}
	slices.SortFunc(out, func(a, b FaceView) int { return cmp.Compare(ptrOr(a.Taken), ptrOr(b.Taken)) })
	return out
}

func ptrOr(p *int64) int64 {
	if p == nil {
		return 0
	}
	return *p
}

// ImageFaces returns the faces in one image, left to right.
func (s *Store) ImageFaces(ctx context.Context, imageID int64) []FaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FaceView
	var fs []*faceRec
	for _, fid := range s.facesByImage[imageID] {
		if f := s.faces[fid]; s.shown(f) {
			out = append(out, s.faceView(f))
			fs = append(fs, f)
		}
	}
	// A person appears once in a photo: if two faces claim the same one,
	// the stronger claim keeps the name (tagged, then confirmed, then the
	// bigger face) and the other shows as unknown.
	claim := func(i int) (int, float32) {
		f, c := fs[i], s.clusters[fs[i].ClusterID]
		rank := 0
		switch {
		case f.Tagged == out[i].PersonID:
			rank = 2
		case c != nil && !c.Auto:
			rank = 1
		}
		return rank, f.Box[2] - f.Box[0]
	}
	best := map[int64]int{}
	for i, v := range out {
		if v.PersonID == 0 || fs[i].Picture {
			continue // a picture of someone may hang behind them
		}
		j, seen := best[v.PersonID]
		if !seen {
			best[v.PersonID] = i
			continue
		}
		ri, wi := claim(i)
		rj, wj := claim(j)
		loser := i
		if ri > rj || (ri == rj && wi > wj) {
			best[v.PersonID], loser = i, j
		}
		if s.outOfTime(fs[loser], v.PersonID) && !s.outOfTime(fs[best[v.PersonID]], v.PersonID) {
			out[loser].MaybePicture = v.PersonID
		}
		// Shown as the other face's, not quietly as unknown: its group is
		// filed under them, and one of the two is wrong.
		out[loser].Twice, out[loser].TwiceName = out[loser].PersonID, out[loser].Person
		out[loser].PersonID, out[loser].Person = 0, ""
	}
	slices.SortFunc(out, func(a, b FaceView) int { return cmp.Compare(a.Box[0], b.Box[0]) })
	return out
}

// PersonImageIDs returns the images showing a person.
func (s *Store) PersonImageIDs(personID int64) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[int64]bool{}
	var out []int64
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID == personID && !seen[f.ImageID] && !f.Picture {
			seen[f.ImageID] = true
			out = append(out, f.ImageID)
		}
	}
	return out
}

// PictureImageIDs returns photos with a picture of person pid in them (on
// a wall, in a frame); of anyone, with pid 0.
func (s *Store) PictureImageIDs(pid int64) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[int64]bool{}
	var out []int64
	for _, f := range s.faces {
		if !f.Picture || f.NotFace || seen[f.ImageID] {
			continue
		}
		if c := s.clusters[f.ClusterID]; pid != 0 && (c == nil || c.PersonID != pid) && f.Tagged != pid {
			continue
		}
		seen[f.ImageID] = true
		out = append(out, f.ImageID)
	}
	return out
}

// passerBy reports whether a face is shown as background (faint,
// unlabelled): a background face of someone not named and seen on one day
// only. Someone named, or seen on several days, matters however small (a
// spectator recurs too, but only at the one event).
func (s *Store) passerBy(f *faceRec) bool {
	c := s.clusters[f.ClusterID]
	return (c == nil || (c.PersonID == 0 && len(c.days) < 2)) && s.isBackground(f)
}

// AloneImageIDs returns photos with exactly one person: background faces,
// junk and "not a person" detections don't count.
func (s *Store) AloneImageIDs() []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []int64
	for img, ids := range s.facesByImage {
		n := 0
		for _, id := range ids {
			f := s.faces[id]
			if f == nil || f.NotFace || (f.ClusterID == 0 && s.isJunk(f)) || s.passerBy(f) || s.notSomeone(f) {
				continue
			}
			if n++; n > 1 {
				break
			}
		}
		if n == 1 {
			out = append(out, img)
		}
	}
	return out
}

// ---------- user actions ----------

// ErrNameRequired is returned when naming without a name or person.
var ErrNameRequired = errors.New("name or person required")

func putGroup(tx *bolt.Tx, c *cluster) error {
	data, err := json.Marshal(c.rec())
	if err != nil {
		return err
	}
	return tx.Bucket(bucketGroups).Put(key(c.id), data)
}

// NameGroup says every face in group gid is a person: an existing one
// (personID) or a new or existing one by name, as if each was confirmed
// (for views that show a whole group). It returns the person id.
func (s *Store) NameGroup(ctx context.Context, gid, personID int64, name string) (int64, error) {
	s.mu.RLock()
	c := s.clusters[gid]
	var ids []int64
	if c != nil {
		for _, f := range s.groupFaces(gid) {
			ids = append(ids, f.ID)
		}
	}
	s.mu.RUnlock()
	if c == nil {
		return 0, ErrNotFound
	}
	if personID == 0 && strings.TrimSpace(name) == "" {
		return 0, ErrNameRequired
	}
	return s.ConfirmFaces(ctx, personID, name, ids, nil)
}

// TagFace says this one face is a person (new or existing by name), as
// when tagging someone in a photo: it joins the person's group it fits.
func (s *Store) TagFace(ctx context.Context, faceID, personID int64, name string) (int64, error) {
	s.mu.RLock()
	_, ok := s.faces[faceID]
	s.mu.RUnlock()
	if !ok {
		return 0, ErrNotFound
	}
	pid, err := s.ConfirmFaces(ctx, personID, name, []int64{faceID}, nil)
	if err != nil {
		return 0, err
	}
	// Looked at on its own: not doubted again.
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.faces[faceID]
	if f == nil || f.Checked {
		return pid, nil
	}
	f.Checked = true
	ch := s.newChanges()
	ch.face(f)
	return pid, ch.save()
}

// KnownPerson reports whether a person by that id or name exists already.
func (s *Store) KnownPerson(personID int64, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, isNew, err := s.resolvePerson(personID, name)
	return err == nil && !isNew
}

// resolvePerson finds a person by id, or by name (any case), or makes a
// new one by name (not yet stored: the caller assigns the id).
func (s *Store) resolvePerson(personID int64, name string) (*personRec, bool, error) {
	name = strings.TrimSpace(name)
	switch {
	case personID != 0:
		if p := s.people[personID]; p != nil {
			return p, false, nil
		}
		return nil, false, ErrNotFound
	case name != "":
		for _, q := range s.people {
			if strings.EqualFold(q.Name, name) {
				return q, false, nil
			}
		}
		return &personRec{Name: name, CreatedAt: time.Now().Unix()}, true, nil
	}
	return nil, false, ErrNameRequired
}

// RejectGroup records that group gid is not person pid ("No" to "Is this
// also …?"), and unassigns it if it was.
func (s *Store) RejectGroup(ctx context.Context, gid, pid int64) error {
	return s.updateGroup(gid, func(c *cluster) {
		if !slices.Contains(c.Rejected, pid) {
			c.Rejected = append(slices.Clone(c.Rejected), pid)
		}
		if c.PersonID == pid {
			c.PersonID, c.Auto = 0, false
		}
	})
}

// HideGroup hides (or unhides) a group of strangers, statues, pets.
func (s *Store) HideGroup(ctx context.Context, gid int64, hidden bool) error {
	return s.updateGroup(gid, func(c *cluster) {
		c.Hidden = hidden
		if hidden {
			c.PersonID, c.Auto = 0, false
		}
	})
}

func (s *Store) updateGroup(gid int64, change func(*cluster)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.clusters[gid]
	if c == nil {
		return ErrNotFound
	}
	upd := *c
	change(&upd)
	if err := s.db.Update(func(tx *bolt.Tx) error { return putGroup(tx, &upd) }); err != nil {
		return err
	}
	c.PersonID, c.Auto, c.Rejected, c.Hidden, c.SkippedAt = upd.PersonID, upd.Auto, upd.Rejected, upd.Hidden, upd.SkippedAt
	return nil
}

// RemoveFace takes one face out of its group (a wrong match) into a group
// of its own, remembering that it is not the group's person.
func (s *Store) RemoveFace(ctx context.Context, faceID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.faces[faceID]
	if !ok {
		return ErrNotFound
	}
	old := s.clusters[f.ClusterID]
	s.nextCluster++
	nc := &cluster{id: s.nextCluster}
	if old != nil && old.PersonID != 0 {
		nc.Rejected = []int64{old.PersonID}
	}
	if f.Tagged != 0 && !slices.Contains(nc.Rejected, f.Tagged) {
		nc.Rejected = append(nc.Rejected, f.Tagged)
	}
	c := *f
	c.ClusterID, c.Tagged = nc.id, 0
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := put(tx, bucketFaces, c.ID, &c); err != nil {
			return err
		}
		return putGroup(tx, nc)
	}); err != nil {
		return err
	}
	if old != nil {
		s.clusterRemove(old, f)
	}
	f.ClusterID, f.Tagged = nc.id, 0
	s.clusters[nc.id] = nc
	s.clusterAdd(nc, f)
	return nil
}

// NotAFace marks a detection as not a person (a cat, a pattern) and takes
// it out of its group for good.
func (s *Store) NotAFace(ctx context.Context, faceID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.faces[faceID]
	if !ok {
		return ErrNotFound
	}
	c := *f
	c.ClusterID, c.NotFace = 0, true
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, c.ID, &c) }); err != nil {
		return err
	}
	if old := s.clusters[f.ClusterID]; old != nil {
		s.clusterRemove(old, f)
	}
	f.ClusterID, f.NotFace = 0, true
	return nil
}

// autoCover picks a person's picture when they haven't chosen one: a face
// looking at the camera (required when there is one), typical of them
// (closest to their average face), preferably recent, and large enough. Only
// faces from groups the user confirmed count when there are any;
// background faces and junk never do.
func (s *Store) autoCover(p *personRec, byGroup map[int64][]*faceRec) *faceRec {
	var confirmed, all []*faceRec
	for id, c := range s.clusters {
		if c.PersonID != p.ID {
			continue
		}
		for _, f := range byGroup[id] {
			if f.NotFace || f.bg || f.Score < 0.6 || s.facePx(f) < 64 || s.isJunk(f) {
				continue
			}
			all = append(all, f)
			if !c.Auto {
				confirmed = append(confirmed, f)
			}
		}
	}
	cands := all
	if len(confirmed) > 0 {
		cands = confirmed
	}
	if len(cands) == 0 {
		return nil
	}
	// Facing the camera is a must when anyone does.
	var frontal []*faceRec
	for _, f := range cands {
		if s.frontal(f) >= 0.75 {
			frontal = append(frontal, f)
		}
	}
	if len(frontal) > 0 {
		cands = frontal
	}
	var latest float32
	for _, f := range cands {
		if t, ok := s.captureYear(f); ok {
			latest = max(latest, t)
		}
	}
	// Recent: their last three years, if that leaves a few (children's
	// faces change fast; an old photo isn't how they look).
	var recent []*faceRec
	for _, f := range cands {
		if t, ok := s.captureYear(f); ok && t >= latest-3 {
			recent = append(recent, f)
		}
	}
	if len(recent) >= 5 {
		cands = recent
	}
	// Typical: similarity to their average face, where each day of
	// photos counts once, so one big event (a holiday, a party) can't
	// define how someone looks.
	days := map[int64][]float32{}
	for _, f := range cands {
		day := int64(0)
		if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil {
			day = *im.TakenAt / 86400
		}
		d := days[day]
		if d == nil {
			d = make([]float32, len(f.emb))
			days[day] = d
		}
		for i, x := range f.emb {
			d[i] += x
		}
	}
	mean := make([]float32, len(cands[0].emb))
	for _, d := range days {
		dn := float32(norm(d))
		for i, x := range d {
			mean[i] += x / dn
		}
	}
	mn := float32(norm(mean))
	type cand struct {
		f     *faceRec
		score float32
	}
	var scored []cand
	var best *faceRec
	bestScore := float32(-1)
	for _, f := range cands {
		var sim float32
		for i, x := range f.emb {
			sim += x * mean[i]
		}
		sim /= mn * float32(norm(f.emb))
		score := sim + 0.8*s.frontal(f) + 0.2*min(s.facePx(f)/250, 1) + 0.2*f.Score
		scored = append(scored, cand{f, score})
	}
	// The best few are then judged on their pixels: sharp, eyes open.
	slices.SortFunc(scored, func(a, b cand) int { return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.f.ID, b.f.ID)) })
	scored = scored[:min(12, len(scored))]
	var maxSharp, maxEyes float32
	qs := make([]faceQuality, len(scored))
	for i, c := range scored {
		qs[i] = s.quality(c.f)
		maxSharp, maxEyes = max(maxSharp, qs[i].sharp), max(maxEyes, qs[i].eyes)
	}
	for i, c := range scored {
		score := c.score
		if qs[i].ok && maxSharp > 0 && maxEyes > 0 {
			score += 0.3*qs[i].sharp/maxSharp + 0.3*qs[i].eyes/maxEyes
		}
		if best == nil || score > bestScore {
			best, bestScore = c.f, score
		}
	}
	return best
}

// facePx is a face's width in pixels of the original photo.
func (s *Store) facePx(f *faceRec) float32 {
	im := s.images[f.ImageID]
	if im == nil {
		return 0
	}
	return (f.Box[2] - f.Box[0]) * float32(max(im.Width, im.Height))
}

// frontal is 1 for a face looking straight at the camera and upright,
// falling to 0 when turned or tilted, judged from the eye and nose
// landmarks (in pixels, as they are stored relative to width and height).
func (s *Store) frontal(f *faceRec) float32 {
	im := s.images[f.ImageID]
	if im == nil {
		return 0
	}
	w, h := float64(im.Width), float64(im.Height)
	le, re, nose := f.Landmarks[0], f.Landmarks[1], f.Landmarks[2]
	lx, ly := float64(le[0])*w, float64(le[1])*h
	rx, ry := float64(re[0])*w, float64(re[1])*h
	nx := float64(nose[0]) * w
	eye := math.Hypot(rx-lx, ry-ly)
	if eye == 0 {
		return 0
	}
	yaw := math.Abs(nx-(lx+rx)/2) / eye        // 0 when the nose is between the eyes
	roll := math.Abs(math.Atan2(ry-ly, rx-lx)) // radians off level
	// Pitch: looking down (or up) shortens the eyes-to-mouth distance
	// against the distance between the eyes; about 1.1 face-on.
	mouthY := (float64(f.Landmarks[3][1]) + float64(f.Landmarks[4][1])) / 2 * h
	pitch := min(1, max(0, ((mouthY-(ly+ry)/2)/eye-0.7)/0.35))
	return float32(max(0, 1-2*yaw) * max(0, 1-roll/0.5) * pitch)
}

// pickedCover is the face the user chose as the person's picture, while
// it is still theirs; nil otherwise.
func (s *Store) pickedCover(p *personRec) *faceRec {
	f := s.faces[p.CoverFace]
	if f == nil || f.NotFace {
		return nil
	}
	if c := s.clusters[f.ClusterID]; c == nil || c.PersonID != p.ID {
		return nil
	}
	return f
}

// UpdatePerson renames a person, sets their birth year, or picks the face
// shown as their picture.
// PersonEdit is a change to a person; nil fields stay as they are.
type PersonEdit struct {
	Name      *string `json:"name"`
	BirthYear *int    `json:"birth_year"`
	// BirthUnknown: asked the birth year, the user doesn't know it.
	BirthUnknown *bool   `json:"birth_unknown"`
	CoverFace    *int64  `json:"cover_face"`
	Picture      *bool   `json:"picture"`
	Gender       *string `json:"gender"` // "female", "male", or "" to leave it to the faces
}

func (s *Store) UpdatePerson(ctx context.Context, id int64, e PersonEdit) error {
	name, birthYear, coverFace := e.Name, e.BirthYear, e.CoverFace
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.people[id]
	if !ok {
		return ErrNotFound
	}
	c := *p
	if name != nil && strings.TrimSpace(*name) != "" {
		c.Name = strings.TrimSpace(*name)
	}
	if birthYear != nil {
		c.BirthYear = *birthYear
	}
	if e.BirthUnknown != nil {
		c.BirthUnknown = *e.BirthUnknown
	}
	if coverFace != nil {
		c.CoverFace = *coverFace
	}
	if e.Picture != nil {
		c.Picture = *e.Picture
	}
	if e.Gender != nil {
		if g := *e.Gender; g != "" && g != Female && g != Male {
			return ErrBadRelation
		}
		c.Gender = *e.Gender
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketPeople, id, &c) }); err != nil {
		return err
	}
	*p = c
	return nil
}

// MergePeople moves all of from's groups to into and deletes from.
func (s *Store) MergePeople(ctx context.Context, from, into int64) error {
	if from == into {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.people[from] == nil || s.people[into] == nil {
		return ErrNotFound
	}
	var changed []*cluster
	var upds []cluster
	for _, c := range s.clusters {
		touched := c.PersonID == from || slices.Contains(c.Rejected, from)
		if !touched {
			continue
		}
		u := *c
		if u.PersonID == from {
			u.PersonID = into
		}
		if slices.Contains(u.Rejected, from) {
			u.Rejected = slices.DeleteFunc(slices.Clone(u.Rejected), func(id int64) bool { return id == from })
			if !slices.Contains(u.Rejected, into) {
				u.Rejected = append(u.Rejected, into)
			}
		}
		changed, upds = append(changed, c), append(upds, u)
	}
	rels := s.mergeRelations(from, into)
	err := s.db.Update(func(tx *bolt.Tx) error {
		for i := range upds {
			if err := putGroup(tx, &upds[i]); err != nil {
				return err
			}
		}
		for i := range rels {
			if err := put(tx, bucketPeople, rels[i].ID, &rels[i]); err != nil {
				return err
			}
		}
		return tx.Bucket(bucketPeople).Delete(key(from))
	})
	if err != nil {
		return err
	}
	for i, c := range changed {
		c.PersonID, c.Rejected = upds[i].PersonID, upds[i].Rejected
	}
	for _, u := range rels {
		*s.people[u.ID] = u
	}
	if s.people[into].BirthYear == 0 && s.people[from].BirthYear != 0 {
		s.people[into].BirthYear = s.people[from].BirthYear
	}
	delete(s.people, from)
	return nil
}

// ---------- review inbox ----------

// Suggestion is a person a group may be, with how alike they look.
type Suggestion struct {
	ID     int64   `json:"id"`
	Name   string  `json:"name"`
	Cover  string  `json:"cover"`
	Score  float32 `json:"score"`            // likeness
	Looked float32 `json:"looked,omitempty"` // how old they looked when the group's photos were taken
	Why    string  `json:"why,omitempty"`    // a relative of theirs in its photos: "with Bob (partner) in 3 of 5 photos"
	Kin    string  `json:"kin,omitempty"`    // related to another suggestion: "Carol's mother"
	Off    string  `json:"off,omitempty"`    // the face reads as the other gender: "reads as a man"
	rank   float32
}

// offWord says a face reads as the other gender than a person's.
func offWord(gender string) string {
	if gender == Female {
		return "reads as a man"
	}
	return "reads as a woman"
}

// suggest lists who group c (with faces) may be, likeliest first: by
// likeness, helped by relatives of theirs in its photos. Each says how
// old the person looked then, and relatives suggested together say how
// they are related, so a parent and child who look alike can be told
// apart. Call with s.mu held.
func (s *Store) suggest(c *cluster, faces []*faceRec, covers map[int64]string, idx personIndex, fam map[int64]map[int64]string) []Suggestion {
	out := []Suggestion{}
	cb, ca, cok := c.birth()
	for pid, p := range s.people {
		score, ok := idx.score(c, pid)
		if !ok {
			continue
		}
		share, why := s.kinGroup(faces, fam[pid])
		if score < candidateMin && (share < kinGroupShare || score < kinSim) {
			continue
		}
		sg := Suggestion{ID: pid, Name: p.Name, Cover: covers[pid], Score: score, Why: why, rank: score + kinGroupBonus*share}
		if pi := idx[pid]; pi != nil && genderMismatch(pi.gender, faces...) {
			sg.Off, sg.rank = offWord(pi.gender), sg.rank-genderMiss
		}
		if pi := idx[pid]; pi != nil && cok && !p.Picture {
			if l, ok := pi.ages.looksAt(cb + ca); ok {
				sg.Looked = l
			}
		}
		out = append(out, sg)
	}
	slices.SortFunc(out, func(a, b Suggestion) int { return cmp.Or(cmp.Compare(b.rank, a.rank), cmp.Compare(a.ID, b.ID)) })
	out = out[:min(3, len(out))]
	labelKin(out, fam, idx.genderOf)
	return out
}

// genderOf is a person's gender in the index ("" unknown).
func (idx personIndex) genderOf(pid int64) string {
	if pi := idx[pid]; pi != nil {
		return pi.gender
	}
	return ""
}

// labelKin says how suggestions that are relatives are related:
// "Carol’s mother".
func labelKin(out []Suggestion, fam map[int64]map[int64]string, gender func(int64) string) {
	for i := range out {
		for j := range out {
			if kind := fam[out[j].ID][out[i].ID]; i != j && kind != "" && out[i].Kin == "" {
				out[i].Kin = out[j].Name + "’s " + kinWord(kind, gender(out[i].ID)) // what i is to j
			}
		}
	}
}

// ReviewItem is an unknown group in the review inbox.
type ReviewItem struct {
	GroupView
	Suggestions []Suggestion `json:"suggestions"`
	Company     *Company     `json:"company,omitempty"` // who they're photographed with
}

// inReview reports whether an unknown group belongs in the inbox.
func (c *cluster) inReview(minSize int) bool {
	// Groups filed automatically stay until the user confirms them: only
	// confirmed groups make more groups confident matches.
	return (c.PersonID == 0 || c.Auto) && !c.Hidden && c.fg >= minSize && (c.SkippedAt == 0 || c.n*2 >= c.SkippedAt*3)
}

// ReviewFaces is how many faces the inbox groups hold.
func (s *Store) ReviewFaces(minSize int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, c := range s.clusters {
		if c.inReview(minSize) {
			n += c.n
		}
	}
	return n
}

// ReviewCount is how many groups wait in the inbox.
func (s *Store) ReviewCount(minSize int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, c := range s.clusters {
		if c.inReview(minSize) {
			n++
		}
	}
	return n
}

// Review returns inbox groups, biggest first, each with up to three
// suggested people (face similarity and birth year both fit).
func (s *Store) Review(ctx context.Context, minSize, offset, limit, sample int, order string) ([]ReviewItem, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var cs []*cluster
	for _, c := range s.clusters {
		if c.inReview(minSize) {
			cs = append(cs, c)
		}
	}
	idx := s.personIndex()
	company := s.company()
	bySize := func(a, b *cluster) int { return cmp.Or(cmp.Compare(b.n, a.n), cmp.Compare(a.id, b.id)) }
	when := func(c *cluster) float32 { t, _ := c.when(); return t }
	switch order {
	case ReviewKnown:
		score := func(c *cluster) float32 {
			if k := company[c.id]; k != nil {
				return k.Score
			}
			return 0
		}
		slices.SortFunc(cs, func(a, b *cluster) int { return cmp.Or(cmp.Compare(score(b), score(a)), bySize(a, b)) })
	case ReviewByMatch:
		// Automatically filed groups first (already confident matches),
		// then by how alike they are to someone.
		score := s.bestScores(cs, idx)
		slices.SortFunc(cs, func(a, b *cluster) int {
			return cmp.Or(cmp.Compare(boolInt(b.Auto), boolInt(a.Auto)), cmp.Compare(score[b], score[a]), bySize(a, b))
		})
	case ReviewNewest:
		slices.SortFunc(cs, func(a, b *cluster) int { return cmp.Or(cmp.Compare(when(b), when(a)), bySize(a, b)) })
	case ReviewOldest:
		slices.SortFunc(cs, func(a, b *cluster) int { return cmp.Or(cmp.Compare(when(a), when(b)), bySize(a, b)) })
	default:
		slices.SortFunc(cs, bySize)
	}
	total := len(cs)
	cs = cs[min(offset, len(cs)):]
	cs = cs[:min(limit, len(cs))]
	byGroup := s.facesByGroup()
	covers := s.personCovers(byGroup)
	out := make([]ReviewItem, 0, len(cs))
	for _, c := range cs {
		it := s.reviewItem(c, byGroup[c.id], covers, idx, sample)
		it.Company = company[c.id]
		out = append(out, it)
	}
	return out, total
}

// Review orders.
const (
	ReviewBySize  = "size"   // biggest groups first
	ReviewByMatch = "match"  // most like someone named first
	ReviewNewest  = "newest" // most recent photos first
	ReviewOldest  = "oldest"
	ReviewKnown   = "known" // seen most with people named: probably someone you know
)

// bestScores is each group's similarity to the person it looks most like,
// by the same rules as its suggestions (score), so the top of "likely
// match" always has a suggestion; computed in parallel with unit centroids.
func (s *Store) bestScores(cs []*cluster, idx personIndex) map[*cluster]float32 {
	unit := clusterUnit
	type ref struct {
		pid int64
		pi  *personInfo
		u   []float32
	}
	var refs []ref
	for pid, pi := range idx {
		for _, g := range pi.groups {
			refs = append(refs, ref{pid, pi, unit(g)})
		}
	}
	scores := make([]float32, len(cs))
	var wg sync.WaitGroup
	next := make(chan int)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				c := cs[i]
				u := unit(c)
				cb, ca, cok := c.birth()
				best := float32(-1)
				ruledOut := map[int64]bool{} // the same rules as the suggestions (score)
				byPerson := map[int64]float32{}
				for _, r := range refs {
					out, seen := ruledOut[r.pid]
					if !seen {
						out = slices.Contains(c.Rejected, r.pid) || r.pi.clashes(c)
						ruledOut[r.pid] = out
					}
					if out {
						continue
					}
					var d float32
					for k, x := range u {
						d += x * r.u[k]
					}
					if p, ok := byPerson[r.pid]; !ok || d > p {
						byPerson[r.pid] = d
					}
				}
				for pid, d := range byPerson {
					if idx[pid].ages.fitsLike(cb, ca, cok, d) {
						best = max(best, d)
					}
				}
				scores[i] = best
			}
		}()
	}
	for i := range cs {
		next <- i
	}
	close(next)
	wg.Wait()
	out := make(map[*cluster]float32, len(cs))
	for i, c := range cs {
		out[c] = scores[i]
	}
	return out
}

func (s *Store) reviewItem(c *cluster, faces []*faceRec, covers map[int64]string, idx personIndex, sample int) ReviewItem {
	it := ReviewItem{GroupView: s.groupView(c, faces, sample)}
	it.Suggestions = s.suggest(c, faces, covers, idx, s.family())
	return it
}

// GroupItem is one group as a review card, whether or not it is in the
// inbox (opened from a face in a photo).
func (s *Store) GroupItem(ctx context.Context, gid int64, sample int) (ReviewItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c := s.clusters[gid]
	if c == nil || c.n == 0 {
		return ReviewItem{}, ErrNotFound
	}
	byGroup := s.facesByGroup()
	return s.reviewItem(c, byGroup[gid], s.personCovers(byGroup), s.personIndex(), sample), nil
}

// personCovers picks a cover face crop per person: the best face of their
// biggest group.
func (s *Store) personCovers(byGroup map[int64][]*faceRec) map[int64]string {
	covers := map[int64]string{}
	sizes := map[int64]int{}
	for id, c := range s.clusters {
		if c.PersonID == 0 || len(byGroup[id]) == 0 || c.n <= sizes[c.PersonID] {
			continue
		}
		sizes[c.PersonID] = c.n
		covers[c.PersonID] = s.faceView(byGroup[id][0]).Crop
	}
	for id, p := range s.people {
		if f := s.pickedCover(p); f != nil {
			covers[id] = s.faceView(f).Crop
		} else if f := s.autoCover(p, byGroup); f != nil {
			covers[id] = s.faceView(f).Crop
		}
	}
	return covers
}

// SkipGroup takes a group out of the inbox until it grows.
func (s *Store) SkipGroup(ctx context.Context, gid int64) error {
	return s.updateGroup(gid, func(c *cluster) { c.SkippedAt = max(c.n, 1) })
}

// GroupSuggestions returns up to three people a group may be.
func (s *Store) GroupSuggestions(ctx context.Context, gid int64) []Suggestion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []Suggestion{}
	c := s.clusters[gid]
	if c == nil {
		return out
	}
	byGroup := s.facesByGroup()
	return s.suggest(c, byGroup[gid], s.personCovers(byGroup), s.personIndex(), s.family())
}

// unnamedClusters lists groups without a person.
func (s *Store) unnamedClusters() []*cluster {
	var out []*cluster
	for _, c := range s.clusters {
		if c.PersonID == 0 {
			out = append(out, c)
		}
	}
	return out
}
