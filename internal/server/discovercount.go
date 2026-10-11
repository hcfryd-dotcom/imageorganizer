package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// discoverStale is how old the Discover queue may get before it's built
// again, in the background: new photos change it too.
const discoverStale = 5 * time.Minute

// fileRounds is how many passes of automatic filing run after a change:
// each learns from the last, and few file anything after the third.
const fileRounds = 6

// fileEnough: a pass filing fewer faces than this is the last (another
// would file about as few, for seconds of work).
const fileEnough = 20

// discoverQueue is the Discover queue, built in the background: after a
// change, the faces now near certain are filed first (FileFaces), then the
// questions are listed, the most worth first. Building takes seconds on a
// big library, so the last queue is served meanwhile.
type discoverQueue struct {
	mu    sync.Mutex
	items []store.QueueItem
	at    time.Time     // when built; zero: never
	dirty bool          // a change was made: build again
	busy  bool          // building in the background
	ready chan struct{} // closed once first built
}

// discoverItems is the queue as last built, built again in the
// background after a change made here, or once stale. wait: wait for the
// first build (else nil until then).
func (s *Server) discoverItems(wait bool) []store.QueueItem {
	c := &s.disc
	c.mu.Lock()
	if c.ready == nil {
		c.ready = make(chan struct{})
		c.dirty = true
	}
	if (c.dirty || !c.at.IsZero() && time.Since(c.at) > discoverStale) && !c.busy {
		c.dirty = false
		c.busy = true
		first := c.at.IsZero()
		go func() {
			items := s.buildQueue()
			c.mu.Lock()
			c.items, c.at, c.busy = items, time.Now(), false
			c.mu.Unlock()
			if first {
				close(c.ready)
			}
		}()
	}
	ready := c.ready
	c.mu.Unlock()
	if wait {
		<-ready
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.items
}

// learnStatus is what the background learning is doing, pushed to the
// browsers as it goes ("learn" events): the faces it files show up in the
// counts as they're filed, not all at once at the end.
type learnStatus struct {
	Busy      bool            `json:"busy"`
	Phase     string          `json:"phase"` // "filing", "questions", or "" when done
	Pass      int             `json:"pass,omitempty"`
	Filed     int             `json:"filed"`   // faces filed by themselves this time
	Unfiled   int             `json:"unfiled"` // and taken back
	Faces     store.FaceStats `json:"faces"`
	Questions int             `json:"questions"` // in the Discover queue, when done
}

// learnEvery is how often progress is pushed, at most.
const learnEvery = 400 * time.Millisecond

// buildQueue files what is near certain, then lists the questions, saying
// how it goes.
func (s *Server) buildQueue() []store.QueueItem {
	ctx := context.Background()
	start := time.Now()
	st := learnStatus{Busy: true, Phase: "filing", Pass: 1}
	var last time.Time
	push := func(force bool) {
		if !force && time.Since(last) < learnEvery {
			return
		}
		last = time.Now()
		st.Faces = s.Store.FaceStats()
		s.events.publish("learn", st)
	}
	push(true)
	for pass := 1; pass <= fileRounds; pass++ {
		st.Pass = pass
		filed0, unfiled0 := st.Filed, st.Unfiled
		a, _, err := s.Store.FileFacesProgress(ctx, func(filed, unfiled int) {
			st.Filed, st.Unfiled = filed0+filed, unfiled0+unfiled
			push(false)
		})
		if err != nil {
			slog.Warn("filing faces", "err", err)
			break
		}
		if a < fileEnough {
			break
		}
	}
	// How the faces confirmed since last time match now, to tell later
	// whether what's learnt since makes them less likely.
	if _, err := s.Store.RecordApprovals(ctx); err != nil {
		slog.Warn("recording approvals", "err", err)
	}
	st.Phase = "questions"
	push(true)
	items := s.Store.DiscoverQueue(ctx)
	st.Busy, st.Phase, st.Questions = false, "", len(items)
	push(true)
	if st.Filed > 0 || st.Unfiled > 0 {
		slog.Info("filed faces", "filed", st.Filed, "unfiled", st.Unfiled, "questions", len(items), "took", time.Since(start).Round(time.Millisecond))
	}
	return items
}

// discoverCount is the Discover badge: the questions waiting.
func (s *Server) discoverCount() int { return len(s.discoverItems(false)) }

// countChanges marks the Discover queue to be built again after any
// change made through the API.
func (s *Server) countChanges(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		if r.Method != http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/") {
			s.disc.mu.Lock()
			s.disc.dirty = true
			s.disc.mu.Unlock()
		}
	})
}
