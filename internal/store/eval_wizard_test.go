package store

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// The wizard bench (experiment), on a copy of a real library whose naming
// is the answer key:
//
//	IO_EVAL_DB=<copy> IO_EVAL_DIR=<work dir> go test -run TestEvalWizardPrepare
//	IO_EVAL_DIR=<work dir> go test -run TestEvalWizard$
//
// Prepare saves who is who (truth.json) and a started-over copy
// (base.bolt). Each run starts from that copy, plays the user naming what
// Discover shows (by what most of the shown faces truly are), lets the app
// grow on its own, and measures it against the truth.

type truth struct {
	Face map[int64]string  `json:"face"` // face -> person name (confirmed or tagged)
	Rel  map[string]string `json:"rel"`  // "A|B" -> what B is to A (entered or worked out)
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	in, err := os.Open(from)
	if err != nil {
		t.Fatal(err)
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
	if err := out.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEvalWizardPrepare(t *testing.T) {
	src, dir := os.Getenv("IO_EVAL_DB"), os.Getenv("IO_EVAL_DIR")
	if src == "" || dir == "" {
		t.Skip("set IO_EVAL_DB and IO_EVAL_DIR")
	}
	s, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	tr := truth{Face: map[int64]string{}}
	for _, f := range s.faces {
		if f.NotFace || f.Picture {
			continue
		}
		pid := f.Tagged
		if c := s.clusters[f.ClusterID]; pid == 0 && c != nil && c.PersonID != 0 && !c.Auto {
			pid = c.PersonID
		}
		if p := s.people[pid]; p != nil && !p.Picture {
			tr.Face[f.ID] = p.Name
		}
	}
	tr.Rel = map[string]string{}
	for a, rel := range s.family() {
		for b, k := range rel {
			tr.Rel[s.people[a].Name+"|"+s.people[b].Name] = k
		}
	}
	s.Close()
	data, _ := json.Marshal(tr)
	if err := os.WriteFile(filepath.Join(dir, "truth.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Logf("truth: %d faces named", len(tr.Face))
	base := filepath.Join(dir, "base.bolt")
	copyFile(t, src, base)
	b, err := Open(base)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if err := b.StartOver(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// wizardRun opens a fresh copy of the started-over library and the truth.
func wizardRun(t *testing.T) (*Store, truth) {
	dir := os.Getenv("IO_EVAL_DIR")
	if dir == "" {
		t.Skip("set IO_EVAL_DIR (after TestEvalWizardPrepare)")
	}
	var tr truth
	data, err := os.ReadFile(filepath.Join(dir, "truth.json"))
	if err != nil {
		t.Skip("no truth.json: run TestEvalWizardPrepare")
	}
	if err := json.Unmarshal(data, &tr); err != nil {
		t.Fatal(err)
	}
	run := filepath.Join(dir, "run.bolt")
	copyFile(t, filepath.Join(dir, "base.bolt"), run)
	s, err := Open(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, tr
}

// score compares who the library says each face is with the truth.
type wizardScore struct {
	named, right, truth int // faces given a person; of them right; faces in the truth
	people              int
}

func (s *Store) wizardScore(tr truth) wizardScore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sc wizardScore
	sc.truth = len(tr.Face)
	sc.people = len(s.people)
	for _, f := range s.faces {
		if f.NotFace {
			continue
		}
		pid := f.Tagged
		if c := s.clusters[f.ClusterID]; pid == 0 && c != nil {
			pid = c.PersonID
		}
		p := s.people[pid]
		if p == nil {
			continue
		}
		want, known := tr.Face[f.ID]
		if !known {
			continue // the truth doesn't say: can't judge
		}
		sc.named++
		if want == p.Name {
			sc.right++
		}
	}
	return sc
}

func (sc wizardScore) String() string {
	prec := 100 * float64(sc.right) / float64(max(sc.named, 1))
	rec := 100 * float64(sc.right) / float64(max(sc.truth, 1))
	return time.Now().Format("15:04:05") + "  " + itoa(sc.people) + " people · precision " + ftoa(prec) + "% · recall " + ftoa(rec) + "% (" + itoa(sc.right) + " of " + itoa(sc.truth) + " faces right, " + itoa(sc.named-sc.right) + " wrong)"
}

func ftoa(x float64) string {
	b, _ := json.Marshal(float64(int(x*10)) / 10)
	return string(b)
}

// playDiscover plays the user on Discover: for each person shown, if most
// shown faces are truly one person, name them so; else skip. Rounds until
// nothing new is named, at most rounds. It returns the names given.
func (s *Store) playDiscover(ctx context.Context, tr truth, rounds, perRound int) int {
	given := 0
	skip := map[int64]bool{}
	for range rounds {
		named := 0
		for _, ch := range s.DiscoverPeople(ctx, perRound) {
			if skip[ch.Groups[0]] {
				continue
			}
			votes := map[string]int{}
			labeled := 0
			for _, f := range ch.Faces {
				if n, ok := tr.Face[f.ID]; ok {
					votes[n]++
					labeled++
				}
			}
			best, bn := "", 0
			for n, v := range votes {
				if v > bn {
					best, bn = n, v
				}
			}
			// "Is this also X?": yes when most faces shown are X, else no.
			if ch.Person != 0 {
				if labeled*2 >= len(ch.Faces) && bn*10 >= labeled*7 && best == ch.PersonName {
					for _, g := range ch.Groups {
						s.NameGroup(ctx, g, ch.Person, "")
					}
					evalAlsoYes++
					named++
				} else {
					for _, g := range ch.Groups {
						s.RejectGroup(ctx, g, ch.Person)
					}
					evalAlsoNo++
					named++ // an answer changes the list too
				}
				continue
			}
			// A user tells who it is when most faces shown are one person
			// they know.
			if labeled*2 < len(ch.Faces) || bn*10 < labeled*7 {
				if os.Getenv("IO_EVAL_V") != "" {
					fmt.Printf("skip: %d photos, %d groups, %d of %d shown known: %v\n", ch.Photos, len(ch.Groups), labeled, len(ch.Faces), votes)
				}
				skip[ch.Groups[0]] = true
				continue
			}
			var pid int64
			for _, g := range ch.Groups {
				id, err := s.NameGroup(ctx, g, pid, best)
				if err == nil {
					pid = id
				}
			}
			var shown []int64
			for _, f := range ch.Faces {
				if tr.Face[f.ID] == best {
					shown = append(shown, f.ID)
				}
			}
			s.MarkSeen(ctx, pid, shown)
			named++
			given++
		}
		if named == 0 {
			break
		}
	}
	return given
}

func TestEvalWizard(t *testing.T) {
	s, tr := wizardRun(t)
	ctx := context.Background()
	t.Logf("start:    %s", s.wizardScore(tr))
	st := time.Now()
	given := s.playDiscover(ctx, tr, 10, 60)
	t.Logf("discover: %s · %d names given, %d yes / %d no to \"also …?\", in %v", s.wizardScore(tr), given, evalAlsoYes, evalAlsoNo, time.Since(st).Round(time.Second))
	// Let the app grow on its own: auto-accept near-certain faces for each
	// person, rounds until nothing more comes.
	st = time.Now()
	for round := range 6 {
		n := 0
		for _, pid := range s.LanePeople(ctx, 200) {
			acc, err := s.AutoAccept(ctx, pid)
			if err != nil {
				t.Fatal(err)
			}
			n += len(acc)
		}
		t.Logf("auto %d:   %s · %d accepted", round+1, s.wizardScore(tr), n)
		if n == 0 {
			break
		}
	}
	t.Logf("auto took %v", time.Since(st).Round(time.Second))
	s.missed(t, tr)
	// Relation questions, once the people are named.
	gs := s.GuessRelations(ctx, 30)
	right, related := 0, 0
	for _, g := range gs {
		k, ok := tr.Rel[g.AName+"|"+g.BName]
		if ok {
			related++
			if k == g.Kind {
				right++
			}
		}
		if os.Getenv("IO_EVAL_V") != "" {
			fmt.Printf("ask: is %s %s's %s? (truth: %q)\n", g.BName, g.AName, g.Kind, k)
		}
	}
	t.Logf("relations: top %d questions, %d about real family, %d guessed right", len(gs), related, right)
	// Play the user answering 40 questions, top first: the truth, or
	// "not family".
	ids := map[string]int64{}
	s.mu.RLock()
	for id, p := range s.people {
		ids[p.Name] = id
	}
	s.mu.RUnlock()
	yes, other, none := 0, 0, 0
	for range 40 {
		gs := s.GuessRelations(ctx, 1)
		if len(gs) == 0 {
			break
		}
		g := gs[0]
		k, ok := tr.Rel[g.AName+"|"+g.BName]
		switch {
		case ok && k == g.Kind:
			yes++
		case ok:
			other++
		default:
			none++
		}
		if os.Getenv("IO_EVAL_V") != "" {
			fmt.Printf("answer: is %s %s's %s? %q\n", g.BName, g.AName, g.Kind, k)
		}
		if ok && (k == KinParent || k == KinChild || k == KinSibling || k == KinPartner) {
			if err := s.SetRelation(ctx, g.A, g.B, k, true); err != nil {
				t.Fatal(err)
			}
		} else if err := s.SetAsked(ctx, g.A, g.B, true); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("answering 40: %d yes, %d family another way, %d not family (as far as the truth knows)", yes, other, none)
	// The worst-named people, to see what goes wrong.
	s.mu.RLock()
	wrong := map[string]int{}
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID == 0 {
			continue
		}
		if want, ok := tr.Face[f.ID]; ok && want != s.people[c.PersonID].Name {
			wrong[s.people[c.PersonID].Name+" ← really "+want]++
		}
	}
	s.mu.RUnlock()
	type kv struct {
		k string
		v int
	}
	var ws []kv
	for k, v := range wrong {
		ws = append(ws, kv{k, v})
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].v > ws[j].v })
	for _, w := range ws[:min(len(ws), 8)] {
		t.Logf("  wrong: %s ×%d", w.k, w.v)
	}
}

// missed tells where the faces the library doesn't name yet are: per true
// person, and by the size of the group they sit in.
func (s *Store) missed(t *testing.T, tr truth) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	per := map[string]int{}
	bySize := map[string]int{}
	people := map[string]bool{}
	for _, f := range s.faces {
		want, ok := tr.Face[f.ID]
		if !ok || f.NotFace {
			continue
		}
		people[want] = true
		c := s.clusters[f.ClusterID]
		if c != nil && c.PersonID != 0 || f.Tagged != 0 {
			continue
		}
		n := 0
		if c != nil {
			n = c.n
		}
		per[want+" "+map[bool]string{true: "(5+)", false: "(<5)"}[n >= 5]]++
		switch {
		case n <= 1:
			bySize["alone"]++
		case n < 5:
			bySize["2-4"]++
		case n < 20:
			bySize["5-19"]++
		default:
			bySize["20+"]++
		}
	}
	t.Logf("missed by group size: %v; %d people in the truth", bySize, len(people))
	type kv struct {
		k string
		v int
	}
	var ws []kv
	for k, v := range per {
		ws = append(ws, kv{k, v})
	}
	sort.Slice(ws, func(i, j int) bool { return ws[i].v > ws[j].v })
	for _, w := range ws[:min(len(ws), 12)] {
		t.Logf("  missed: %s ×%d", w.k, w.v)
	}
}

// TestEvalWizardDemo leaves run.bolt as after playing Discover: people
// named, no relations answered (to look at the questions in the UI).
func TestEvalWizardDemo(t *testing.T) {
	s, tr := wizardRun(t)
	s.playDiscover(context.Background(), tr, 10, 60)
}

// TestEvalChainPurity: the first Discover list from scratch, by all faces
// in each chain's groups: how pure each is by the truth.
func TestEvalChainPurity(t *testing.T) {
	s, tr := wizardRun(t)
	chains := s.DiscoverPeople(context.Background(), 200)
	s.mu.RLock()
	defer s.mu.RUnlock()
	in := map[int64]int{}
	for i, ch := range chains {
		for _, g := range ch.Groups {
			in[g] = i
		}
	}
	votes := make([]map[string]int, len(chains))
	for i := range votes {
		votes[i] = map[string]int{}
	}
	for _, f := range s.faces {
		i, ok := in[f.ClusterID]
		if !ok {
			continue
		}
		if n, ok := tr.Face[f.ID]; ok {
			votes[i][n]++
		} else {
			votes[i][""]++
		}
	}
	impure := 0
	for i, ch := range chains {
		best, bn, labeled := "", 0, 0
		for n, v := range votes[i] {
			if n == "" {
				continue
			}
			labeled += v
			if v > bn {
				best, bn = n, v
			}
		}
		p := float64(bn) / float64(max(labeled, 1))
		if labeled > 0 && p < 0.9 {
			impure++
		}
		fmt.Printf("%2d %-20s purity %.2f labeled %4d unlabeled %4d groups %2d days %d alike %.2f still %v\n", i+1, best, p, labeled, votes[i][""], len(ch.Groups), ch.Days, ch.Alike, ch.Still)
	}
	t.Logf("%d of %d chains under 90%% pure", impure, len(chains))
}

var evalAlsoYes, evalAlsoNo int // "Is this also …?" answers in playDiscover

// TestEvalWizardSteps plays a user one row at a time (the list reloads
// after each answer, as in the UI), and reports how far each number of
// answers gets.
func TestEvalWizardSteps(t *testing.T) {
	s, tr := wizardRun(t)
	ctx := context.Background()
	skip := map[int64]bool{}
	var names, yes, no, skips, strangers int
	marks := map[int]bool{10: true, 25: true, 50: true, 100: true, 150: true, 200: true}
	st := time.Now()
	for step := 1; step <= 200; step++ {
		var ch *Chain
		for _, c := range s.DiscoverPeople(ctx, 30) {
			if !skip[c.Groups[0]] {
				ch = &c
				break
			}
		}
		if ch == nil {
			break
		}
		votes := map[string]int{}
		labeled := 0
		for _, f := range ch.Faces {
			if n, ok := tr.Face[f.ID]; ok {
				votes[n]++
				labeled++
			}
		}
		best, bn := "", 0
		for n, v := range votes {
			if v > bn {
				best, bn = n, v
			}
		}
		clear := labeled*2 >= len(ch.Faces) && bn*10 >= labeled*7
		switch {
		case ch.Person != 0 && clear && best == ch.PersonName:
			for _, g := range ch.Groups {
				s.NameGroup(ctx, g, ch.Person, "")
			}
			yes++
		case ch.Person != 0:
			for _, g := range ch.Groups {
				s.RejectGroup(ctx, g, ch.Person)
			}
			no++
		case clear:
			if os.Getenv("IO_EVAL_V") != "" {
				s.whyNotAlso(ch, best)
			}
			var pid int64
			for _, g := range ch.Groups {
				if id, err := s.NameGroup(ctx, g, pid, best); err == nil {
					pid = id
				}
			}
			names++
		case labeled == 0: // no one the user named: a stranger
			for _, g := range ch.Groups {
				s.HideGroup(ctx, g, true)
			}
			strangers++
		default:
			skip[ch.Groups[0]] = true
			skips++
		}
		if marks[step] {
			t.Logf("%3d answers (%d names, %d yes, %d no, %d strangers, %d skips): %s · %v", step, names, yes, no, strangers, skips, s.wizardScore(tr), time.Since(st).Round(time.Second))
		}
	}
}

// whyNotAlso says why a row the user names after someone already named
// wasn't asked as "Is this also …?".
func (s *Store) whyNotAlso(ch *Chain, name string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pi := range s.confirmedIndex() {
		if pi.p.Name != name {
			continue
		}
		var us [][]float32
		for _, g := range pi.groups {
			us = append(us, clusterUnit(g))
		}
		for _, gid := range ch.Groups {
			c := s.clusters[gid]
			sim, _ := bestRef(clusterUnit(c), us)
			cb, ca, cok := c.birth()
			lk, _ := pi.ages.looksAt(float32(c.timeSum / float64(max(c.timeN, 1))))
			shared := 0
			for img := range c.imgs {
				if pi.imgs[img] {
					shared++
				}
			}
			fmt.Printf("  shared %d of %d · %s again: group %d n %d sim %.2f fits %v (born %.1f age %.1f; looks %.1f then) clashes %v rejected %v\n", shared, len(c.imgs), name, gid, c.n, sim, pi.ages.fits(cb, ca, cok), cb, ca, lk, pi.clashes(c), c.Rejected)
		}
		return
	}
}
