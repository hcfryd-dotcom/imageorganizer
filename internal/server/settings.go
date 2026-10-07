package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/embed"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Issue is something the user should know about: shown on the Settings
// page and as a "!" on its icon.
type Issue struct {
	Level string `json:"level"` // "error", "warning" or "info"
	Text  string `json:"text"`
}

// SettingsState is the Settings page: what is saved, what is in effect,
// which settings the command line fixes, and what needs attention.
type SettingsState struct {
	Saved         store.Settings    `json:"saved"`
	Effective     store.Settings    `json:"effective"` // values in use now, defaults filled in
	Defaults      store.Settings    `json:"defaults"`
	Locked        map[string]string `json:"locked"` // setting -> the flag that fixes it
	RestartNeeded bool              `json:"restart_needed"`
	Issues        []Issue           `json:"issues"`
	DataDir       string            `json:"data_dir"`
}

// SettingsHooks connect the Settings page to the running program.
type SettingsHooks struct {
	State   func() SettingsState
	Save    func(store.Settings) error // stores and applies what applies at once
	Restart func()                     // graceful exit; a service manager starts it again
}

func (s *Server) settingsRoutes(mux *http.ServeMux) {
	if s.Settings == nil {
		return
	}
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Settings.State())
	})
	mux.HandleFunc("PUT /api/settings", func(w http.ResponseWriter, r *http.Request) {
		var st store.Settings
		if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
			httpError(w, 400, err)
			return
		}
		if err := s.Settings.Save(st); err != nil {
			httpError(w, 500, err)
			return
		}
		writeJSON(w, s.Settings.State())
	})
	mux.HandleFunc("POST /api/settings/test-embed", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			URL string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
			httpError(w, 400, err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		model, vision, err := embed.Probe(ctx, req.URL)
		out := map[string]any{"ok": err == nil, "model": model, "vision": vision}
		if err != nil {
			out["error"] = err.Error()
		}
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /api/restart", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]bool{"ok": true})
		go s.Settings.Restart()
	})
}

// issueCount is what the Settings icon shows: problems and pending restarts.
func (s *Server) issueCount() int {
	if s.Settings == nil {
		return 0
	}
	return len(s.Settings.State().Issues)
}
