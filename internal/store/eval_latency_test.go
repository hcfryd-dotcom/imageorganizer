package store

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"
)

// TestEvalLatency measures how long a write waits for the library while
// the background learning runs (filing passes, then the Discover queue).
//
//	IO_EVAL_DB=<copy> go test -run TestEvalLatency -v
func TestEvalLatency(t *testing.T) {
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
	probe := func(name string, work func()) {
		done := make(chan struct{})
		var waits []time.Duration
		go func() {
			for {
				select {
				case <-done:
					return
				case <-time.After(50 * time.Millisecond):
				}
				t0 := time.Now()
				s.mu.Lock()
				s.mu.Unlock()
				waits = append(waits, time.Since(t0))
			}
		}()
		t0 := time.Now()
		work()
		close(done)
		time.Sleep(100 * time.Millisecond)
		slices.Sort(waits)
		if len(waits) == 0 {
			waits = []time.Duration{0}
		}
		t.Logf("%-12s took %8v  writer waits: n=%d median %v p90 %v max %v", name, time.Since(t0).Round(time.Millisecond),
			len(waits), waits[len(waits)/2].Round(time.Millisecond), waits[len(waits)*9/10].Round(time.Millisecond), waits[len(waits)-1].Round(time.Millisecond))
	}
	probe("file", func() { s.FileFaces(ctx) })
	probe("people", func() { s.DiscoverPeople(ctx, 100) })
	probe("relations", func() { s.GuessRelations(ctx, 0) })
	probe("doubtfaces", func() { s.DoubtfulFaces(ctx, 0) })
	probe("sessions", func() { s.DateSessions(ctx) })
	probe("dates", func() { s.DoubtfulDates(ctx, 0) })
	probe("queue", func() { s.DiscoverQueue(ctx) })
}

func TestEvalLatencyParts(t *testing.T) {
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
	for i := 0; i < 2; i++ {
		t0 := time.Now()
		m := s.newMatcher()
		t1 := time.Now()
		s.bornFromFaces()
		t2 := time.Now()
		s.doubtfulDates(0)
		t3 := time.Now()
		s.personCovers(s.facesByGroup())
		t4 := time.Now()
		_ = m
		t.Logf("matcher %v born %v doubtfulDates %v covers %v", t1.Sub(t0).Round(time.Millisecond), t2.Sub(t1).Round(time.Millisecond), t3.Sub(t2).Round(time.Millisecond), t4.Sub(t3).Round(time.Millisecond))
	}
}
