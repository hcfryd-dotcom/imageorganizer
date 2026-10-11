package store

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

// TestEvalStartOver plays the user starting over from no names on a copy
// of a library named the old way (grouping version < startOverBelow):
// each round they name the people Discover asks about, accept a batch of
// each known person's likeliest faces (ticking off the wrong ones), and
// faces near certain are filed by themselves. The old names (Was) answer
// for the user, and judge what was filed: "sure" counts only faces the
// user had tagged one by one, "all" also faces named with their group
// (themselves sometimes wrong).
//
//	IO_EVAL_DB=<copy, rewritten> IO_EVAL_ROUNDS=8 IO_EVAL_BATCH=100 [IO_EVAL_WHO=3] go test -run TestEvalStartOver -v -timeout 3h
func TestEvalStartOver(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	rounds, batch, whoPer := 8, 100, 1000
	if v, err := strconv.ParseFloat(os.Getenv("IO_EVAL_FILESIM"), 32); err == nil {
		fileSim, fileKeep = float32(v), float32(v)-0.05 // the bar to file at
		t.Logf("filing at %.2f, keeping at %.2f", fileSim, fileKeep)
	}
	if v, err := strconv.ParseFloat(os.Getenv("IO_EVAL_RIVALSELF"), 32); err == nil {
		rivalSelf = float32(v)
	}
	if v, err := strconv.ParseFloat(os.Getenv("IO_EVAL_SHOT"), 32); err == nil {
		shotBonus = float32(v)
		t.Logf("near-identical shots count %.2f", shotBonus)
	}
	if v, err := strconv.ParseFloat(os.Getenv("IO_EVAL_AGEUNKNOWN"), 32); err == nil {
		ageUnknownSim = float32(v)
	}
	t.Logf("rivals like them from %.2f are them; unread ages filed from %.2f", rivalSelf, ageUnknownSim)
	if n, err := strconv.Atoi(os.Getenv("IO_EVAL_WHO")); err == nil {
		whoPer = n // people named per round: few, as when batches come first
	}
	if n, err := strconv.Atoi(os.Getenv("IO_EVAL_ROUNDS")); err == nil {
		rounds = n
	}
	if n, err := strconv.Atoi(os.Getenv("IO_EVAL_BATCH")); err == nil {
		batch = n
	}
	// The faces tagged one by one, before starting over.
	sure := map[int64]bool{}
	db, err := bolt.Open(path, 0o600, &bolt.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketFaces).ForEach(func(_, v []byte) error {
			var f struct {
				ID  int64 `json:"id"`
				Tag int64 `json:"tag"`
			}
			if json.Unmarshal(v, &f) == nil && f.Tag != 0 {
				sure[f.ID] = true
			}
			return nil
		})
	})
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	truth := map[int64]int64{}
	s.mu.RLock()
	known := 0
	for id, f := range s.faces {
		if f.Tagged != 0 {
			t.Fatalf("face %d still named after starting over", id)
		}
		truth[id] = f.Was
		if f.Was != 0 {
			known++
		}
	}
	s.mu.RUnlock()
	t.Logf("faces with an old name: %d (tagged one by one: %d)", known, len(sure))

	clicks, shown := 0, 0
	named := map[int64]bool{}
	for round := 1; round <= rounds; round++ {
		start := time.Now()
		// Who is this? The people seen on many days.
		asked := 0
		for _, ch := range s.DiscoverPeople(ctx, 60) {
			if ch.Days < chainWorth || ch.Person != 0 { // someone known: their batches cover it
				continue
			}
			if asked++; asked > whoPer {
				break
			}
			votes := map[int64]int{}
			for _, f := range ch.Faces {
				votes[truth[f.ID]]++
			}
			var who int64
			for pid, n := range votes {
				if n > votes[who] || who == 0 && n == votes[who] && pid != 0 {
					who = pid
				}
			}
			clicks++
			shown += len(ch.Faces)
			if os.Getenv("IO_EVAL_SHOW") != "" {
				t.Logf("  who: %d photos on %d days, %d groups, asks %q → old name %d (new %v), votes %v", ch.Photos, ch.Days, len(ch.Groups), ch.PersonName, who, !named[who], votes)
			}
			if who == 0 {
				for _, g := range ch.Groups {
					s.HideGroup(ctx, g, true)
				}
				continue
			}
			var yes, no []int64
			for _, f := range ch.Faces {
				if truth[f.ID] == who {
					yes = append(yes, f.ID)
				} else {
					no = append(no, f.ID)
				}
			}
			if _, err := s.ConfirmFaces(ctx, who, "", yes, no); err != nil {
				t.Fatal(err)
			}
			named[who] = true
		}
		// More of them: a batch each.
		for pid := range named {
			fs := s.FaceCandidates(ctx, pid, batch)
			if len(fs) == 0 {
				continue
			}
			var yes, no []int64
			for _, f := range fs {
				if truth[f.ID] == pid {
					yes = append(yes, f.ID)
				} else {
					no = append(no, f.ID)
				}
			}
			clicks++
			shown += len(fs)
			if _, err := s.ConfirmFaces(ctx, pid, "", yes, no); err != nil {
				t.Fatal(err)
			}
		}
		// Then what is near certain files itself, learning as it goes.
		filed, unfiled := 0, 0
		for pass := 0; pass < 6; pass++ {
			a, b, err := s.FileFaces(ctx)
			if err != nil {
				t.Fatal(err)
			}
			filed, unfiled = filed+a, unfiled+b
			if a == 0 {
				break
			}
		}
		// Tally.
		s.mu.RLock()
		var conf, confWrong, auto, autoWrong, autoUnknown, sureAuto, sureWrong, left int
		for id, f := range s.faces {
			c := s.clusters[f.ClusterID]
			pid := int64(0)
			if c != nil {
				pid = c.PersonID
			}
			switch {
			case f.Tagged != 0:
				conf++
				if truth[id] != f.Tagged {
					confWrong++
				}
			case pid != 0:
				auto++
				switch truth[id] {
				case pid:
				case 0:
					autoUnknown++
				default:
					autoWrong++
				}
				if sure[id] {
					sureAuto++
					if truth[id] != pid {
						sureWrong++
					}
				}
			case truth[id] != 0 && named[truth[id]]:
				left++
			}
		}
		s.mu.RUnlock()
		t.Logf("round %d: people %d · clicks %d (faces shown %d) · confirmed %d (wrong %d) · filed %d (+%d −%d this round): wrong %d (%.2f%%), no old name %d; tagged-only: %d wrong of %d · still unnamed of known people %d · %s",
			round, len(named), clicks, shown, conf, confWrong, auto, filed, unfiled, autoWrong, 100*float64(autoWrong)/float64(max(auto, 1)),
			autoUnknown, sureWrong, sureAuto, left, time.Since(start).Round(time.Second))
	}
	// The queue as Discover shows it now.
	start := time.Now()
	q := s.DiscoverQueue(ctx)
	kinds := map[string]int{}
	for _, it := range q {
		kinds[it.Kind]++
	}
	t.Logf("queue: %d questions %v, built in %s", len(q), kinds, time.Since(start).Round(time.Millisecond))
	for i, it := range q[:min(len(q), 20)] {
		t.Logf("  %2d. %-8s worth %6.1f", i+1, it.Kind, it.Value)
	}
	start = time.Now()
	a, b, _ := s.FileFaces(ctx)
	t.Logf("a pass of filing with nothing new: +%d −%d in %s", a, b, time.Since(start).Round(time.Millisecond))
}
