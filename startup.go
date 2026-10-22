package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The start-up page. The program listens from its first moment, so the
// browser the start script opens finds it at once: until the library is
// open and everything is ready, every page is one that shows, in a bar,
// what the program is doing, and goes to the photos by itself when it is
// ready. Then the real handler takes over.

// startStep is a step of starting up: a key the page words (Danish or
// English) and how far along the bar it begins.
type startStep struct {
	Key  string  `json:"key"`
	From float64 `json:"from"`
}

var startSteps = map[string]float64{
	"starting":  0.02, // the program has started
	"opening":   0.05, // opening the library file
	"images":    0.10, // reading the photos
	"files":     0.30, // where their files are
	"vectors":   0.45, // the search index (search by meaning)
	"albums":    0.62,
	"faces":     0.65, // faces, people and names
	"models":    0.85, // face recognition models
	"preparing": 0.95,
}

type startup struct {
	mu    sync.Mutex
	step  string
	since time.Time // when the program started
	at    time.Time // when the step began
	ready atomic.Pointer[http.Handler]
}

func newStartup() *startup {
	now := time.Now()
	return &startup{step: "starting", since: now, at: now}
}

// set moves on to step (one of startSteps).
func (u *startup) set(step string) {
	u.mu.Lock()
	u.step, u.at = step, time.Now()
	u.mu.Unlock()
}

// done hands every request from now on to h.
func (u *startup) done(h http.Handler) { u.ready.Store(&h) }

func (u *startup) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h := u.ready.Load(); h != nil {
		(*h).ServeHTTP(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case r.URL.Path == "/api/startup":
		u.mu.Lock()
		step, since, at := u.step, u.since, u.at
		u.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"step": step, "from": startSteps[step], "next": nextStep(step),
			"secs": time.Since(since).Seconds(), "step_secs": time.Since(at).Seconds(),
		})
	case strings.HasPrefix(r.URL.Path, "/api/"), strings.HasPrefix(r.URL.Path, "/thumb/"), strings.HasPrefix(r.URL.Path, "/preview/"):
		w.Header().Set("Retry-After", "2")
		http.Error(w, "starting up", http.StatusServiceUnavailable)
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(startupPage))
	}
}

// nextStep is where the bar goes after step (the next step's start).
func nextStep(step string) float64 {
	from, next := startSteps[step], 1.0
	for _, v := range startSteps {
		if v > from && v < next {
			next = v
		}
	}
	return next
}

const startupPage = `<!doctype html>
<html lang="da"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>ImageOrganizer starter…</title>
<style>
  :root { color-scheme: dark; }
  html, body { height: 100%; margin: 0; }
  body { display: grid; place-items: center; font-family: "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif; color: #f3f1f7;
    background: radial-gradient(1100px 760px at 92% -12%, rgba(155, 123, 255, .26), transparent 62%), radial-gradient(900px 700px at -12% 112%, rgba(95, 224, 255, .14), transparent 62%), #0d0c13; }
  .box { width: min(520px, 86vw); }
  h1 { font: 400 34px "Sitka Heading", Georgia, serif; margin: 0 0 6px; }
  p { margin: 0; color: #a3a0b5; }
  .bar { height: 10px; border-radius: 999px; background: rgba(255, 255, 255, .08); overflow: hidden; margin: 26px 0 12px; }
  .bar > div { height: 100%; width: 2%; border-radius: 999px; background: linear-gradient(100deg, #9b7bff, #ff6fb1 55%, #ffb547); transition: width .5s ease; }
  #step { color: #f3f1f7; font-size: 16px; }
  .row { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; }
  #time { font-size: 13px; font-variant-numeric: tabular-nums; }
</style></head>
<body><div class="box">
  <h1>ImageOrganizer</h1>
  <p id="lead"></p>
  <div class="bar"><div id="fill"></div></div>
  <div class="row"><span id="step"></span><span id="time" class="muted"></span></div>
</div>
<script>
(function () {
  var lang = 'da';
  try { lang = localStorage.getItem('lang') || 'da'; } catch (e) {}
  var T = {
    da: {lead: 'Programmet starter. Det tager typisk et minut.', starting: 'Starter programmet…', opening: 'Åbner biblioteket…',
      images: 'Læser billederne…', files: 'Finder billedernes filer…', vectors: 'Indlæser søgning efter indhold…', albums: 'Indlæser albums…',
      faces: 'Indlæser ansigter, personer og navne…', models: 'Starter ansigtsgenkendelse…', preparing: 'Gør klar…',
      ready: 'Klar! Åbner dine billeder…', gone: 'Venter på programmet…', s: ' s'},
    en: {lead: 'The program is starting. It usually takes a minute.', starting: 'Starting the program…', opening: 'Opening the library…',
      images: 'Reading the photos…', files: 'Finding the photos’ files…', vectors: 'Loading search by meaning…', albums: 'Loading albums…',
      faces: 'Loading faces, people and names…', models: 'Starting face recognition…', preparing: 'Getting ready…',
      ready: 'Ready! Opening your photos…', gone: 'Waiting for the program…', s: ' s'}
  }[lang === 'en' ? 'en' : 'da'];
  document.title = lang === 'en' ? 'ImageOrganizer is starting…' : 'ImageOrganizer starter…';
  document.getElementById('lead').textContent = T.lead;
  var fill = document.getElementById('fill'), step = document.getElementById('step'), time = document.getElementById('time');
  function poll() {
    fetch('/api/startup', {cache: 'no-store'}).then(function (r) {
      // Not the start-up page any more: the program is ready.
      if (r.status === 404 || !(r.headers.get('content-type') || '').includes('json')) { ready(); return null; }
      return r.json();
    }).then(function (s) {
      if (!s) return;
      // Within a step the bar creeps on towards the next, never reaching it.
      var p = s.from + (s.next - s.from) * (1 - Math.exp(-s.step_secs / 12));
      fill.style.width = Math.max(2, p * 100).toFixed(1) + '%';
      step.textContent = T[s.step] || T.starting;
      time.textContent = Math.round(s.secs) + T.s;
      setTimeout(poll, 400);
    }).catch(function () { step.textContent = T.gone; setTimeout(poll, 1000); });
  }
  function ready() {
    fill.style.width = '100%';
    step.textContent = T.ready;
    setTimeout(function () { location.reload(); }, 400);
  }
  poll();
})();
</script>
</body></html>`
