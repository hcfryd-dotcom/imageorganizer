package server

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Filing every inbox group whose best suggestion is sure enough at once,
// instead of one card at a time.

func (s *Server) confidentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/review/confident", s.confidentPreview)
	mux.HandleFunc("POST /api/review/confident", s.confidentAccept)
}

type confidentPerson struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Groups int    `json:"groups"`
	Faces  int    `json:"faces"`
}

type confidentSummary struct {
	Min       float32           `json:"min"`
	Groups    int               `json:"groups"`
	Faces     int               `json:"faces"`
	Ambiguous int               `json:"ambiguous"` // left out: as alike to two people
	People    []confidentPerson `json:"people"`
}

// parseMin reads a likeness threshold given as a fraction (0.75) or a
// percentage (75). Below 50% suggestions are guesses, so it is refused.
func parseMin(v string) (float32, error) {
	f, err := strconv.ParseFloat(v, 32)
	if err != nil {
		return 0, fmt.Errorf("give a percentage, such as 75")
	}
	if f > 1 {
		f /= 100
	}
	if f < 0.5 || f > 1 {
		return 0, fmt.Errorf("choose between 50%% and 100%%")
	}
	return float32(f), nil
}

func summarize(minScore float32, picks []store.ConfidentPick, ambiguous int) confidentSummary {
	sum := confidentSummary{Min: minScore, Groups: len(picks), Ambiguous: ambiguous, People: []confidentPerson{}}
	by := map[int64]*confidentPerson{}
	for _, p := range picks {
		sum.Faces += p.Faces
		cp := by[p.Person]
		if cp == nil {
			cp = &confidentPerson{ID: p.Person, Name: p.Name}
			by[p.Person] = cp
		}
		cp.Groups++
		cp.Faces += p.Faces
	}
	for _, cp := range by {
		sum.People = append(sum.People, *cp)
	}
	slices.SortFunc(sum.People, func(a, b confidentPerson) int {
		return cmp.Or(cmp.Compare(b.Faces, a.Faces), cmp.Compare(a.Name, b.Name))
	})
	return sum
}

func (s *Server) confidentPreview(w http.ResponseWriter, r *http.Request) {
	minScore, err := parseMin(r.URL.Query().Get("min"))
	if err != nil {
		httpError(w, 400, err)
		return
	}
	picks, amb := s.Store.ConfidentReview(r.Context(), s.Store.ReviewMin(), minScore)
	writeJSON(w, summarize(minScore, picks, amb))
}

func (s *Server) confidentAccept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Min json.Number `json:"min"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	minScore, err := parseMin(req.Min.String())
	if err != nil {
		httpError(w, 400, err)
		return
	}
	picks, amb := s.Store.ConfidentReview(r.Context(), s.Store.ReviewMin(), minScore)
	var done []store.ConfidentPick
	failed := 0
	for _, p := range picks {
		if r.Context().Err() != nil {
			break
		}
		if _, err := s.Store.NameGroup(r.Context(), p.Group, p.Person, ""); err != nil {
			// Merged or changed meanwhile (filing one group can file others).
			failed++
			continue
		}
		done = append(done, p)
	}
	s.Log.Info("filed confident review groups", "min", minScore, "groups", len(done), "failed", failed)
	sum := summarize(minScore, done, amb)
	writeJSON(w, map[string]any{"filed": sum, "failed": failed})
}
