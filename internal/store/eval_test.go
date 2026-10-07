package store

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestEvalGrouping measures grouping rules against a real library: the
// faces in groups the user named are the truth. It needs a database copy
// (curl localhost:8420/debug/db > copy.bolt) and runs with
//
//	IO_EVAL_DB=copy.bolt go test -run EvalGrouping -v -timeout 1h ./internal/store
//
// For each variant it reports how often different people end up in one
// group (mixing), how many groups each person is split into (clicks to
// name them), and, pretending each named group were unnamed, how often its
// best suggestion is the right person at each confidence.
func TestEvalGrouping(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	truth := map[int64]int64{} // face id -> person
	var faces []*faceRec
	for _, f := range s.faces {
		if f.AgeVersion == 0 || f.NotFace {
			continue
		}
		if c := s.clusters[f.ClusterID]; c != nil && c.Hidden {
			continue // strangers, pets: not part of the test
		}
		faces = append(faces, f)
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID != 0 {
			truth[f.ID] = c.PersonID
		}
	}
	t.Logf("%d faces, %d named, %d people", len(faces), len(truth), len(s.people))

	only := os.Getenv("IO_EVAL_ONLY")
	for _, r := range evalVariants() {
		if only != "" && !strings.Contains(r.Name, only) {
			continue
		}
		start := time.Now()
		g := newGrouper(s, r)
		g.run(slices.Clone(faces))
		res := evaluate(s, g, truth)
		t.Logf("%-28s %s (%s)", r.Name, res, time.Since(start).Round(time.Second))
		for _, line := range res.details {
			t.Logf("    %s", line)
		}
	}
}

func evalVariants() []groupingRules {
	base := currentRules
	v := func(name string, change func(*groupingRules)) groupingRules {
		r := base
		r.Name = name
		change(&r)
		return r
	}
	return []groupingRules{
		base,
		v("one-per-photo", func(r *groupingRules) { r.OnePerPhoto = true }),
		v("join .45", func(r *groupingRules) { r.Join = 0.45 }),
		v("join .45 +1pp", func(r *groupingRules) { r.Join = 0.45; r.OnePerPhoto = true }),
		v("join .40 +1pp", func(r *groupingRules) { r.Join = 0.40; r.OnePerPhoto = true }),
		v("events 10m .35 +1pp", func(r *groupingRules) {
			r.OnePerPhoto = true
			r.EventGap, r.EventJoin = 10*time.Minute, 0.35
		}),
		v("events 1h .35 +1pp", func(r *groupingRules) {
			r.OnePerPhoto = true
			r.EventGap, r.EventJoin = time.Hour, 0.35
		}),
		v("events 1h .30 +1pp", func(r *groupingRules) {
			r.OnePerPhoto = true
			r.EventGap, r.EventJoin = time.Hour, 0.30
		}),
		v("time x2 +1pp", func(r *groupingRules) { r.OnePerPhoto = true; r.TimeBase, r.TimeSlope = 2, 0.3 }),
	}
}

type evalResult struct {
	groups, groups2 int     // all groups; groups of 2+ faces
	mixed           float64 // named faces in a group mostly of someone else
	clicks          int     // groups holding named faces: one decision each
	clicks90        int     // groups needed to cover 90% of each person
	looN            int     // named groups (2+ faces) tested blind
	loo             []looPoint
	details         []string
}

type looPoint struct {
	score   float32 // best match among all named people (own name hidden)
	correct bool
	other   float32 // best match among everyone else: as if a stranger
	faces   int
}

func (r evalResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "groups %6d (2+: %5d)  mixed %.2f%%  clicks %5d  clicks90 %5d", r.groups, r.groups2, 100*r.mixed, r.clicks, r.clicks90)
	return b.String()
}

func evaluate(s *Store, g *grouper, truth map[int64]int64) evalResult {
	var res evalResult
	type tally struct {
		c      *cluster
		counts map[int64]int
		named  int
	}
	tallies := map[*cluster]*tally{}
	for fid, c := range g.of {
		tl := tallies[c]
		if tl == nil {
			tl = &tally{c: c, counts: map[int64]int{}}
			tallies[c] = tl
		}
		if p := truth[fid]; p != 0 {
			tl.counts[p]++
			tl.named++
		}
	}
	res.groups = len(g.list)
	for _, c := range g.list {
		if c.n >= 2 {
			res.groups2++
		}
	}

	// Mixing, and which people get mixed up.
	label := map[*cluster]int64{}
	confused := map[[2]int64]int{}
	perPerson := map[int64][]int{} // group sizes (their faces) per person
	mixed, named := 0, 0
	for c, tl := range tallies {
		if tl.named == 0 {
			continue
		}
		var top int64
		for p, n := range tl.counts {
			if top == 0 || n > tl.counts[top] || (n == tl.counts[top] && p < top) {
				top = p
			}
		}
		label[c] = top
		named += tl.named
		for p, n := range tl.counts {
			perPerson[p] = append(perPerson[p], n)
			if p != top {
				mixed += n
				confused[[2]int64{top, p}] += n
			}
		}
	}
	res.mixed = float64(mixed) / float64(max(1, named))
	for _, sizes := range perPerson {
		res.clicks += len(sizes)
		slices.SortFunc(sizes, func(a, b int) int { return b - a })
		total := 0
		for _, n := range sizes {
			total += n
		}
		covered := 0
		for i, n := range sizes {
			covered += n
			if covered*10 >= total*9 {
				res.clicks90 += i + 1
				break
			}
		}
	}
	type pair struct {
		k [2]int64
		n int
	}
	var pairs []pair
	for k, n := range confused {
		pairs = append(pairs, pair{k, n})
	}
	slices.SortFunc(pairs, func(a, b pair) int { return cmp.Or(b.n-a.n, cmp.Compare(a.k[0], b.k[0]), cmp.Compare(a.k[1], b.k[1])) })
	var conf []string
	for _, p := range pairs[:min(5, len(pairs))] {
		conf = append(conf, fmt.Sprintf("%s←%s %d", s.personName(p.k[0]), s.personName(p.k[1]), p.n))
	}
	if len(conf) > 0 {
		res.details = append(res.details, "mixed into: "+strings.Join(conf, ", "))
	}

	// Blind test: for each named group of 2+ faces, suggest the person
	// from the other named groups (best centroid similarity among groups
	// whose birth year fits), as the review suggestions do.
	var tested []*cluster
	for c := range label {
		if c.n >= 2 {
			tested = append(tested, c)
		}
	}
	slices.SortFunc(tested, func(a, b *cluster) int { return cmp.Compare(a.id, b.id) })
	points := make([]looPoint, len(tested))
	var wg sync.WaitGroup
	next := make(chan int)
	for range runtime.GOMAXPROCS(0) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				c := tested[i]
				cb, ca, cok := c.birth()
				best := map[int64]float32{}
				for o, p := range label {
					if o == c {
						continue
					}
					ob, oa, ook := o.birth()
					if !birthCompatible(cb, ca, cok, ob, oa, ook) {
						continue
					}
					if sim := groupSim(c, o); sim > best[p] || best[p] == 0 {
						best[p] = sim
					}
				}
				var pick int64
				for p, sim := range best {
					if pick == 0 || sim > best[pick] || (sim == best[pick] && p < pick) {
						pick = p
					}
				}
				other := float32(-1)
				for p, sim := range best {
					if p != label[c] {
						other = max(other, sim)
					}
				}
				points[i] = looPoint{score: best[pick], correct: pick == label[c], other: other, faces: tallies[c].counts[label[c]]}
			}
		}()
	}
	for i := range tested {
		next <- i
	}
	close(next)
	wg.Wait()
	res.looN = len(points)
	res.loo = points
	var cols []string
	for _, th := range []float32{0.30, 0.40, 0.50, 0.60, 0.70} {
		n, ok, nf, okf, all, stranger := 0, 0, 0, 0, 0, 0
		for _, p := range points {
			all += p.faces
			if p.other >= th {
				stranger++
			}
			if p.score >= th {
				n++
				nf += p.faces
				if p.correct {
					ok++
					okf += p.faces
				}
			}
		}
		cols = append(cols, fmt.Sprintf("≥%.2f: %5.1f%% right, covers %5.1f%% groups / %5.1f%% faces · as a stranger, %5.1f%% would be wrongly named",
			th, 100*float64(ok)/float64(max(1, n)), 100*float64(n)/float64(max(1, len(points))), 100*float64(nf)/float64(max(1, all)),
			100*float64(stranger)/float64(max(1, len(points)))))
	}
	right := 0
	for _, p := range points {
		if p.correct {
			right++
		}
	}
	res.details = append(res.details, fmt.Sprintf("blind test on %d named groups: top suggestion right %.1f%%", len(points), 100*float64(right)/float64(max(1, len(points)))))
	for _, c := range cols {
		res.details = append(res.details, "  "+c)
	}
	return res
}

func (s *Store) personName(id int64) string {
	if p := s.people[id]; p != nil {
		return p.Name
	}
	return fmt.Sprint(id)
}

// TestEvalFaceSize shows how face size separates the people the user
// names from background faces: per threshold, how many named faces would
// be dropped and how many unnamed ones (and review groups) would go.
func TestEvalFaceSize(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// px is the face width in pixels of the original photo; rel is its
	// width relative to the largest face in the same photo.
	px := func(f *faceRec) float32 {
		im := s.images[f.ImageID]
		return (f.Box[2] - f.Box[0]) * float32(max(im.Width, im.Height)) // box is relative to the upright photo
	}
	largest := map[int64]float32{}
	for _, f := range s.faces {
		largest[f.ImageID] = max(largest[f.ImageID], f.Box[2]-f.Box[0])
	}
	rel := func(f *faceRec) float32 { return (f.Box[2] - f.Box[0]) / largest[f.ImageID] }
	// dom is how much the largest face stands out: largest / median width.
	widths := map[int64][]float32{}
	for _, f := range s.faces {
		widths[f.ImageID] = append(widths[f.ImageID], f.Box[2]-f.Box[0])
	}
	dom := map[int64]float32{}
	for id, ws := range widths {
		slices.Sort(ws)
		dom[id] = ws[len(ws)-1] / ws[len(ws)/2]
	}
	crowd := func(f *faceRec) int { return len(s.facesByImage[f.ImageID]) }

	type face struct {
		dom     float32
		px, rel float32
		crowd   int
		named   bool
		group   int64
	}
	var all []face
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if f.NotFace || (c != nil && c.Hidden) {
			continue
		}
		all = append(all, face{dom[f.ImageID], px(f), rel(f), crowd(f), c != nil && c.PersonID != 0, f.ClusterID})
	}
	review := func(drop func(face) bool) (named, unnamed, groups int) {
		left := map[int64]int{}
		for _, f := range all {
			if drop(f) {
				if f.named {
					named++
				} else {
					unnamed++
				}
				continue
			}
			if !f.named && f.group != 0 {
				left[f.group]++
			}
		}
		for _, n := range left {
			if n >= 2 {
				groups++
			}
		}
		return
	}
	nNamed, nUnnamed := 0, 0
	for _, f := range all {
		if f.named {
			nNamed++
		} else {
			nUnnamed++
		}
	}
	_, _, g0 := review(func(face) bool { return false })
	t.Logf("%d named, %d unnamed faces; %d unnamed groups of 2+", nNamed, nUnnamed, g0)
	try := func(name string, drop func(face) bool) {
		n, u, g := review(drop)
		t.Logf("%-40s drops %5.1f%% of named, %5.1f%% of unnamed faces; unnamed groups of 2+: %d", name,
			100*float64(n)/float64(nNamed), 100*float64(u)/float64(nUnnamed), g)
	}
	for _, p := range []float32{40, 60, 80, 100} {
		try(fmt.Sprintf("width < %.0fpx", p), func(f face) bool { return f.px < p })
	}
	for _, r := range []float32{0.3, 0.4, 0.5} {
		try(fmt.Sprintf("< %.0f%% of largest face", 100*r), func(f face) bool { return f.rel < r })
		try(fmt.Sprintf("< %.0f%% of largest, in photos of 4+ faces", 100*r), func(f face) bool { return f.rel < r && f.crowd >= 4 })
	}
	for _, n := range []int{6, 10, 20} {
		for _, p := range []float32{80, 100, 150} {
			try(fmt.Sprintf("crowd of %d+ faces & < %.0fpx", n, p), func(f face) bool { return f.crowd >= n && f.px < p })
		}
	}
	try("current rule", func(f face) bool { return f.px < 40 || (f.rel < 0.4 && f.px < 100) })
	try("current rule + crowd 10+ & < 100px", func(f face) bool { return f.px < 40 || (f.rel < 0.4 && f.px < 100) || (f.crowd >= 10 && f.px < 100) })
	cur := func(f face) bool {
		return f.px < 40 || (f.px < 100 && (f.crowd >= 10 || f.rel < 0.4)) || (f.crowd >= 4 && f.rel < 0.4)
	}
	try("live rule (with spectators)", cur)
	for _, n := range []int{4, 6, 8} {
		for _, d := range []float32{1.6, 1.8, 2.0} {
			for _, r := range []float32{0.6, 0.7} {
				try(fmt.Sprintf("live + audience: %d+ faces, top %.1fx median, < %.0f%%", n, d, 100*r), func(f face) bool {
					return cur(f) || (f.crowd >= n && f.dom >= d && f.rel < r)
				})
			}
		}
	}
	try("live rule", cur)
	for _, r := range []float32{0.35, 0.4, 0.5} {
		for _, p := range []float32{150, 200, 300} {
			try(fmt.Sprintf("live + (< %.0f%% of largest & < %.0fpx)", 100*r, p), func(f face) bool { return cur(f) || (f.rel < r && f.px < p) })
		}
		try(fmt.Sprintf("live + (< %.0f%% of largest, 4+ faces)", 100*r), func(f face) bool { return cur(f) || (f.rel < r && f.crowd >= 4) })
	}
	try("current rule + crowd 6+ & < 100px", func(f face) bool { return f.px < 40 || (f.rel < 0.4 && f.px < 100) || (f.crowd >= 6 && f.px < 100) })
	try("< 40% of largest & < 100px", func(f face) bool { return f.rel < 0.4 && f.px < 100 })
	try("(< 40% of largest & < 100px) or < 40px", func(f face) bool { return (f.rel < 0.4 && f.px < 100) || f.px < 40 })
}

// TestEvalRegroup rehearses a regroup with the current rules on a database
// copy and reports, per person, photos before and after (confirmed, and
// filed automatically), so a regroup can be checked before it runs on the
// real library. The copy is modified.
func TestEvalRegroup(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy (it is modified)")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	count := func() (confirmed, auto map[int64]int, review, junk int) {
		confirmed, auto = map[int64]int{}, map[int64]int{}
		seen := map[[2]int64]bool{}
		for _, f := range s.faces {
			if s.isJunk(f) {
				junk++
			}
			c := s.clusters[f.ClusterID]
			if c == nil || c.PersonID == 0 || seen[[2]int64{c.PersonID, f.ImageID}] {
				continue
			}
			seen[[2]int64{c.PersonID, f.ImageID}] = true
			if c.Auto {
				auto[c.PersonID]++
			} else {
				confirmed[c.PersonID]++
			}
		}
		return confirmed, auto, s.ReviewCount(2), junk
	}
	c0, a0, r0, junk := count()
	start := time.Now()
	if err := s.db.Update(s.regroup); err != nil {
		t.Fatal(err)
	}
	c1, a1, r1, _ := count()
	t.Logf("regroup took %s; junk detections %d; review groups %d -> %d", time.Since(start).Round(time.Second), junk, r0, r1)
	t.Logf("%-24s %9s %9s   %9s %9s", "person", "confirmed", "after", "auto", "after")
	var ids []int64
	for id := range s.people {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b int64) int { return c0[b] + a0[b] - c0[a] - a0[a] })
	for _, id := range ids {
		flag := ""
		if c1[id] < c0[id]*95/100 {
			flag = "  <-- lost confirmed photos"
		}
		t.Logf("%-24s %9d %9d   %9d %9d%s", s.personName(id), c0[id], c1[id], a0[id], a1[id], flag)
	}
}

// TestEvalRegroupLoss explains where a person's confirmed faces go in a
// regroup: for each new group holding some of them that doesn't end up as
// the person, what the group's faces were before. IO_EVAL_PERSON names
// the person. The copy is modified.
func TestEvalRegroupLoss(t *testing.T) {
	path, who := os.Getenv("IO_EVAL_DB"), os.Getenv("IO_EVAL_PERSON")
	if path == "" || who == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_PERSON")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var pid int64
	for id, p := range s.people {
		if strings.EqualFold(p.Name, who) {
			pid = id
		}
	}
	type was struct {
		person int64
		auto   bool
		junk   bool
	}
	before := map[int64]was{}
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil {
			before[f.ID] = was{c.PersonID, c.Auto, s.isJunk(f)}
		}
	}
	if err := s.db.Update(s.regroup); err != nil {
		t.Fatal(err)
	}
	type comp struct {
		id                                   int64
		mine, mineAuto, other, none, n, junk int
		person                               int64
		others                               map[int64]int
	}
	groups := map[int64]*comp{}
	lostJunk := 0
	for _, f := range s.faces {
		b := before[f.ID]
		if b.person == pid && !b.auto && f.ClusterID == 0 {
			lostJunk++
		}
		c := s.clusters[f.ClusterID]
		if c == nil {
			continue
		}
		g := groups[c.id]
		if g == nil {
			g = &comp{id: c.id, person: c.PersonID, others: map[int64]int{}}
			groups[c.id] = g
		}
		g.n++
		switch {
		case b.person == pid && !b.auto:
			g.mine++
		case b.person == pid:
			g.mineAuto++
		case b.person != 0 && !b.auto:
			g.other++
			g.others[b.person]++
		default:
			g.none++
		}
	}
	t.Logf("%s's confirmed faces now ungrouped as junk: %d", who, lostJunk)
	var lost []*comp
	for _, g := range groups {
		if g.mine > 0 && g.person != pid {
			lost = append(lost, g)
		}
	}
	slices.SortFunc(lost, func(a, b *comp) int { return b.mine - a.mine })
	total := 0
	for _, g := range lost {
		total += g.mine
	}
	t.Logf("%d of %s's confirmed faces in %d groups that aren't %s now", total, who, len(lost), who)
	for _, g := range lost[:min(12, len(lost))] {
		t.Logf("group %d (now %s): %d faces: %d confirmed %s, %d auto %s, %d other confirmed %v, %d unnamed",
			g.id, s.personName(g.person), g.n, g.mine, who, g.mineAuto, who, g.other, g.others, g.none)
	}
}

// TestEvalJunkSample lists crops of a person's confirmed faces that the
// junk test flags (IO_EVAL_PERSON), with their junk similarity.
func TestEvalJunkSample(t *testing.T) {
	path, who := os.Getenv("IO_EVAL_DB"), os.Getenv("IO_EVAL_PERSON")
	if path == "" || who == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_PERSON")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, flagged := 0, 0
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if c == nil || c.Auto || s.personName(c.PersonID) != who {
			continue
		}
		n++
		if s.isJunk(f) {
			flagged++
			if im := s.images[f.ImageID]; im != nil && flagged <= 60 {
				fmt.Printf("CROP %s/%s_%d.jpg %.2f\n", im.Hash[:2], im.Hash, f.Index, f.Score)
			}
		}
	}
	t.Logf("%s: %d confirmed faces, %d flagged as junk", who, n, flagged)
}

// TestEvalAutoAssignTime times the auto-filing pass that runs after a
// group is named.
func TestEvalAutoAssignTime(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := time.Now()
	cs := s.unnamedClusters()
	idx := s.confirmedIndex()
	t.Logf("index: %s, %d unnamed groups", time.Since(start), len(cs))
	refs := 0
	for _, pi := range idx {
		refs += len(pi.groups)
	}
	start = time.Now()
	n := 0
	for _, c := range cs {
		if pid, score := idx.best(c); pid != 0 && score >= autoAssign {
			n++
		}
	}
	t.Logf("best over %d confirmed groups: %s (%d would be filed)", refs, time.Since(start), n)
	for _, who := range strings.Split(os.Getenv("IO_EVAL_PEOPLE"), ",") { // names to time, e.g. "Anna,Ben"
		for id, p := range s.people {
			if p.Name == who {
				start = time.Now()
				s.autoAssignPerson(id) // changes the copy's memory only
				t.Logf("autoAssignPerson(%s): %s", who, time.Since(start))
			}
		}
	}
}

// TestEvalCover shows how a person's automatic picture is chosen: the top
// candidates with their scores (IO_EVAL_PERSON).
func TestEvalCover(t *testing.T) {
	path, who := os.Getenv("IO_EVAL_DB"), os.Getenv("IO_EVAL_PERSON")
	if path == "" || who == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_PERSON")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, p := range s.people {
		if p.Name != who {
			continue
		}
		byGroup := s.facesByGroup()
		pick := s.autoCover(p, byGroup)
		var fs []*faceRec
		for id, c := range s.clusters {
			if c.PersonID == p.ID {
				fs = append(fs, byGroup[id]...)
			}
		}
		slices.SortFunc(fs, func(a, b *faceRec) int { return cmp.Compare(s.frontal(b), s.frontal(a)) })
		show := func(f *faceRec, tag string) {
			im := s.images[f.ImageID]
			t, _ := s.captureYear(f)
			fmt.Printf("CROP %s/%s_%d.jpg %s frontal %.2f px %.0f score %.2f year %.1f bg %v auto %v\n", im.Hash[:2], im.Hash, f.Index, tag,
				s.frontal(f), s.facePx(f), f.Score, t, f.bg, s.clusters[f.ClusterID].Auto)
		}
		if pick != nil {
			show(pick, "PICK")
		}
		for _, f := range fs[:min(11, len(fs))] {
			show(f, "most-frontal")
		}
		t.Logf("%s: %d faces", who, len(fs))
	}
}

// TestEvalConfirmCascade confirms automatically filed groups one at a
// time, as the user would in Review, and counts what each confirmation
// files in turn. The copy is modified.
func TestEvalConfirmCascade(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	var autos []*cluster
	for _, c := range s.clusters {
		if c.Auto && c.PersonID != 0 && c.inReview(2) {
			autos = append(autos, c)
		}
	}
	slices.SortFunc(autos, func(a, b *cluster) int { return cmp.Or(b.n-a.n, cmp.Compare(a.id, b.id)) })
	t.Logf("%d auto-filed groups waiting in review", len(autos))
	total := 0
	for i, c := range autos[:min(20, len(autos))] {
		before := 0
		for _, o := range s.clusters {
			if o.Auto {
				before++
			}
		}
		if _, err := s.NameGroup(ctx, c.id, c.PersonID, ""); err != nil {
			t.Fatal(err)
		}
		after := 0
		for _, o := range s.clusters {
			if o.Auto {
				after++
			}
		}
		added := after - (before - 1)
		total += added
		t.Logf("confirm #%d (%s, %d faces): %d newly filed", i+1, s.personName(c.PersonID), c.n, added)
	}
	t.Logf("20 confirmations filed %d more groups", total)
}

// TestEvalStrictClash counts automatically filed groups that share any
// photo with their person's confirmed faces (would be un-filed by a strict
// rule), per person.
func TestEvalStrictClash(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	idx := s.confirmedIndex()
	autos, strict, faces := 0, map[string]int{}, 0
	for _, c := range s.clusters {
		if !c.Auto || c.PersonID == 0 {
			continue
		}
		autos++
		pi := idx[c.PersonID]
		if pi == nil {
			continue
		}
		for img := range c.imgs {
			if pi.imgs[img] {
				strict[s.personName(c.PersonID)]++
				faces += c.n
				break
			}
		}
	}
	n := 0
	for _, v := range strict {
		n += v
	}
	t.Logf("%d auto-filed groups; %d share a photo with their person (%d faces): %v", autos, n, faces, strict)
}
