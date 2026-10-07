package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"sync"
	"time"
)

// broker fans server events out to connected browsers over SSE. Each browser
// holds one stream; when it drops, the UI knows the server went away, and the
// "hello" sent on reconnect tells it whether its assets are stale.
type broker struct {
	mu   sync.Mutex
	subs map[chan sseEvent]struct{}
}

type sseEvent struct {
	name string
	data []byte
}

func newBroker() *broker { return &broker{subs: map[chan sseEvent]struct{}{}} }

func (b *broker) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 16)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broker) unsubscribe(ch chan sseEvent) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *broker) publish(name string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- sseEvent{name, data}:
		default: // slow client; it will resync on the next event
		}
	}
}

type hello struct {
	AssetHash string `json:"asset_hash"` // everything except CSS: change => reload
	CSSHash   string `json:"css_hash"`   // CSS only: change => hot-swap stylesheets
	BuildID   string `json:"build_id"`   // the Go binary
	Dev       bool   `json:"dev"`
}

func (s *Server) hello() hello {
	a, c := s.assets.hashes()
	return hello{AssetHash: a, CSSHash: c, BuildID: s.BuildID, Dev: s.Dev}
}

func (s *Server) eventStream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.events.subscribe()
	defer s.events.unsubscribe(ch)

	send := func(name string, data []byte) bool {
		_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
		fl.Flush()
		return err == nil
	}
	h, _ := json.Marshal(s.hello())
	st, _ := json.Marshal(s.Scanner.Status())
	// retry: tells EventSource how fast to reconnect after the server restarts.
	fmt.Fprint(w, "retry: 1000\n\n")
	if !send("hello", h) || !send("scan", st) {
		return
	}

	// Heartbeats let the client detect a dead connection that the TCP stack
	// has not noticed yet (e.g. laptop sleep, network change).
	tick := time.NewTicker(10 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.scanCtx.Done():
			// Shutting down: end the stream so http.Server.Shutdown can
			// finish; browsers reconnect to the next instance.
			return
		case ev := <-ch:
			if !send(ev.name, ev.data) {
				return
			}
		case <-tick.C:
			if !send("ping", []byte(`{}`)) {
				return
			}
		}
	}
}

// pumpStatus pushes scan progress while a scan is running, plus one final
// update when it finishes so the UI can refresh.
func (s *Server) pumpStatus(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	wasRunning := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			st := s.Scanner.Status()
			if st.Running || wasRunning {
				s.events.publish("scan", st)
			}
			wasRunning = st.Running
		}
	}
}

// assetWatcher hashes the UI assets. In dev mode it re-hashes periodically
// and calls onChange, so open browsers hot-swap CSS or reload immediately.
type assetWatcher struct {
	web      fs.FS
	dev      bool
	onChange func()

	mu        sync.Mutex
	assetHash string
	cssHash   string
}

func newAssetWatcher(web fs.FS, dev bool, onChange func()) *assetWatcher {
	a := &assetWatcher{web: web, dev: dev, onChange: onChange}
	a.assetHash, a.cssHash = computeHashes(web)
	return a
}

func (a *assetWatcher) hashes() (string, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.assetHash, a.cssHash
}

func (a *assetWatcher) run(ctx context.Context) {
	if !a.dev {
		return
	}
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			ah, ch := computeHashes(a.web)
			a.mu.Lock()
			changed := ah != a.assetHash || ch != a.cssHash
			a.assetHash, a.cssHash = ah, ch
			a.mu.Unlock()
			if changed {
				a.onChange()
			}
		}
	}
}

// index renders index.html with cache-busting versions on asset URLs.
func (a *assetWatcher) index() ([]byte, error) {
	b, err := fs.ReadFile(a.web, "index.html")
	if err != nil {
		return nil, err
	}
	ah, ch := a.hashes()
	html := strings.NewReplacer("__ASSET_HASH__", ah, "__CSS_HASH__", ch).Replace(string(b))
	return []byte(html), nil
}

func computeHashes(web fs.FS) (asset, css string) {
	ah, chh := sha256.New(), sha256.New()
	_ = fs.WalkDir(web, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := fs.ReadFile(web, path)
		if err != nil {
			return nil
		}
		h := ah
		if strings.HasSuffix(path, ".css") {
			h = chh
		}
		fmt.Fprintf(h, "%s\x00%d\x00", path, len(data))
		h.Write(data)
		return nil
	})
	return hex.EncodeToString(ah.Sum(nil)[:8]), hex.EncodeToString(chh.Sum(nil)[:8])
}
