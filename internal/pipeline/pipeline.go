// Package pipeline runs background processing stages. There is no in-memory
// queue: the database is the queue. Each stage repeatedly asks the store for
// a batch of items that still need it (files to import, images without an
// embedding, images without face detection, ...), processes them in
// parallel, and sleeps until woken or a timeout when nothing is left. Work
// therefore survives restarts and stages compose naturally: importing an
// image makes it show up as needing an embedding.
package pipeline

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Stage describes one kind of background work over items of type J.
type Stage[J any] struct {
	Name    string
	Workers int
	Batch   int // items fetched per round; defaults to 4×Workers

	// Ready is called before work starts and again after the stage was
	// offline; it can wait for an external service. Optional.
	Ready func(ctx context.Context) error
	// Next returns up to n items still needing this stage, excluding skip.
	Next func(ctx context.Context, n int, skip map[int64]bool) []J
	// Key identifies an item, for skipping ones that failed.
	Key func(J) int64
	// Do processes one item.
	Do func(ctx context.Context, item J) error
	// Transient reports errors meaning "the service is down" rather than
	// "this item is bad": the stage backs off instead of skipping items.
	Transient func(error) bool
	// Drained is called when the stage runs out of work. Optional.
	Drained func(ctx context.Context)
	// Progress is called after each item succeeds. Optional.
	Progress func()

	Log *slog.Logger
}

// Status is reported to the UI.
type Status struct {
	Name      string `json:"name"`
	Paused    bool   `json:"paused"`
	Running   bool   `json:"running"`
	Online    bool   `json:"online"`
	Failed    int    `json:"failed"`
	LastError string `json:"last_error,omitempty"`
}

// Runner executes a Stage.
type Runner[J any] struct {
	st      Stage[J]
	wake    chan struct{}
	running atomic.Bool
	online  atomic.Bool
	paused  atomic.Bool

	mu      sync.Mutex
	lastErr string
	failed  map[int64]bool
}

func New[J any](st Stage[J]) *Runner[J] {
	if st.Batch == 0 {
		st.Batch = 4 * st.Workers
	}
	if st.Log == nil {
		st.Log = slog.Default()
	}
	return &Runner[J]{st: st, wake: make(chan struct{}, 1), failed: map[int64]bool{}}
}

// Pause stops the stage from starting new items; in-flight items finish.
func (r *Runner[J]) Pause() { r.paused.Store(true) }

// Resume continues a paused stage.
func (r *Runner[J]) Resume() {
	r.paused.Store(false)
	r.Wake()
}

// Wake tells the stage there may be new work.
func (r *Runner[J]) Wake() {
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *Runner[J]) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Status{Name: r.st.Name, Paused: r.paused.Load(), Running: r.running.Load(), Online: r.online.Load(), Failed: len(r.failed), LastError: r.lastErr}
}

func (r *Runner[J]) setErr(err error) {
	r.mu.Lock()
	r.lastErr = ""
	if err != nil {
		r.lastErr = err.Error()
	}
	r.mu.Unlock()
}

// sleep waits for d or a wake-up; false means ctx ended.
func (r *Runner[J]) sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-r.wake:
		return true
	case <-t.C:
		return true
	}
}

// Run processes work until ctx is cancelled. In-flight items finish (or
// observe ctx) before it returns.
func (r *Runner[J]) Run(ctx context.Context) {
	ready := false
	for ctx.Err() == nil {
		if r.paused.Load() {
			r.running.Store(false)
			if !r.sleep(ctx, time.Minute) {
				return
			}
			continue
		}
		if !ready && r.st.Ready != nil {
			if err := r.st.Ready(ctx); err != nil {
				r.online.Store(false)
				r.setErr(err)
				if !r.sleep(ctx, 30*time.Second) {
					return
				}
				continue
			}
			r.setErr(nil)
		}
		ready = true
		r.online.Store(true)

		r.mu.Lock()
		skip := make(map[int64]bool, len(r.failed))
		for k := range r.failed {
			skip[k] = true
		}
		r.mu.Unlock()
		items := r.st.Next(ctx, r.st.Batch, skip)
		if len(items) == 0 {
			if r.running.Swap(false) && r.st.Drained != nil {
				r.st.Drained(ctx)
			}
			// Retry failed items on the next quiet round.
			r.mu.Lock()
			clear(r.failed)
			r.mu.Unlock()
			if !r.sleep(ctx, 2*time.Minute) {
				return
			}
			continue
		}
		r.running.Store(true)
		if r.batch(ctx, items) {
			// Service unavailable: back off, then check Ready again.
			r.online.Store(false)
			ready = false
			if !r.sleep(ctx, 15*time.Second) {
				return
			}
		}
	}
}

// batch processes items in parallel; it reports whether the stage's service
// looks down.
func (r *Runner[J]) batch(ctx context.Context, items []J) (down bool) {
	ch := make(chan J)
	var unavailable atomic.Bool
	var wg sync.WaitGroup
	for range r.st.Workers {
		wg.Go(func() {
			for it := range ch {
				err := r.st.Do(ctx, it)
				switch {
				case err == nil:
					if r.st.Progress != nil {
						r.st.Progress()
					}
				case ctx.Err() != nil:
					// shutting down; not the item's fault
				case r.st.Transient != nil && r.st.Transient(err):
					unavailable.Store(true)
					r.setErr(err)
				default:
					r.mu.Lock()
					r.failed[r.st.Key(it)] = true
					r.mu.Unlock()
					r.setErr(err)
					r.st.Log.Warn(r.st.Name+" failed", "item", r.st.Key(it), "err", err)
				}
			}
		})
	}
	for _, it := range items {
		if ctx.Err() != nil || unavailable.Load() || r.paused.Load() {
			break
		}
		ch <- it
	}
	close(ch)
	wg.Wait()
	return unavailable.Load()
}
