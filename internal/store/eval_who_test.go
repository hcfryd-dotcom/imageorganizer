package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

// TestEvalWho counts where people not named yet drop out on the way to
// "Who is this?": the chain limit, look-alikes of someone named, too few
// days, too few clear faces.
//
//	IO_EVAL_DB=<copy> go test -run TestEvalWho -v
func TestEvalWho(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	all := s.discoverChains()
	var want []*cluster
	for _, sc := range all {
		want = append(want, sc.groups...)
	}
	faces := facesOfGroups(s, want, func(c *cluster) int64 { return c.id })
	var seeded, few, unclear, ok, okPast100, withKnown, withKnownCut int
	for i, sc := range all {
		_, _, top := s.chainCompany(sc.groups)
		clear := s.chainClear(sc.groups, faces)
		switch {
		case sc.who != nil:
			seeded++
		case len(sc.days) < chainWorth:
			few++
		case clear < queueClearMin:
			unclear++
			if top >= 3 {
				withKnownCut++
			}
		default:
			ok++
			if i >= 100 {
				okPast100++
			}
			if top >= 3 {
				withKnown++
			}
		}
		if i < 40 || (sc.who == nil && top >= 5 && i >= 100) {
			t.Logf("#%d who=%v groups=%d photos=%d days=%d clear=%d topDays=%d", i, sc.who != nil, len(sc.groups), sc.n, len(sc.days), clear, top)
		}
	}
	t.Logf("chains %d: seeded(look like someone named) %d, <%d days %d, <%d clear %d (of which ≥3 days with one known: %d), asked-worthy %d (past #100: %d; ≥3 days with one known: %d)",
		len(all), seeded, chainWorth, few, queueClearMin, unclear, withKnownCut, ok, okPast100, withKnown)
}

// TestEvalWhoSeeded: the groups like someone named, which "Who is this?"
// leaves to that person's "More of" batch: are their faces in it?
func TestEvalWhoSeeded(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.newMatcher()
	cands := map[int64]map[int64]bool{}
	for i, sc := range s.discoverChains() {
		if sc.who == nil || i > 30 {
			continue
		}
		pid := sc.who.ID
		if cands[pid] == nil {
			cands[pid] = map[int64]bool{}
			for _, f := range s.faceCandidates(m, pid, queueBatch) {
				cands[pid][f.ID] = true
			}
		}
		g := sc.groups[0]
		var n, inBatch, best, sure int
		var like, score float32
		holds := map[string]int{}
		for _, f := range s.faces {
			if f.ClusterID != g.id || len(f.emb) == 0 {
				continue
			}
			n++
			if cands[pid][f.ID] {
				inBatch++
			}
			likes := m.likes(f)
			if m.best(f, likes) == pid {
				best++
			}
			mt := m.match(f, pid, likes)
			like += mt.Like
			score += mt.Score
			if mt.Sure {
				sure++
			}
			holds[mt.Hold]++
		}
		top, topN := "", 0
		for h, k := range holds {
			if k > topN {
				top, topN = h, k
			}
		}
		t.Logf("#%d person %d group %d faces %d: best-is-them %d, sure %d, in batch %d, mean like %.2f score %.2f; hold: %q ×%d",
			i, pid, g.id, n, best, sure, inBatch, like/float32(max(n, 1)), score/float32(max(n, 1)), top, topN)
	}
}

// TestEvalWhoQueue shows the Discover queue's kinds and its first "Who
// is this?" cards.
func TestEvalWhoQueue(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	t0 := time.Now()
	q := s.DiscoverQueue(context.Background())
	kinds := map[string]int{}
	more := 0
	for _, it := range q {
		kinds[it.Kind]++
		if it.More != nil {
			more += len(it.More.Faces)
		}
	}
	t.Logf("queue %d in %v: %v; faces in batches %d", len(q), time.Since(t0).Round(time.Millisecond), kinds, more)
	for i, it := range q {
		if it.More == nil || it.More.Confirmed >= queueFew {
			continue
		}
		extra := ""
		if it.More != nil {
			extra = fmt.Sprintf("confirmed %d faces %d", it.More.Confirmed, len(it.More.Faces))
		}
		t.Logf("#%d %s %.0f %s", i, it.Kind, it.Value, extra)
	}
	n := 0
	for _, it := range q {
		if it.Who == nil || n >= 25 {
			continue
		}
		n++
		w := it.Who
		t.Logf("value %.0f photos %d days %d clear %d topDays %d maybe %d looksLike=%v", it.Value, w.Photos, w.Days, w.Clear, w.TopDays, w.Maybe, w.LooksLike != "")
	}
}
