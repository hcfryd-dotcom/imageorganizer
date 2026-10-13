package server

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/places"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Places for photos whose files have none (see store/geo.go).
func (s *Server) geoRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/geo/borrow", s.borrowPlaces)
	mux.HandleFunc("POST /api/geo/set", s.setPlace)
	mux.HandleFunc("POST /api/geo/clear", s.clearPlace)
	mux.HandleFunc("GET /api/places", s.searchPlaces)
}

// borrowPlaces counts (apply false) or gives photos without a position
// the position of a photo taken within hours of them.
func (s *Server) borrowPlaces(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hours float64 `json:"hours"`
		Apply bool    `json:"apply"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if !(req.Hours > 0 && req.Hours <= 48) {
		httpError(w, 400, errors.New("hours must be between 0 and 48"))
		return
	}
	n, err := s.Store.BorrowPlaces(r.Context(), time.Duration(req.Hours*float64(time.Hour)), req.Apply)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, map[string]int{"count": n})
}

func (s *Server) setPlace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
		Lat float64 `json:"lat"`
		Lon float64 `json:"lon"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if math.IsNaN(req.Lat) || math.IsNaN(req.Lon) || math.Abs(req.Lat) > 90 || math.Abs(req.Lon) > 180 {
		httpError(w, 400, errors.New("not a position"))
		return
	}
	set, skipped, err := s.Store.SetPlace(r.Context(), req.IDs, req.Lat, req.Lon)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, map[string]int{"set": set, "skipped": skipped})
}

// clearPlace takes away the positions of ids that didn't come from their
// files, or with no ids, all borrowed ("near") or all set ("user").
func (s *Server) clearPlace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
		All string  `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	ids := req.IDs
	if len(ids) == 0 {
		if req.All != store.GeoNear && req.All != store.GeoUser {
			httpError(w, 400, errors.New("ids or all (near or user) needed"))
			return
		}
		ids = nil
	}
	n, err := s.Store.ClearPlace(r.Context(), ids, req.All)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, map[string]int{"cleared": n})
}

// searchPlaces finds towns by the start of their name, offline.
func (s *Server) searchPlaces(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 50 {
		limit = 12
	}
	type place struct {
		Name    string  `json:"name"`
		Country string  `json:"country"`
		Region  string  `json:"region,omitempty"`
		Area    string  `json:"area,omitempty"`
		Lat     float64 `json:"lat"`
		Lon     float64 `json:"lon"`
	}
	out := []place{}
	for _, p := range places.Search(r.URL.Query().Get("q"), limit) {
		out = append(out, place{p.Name, p.Country, p.Region, p.Area, p.Lat, p.Lon})
	}
	writeJSON(w, out)
}
