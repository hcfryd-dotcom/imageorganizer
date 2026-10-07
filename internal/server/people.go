package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// People endpoints. A person is made of match groups: name an unknown
// group, then confirm or reject "Is this also <name>?" candidates.

func (s *Server) peopleRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/people", s.listPeople)
	mux.HandleFunc("PATCH /api/people/{id}", s.updatePerson)
	mux.HandleFunc("GET /api/people/{id}/groups", s.personGroups)
	mux.HandleFunc("GET /api/people/{id}/candidates", s.personCandidates)
	mux.HandleFunc("POST /api/people/{id}/merge", s.mergePerson)
	mux.HandleFunc("GET /api/groups/{id}/faces", s.groupFaces)
	mux.HandleFunc("POST /api/groups/{id}/name", s.nameGroup)
	mux.HandleFunc("POST /api/groups/{id}/reject", s.rejectGroup)
	mux.HandleFunc("POST /api/groups/{id}/hide", s.hideGroup)
	mux.HandleFunc("POST /api/faces/{id}/remove", s.removeFace)
	mux.HandleFunc("POST /api/faces/{id}/notface", s.notAFace)
	mux.HandleFunc("POST /api/faces/{id}/tag", s.tagFace)
	mux.HandleFunc("GET /api/groups/{id}", s.groupItem)
	mux.HandleFunc("GET /api/images/{id}/faces", s.imageFaces)
	mux.HandleFunc("GET /api/review", s.review)
	mux.HandleFunc("POST /api/groups/{id}/skip", s.skipGroup)
	mux.HandleFunc("GET /api/groups/{id}/suggestions", s.groupSuggestions)
}

func (s *Server) listPeople(w http.ResponseWriter, r *http.Request) {
	known, unknown := s.Store.FacePhotoCounts()
	writeJSON(w, map[string]any{
		"people":         s.Store.People(r.Context()),
		"review":         s.Store.ReviewCount(s.Store.ReviewMin()),
		"known_photos":   known,
		"unknown_photos": unknown,
		"alone_photos":   len(s.Store.AloneImageIDs()),
	})
}

func (s *Server) review(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 40
	}
	items, total := s.Store.Review(r.Context(), s.Store.ReviewMin(), offset, limit, 8, r.URL.Query().Get("order"))
	writeJSON(w, map[string]any{"total": total, "faces": s.Store.ReviewFaces(s.Store.ReviewMin()), "items": items})
}

func (s *Server) groupSuggestions(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, s.Store.GroupSuggestions(r.Context(), id))
}

func (s *Server) skipGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.SkipGroup(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) updatePerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Name      *string `json:"name"`
		BirthYear *int    `json:"birth_year"`
		CoverFace *int64  `json:"cover_face"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.UpdatePerson(r.Context(), id, req.Name, req.BirthYear, req.CoverFace); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) personGroups(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.PersonGroups(r.Context(), id, 8)))
}

func (s *Server) personCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 12
	}
	writeJSON(w, nonNil(s.Store.Candidates(r.Context(), id, limit, 6)))
}

func (s *Server) mergePerson(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Into int64 `json:"into"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.MergePeople(r.Context(), id, req.Into); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) groupFaces(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.GroupFaces(r.Context(), id)))
}

// nameGroup assigns a group to a person (new or existing) and returns the
// person with the next "Is this also …?" candidates.
func (s *Server) nameGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Name     string  `json:"name"`
		PersonID int64   `json:"person_id"`
		Shown    []int64 `json:"shown"` // the faces the user saw when deciding
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	pid, err := s.Store.NameGroup(r.Context(), id, req.PersonID, req.Name)
	if err != nil {
		storeError(w, err)
		return
	}
	if err := s.Store.MarkSeen(r.Context(), pid, req.Shown); err != nil {
		s.Log.Warn("recording seen faces failed", "err", err)
	}
	writeJSON(w, map[string]any{
		"person_id":  pid,
		"candidates": nonNil(s.Store.Candidates(r.Context(), pid, 12, 6)),
	})
}

func (s *Server) rejectGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		PersonID int64 `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PersonID == 0 {
		httpError(w, 400, errors.New("person_id required"))
		return
	}
	if err := s.Store.RejectGroup(r.Context(), id, req.PersonID); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) hideGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	hidden := r.URL.Query().Get("unhide") == ""
	if err := s.Store.HideGroup(r.Context(), id, hidden); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) removeFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.RemoveFace(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// tagFace tags one face as a person and returns the person with the next
// "Is this also …?" candidates (the face's old group is usually first).
func (s *Server) tagFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Name     string `json:"name"`
		PersonID int64  `json:"person_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	pid, err := s.Store.TagFace(r.Context(), id, req.PersonID, req.Name)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]any{
		"person_id":  pid,
		"candidates": nonNil(s.Store.Candidates(r.Context(), pid, 12, 6)),
	})
}

func (s *Server) notAFace(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.NotAFace(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// groupItem is one group as a review card, with more sample faces than
// the inbox shows.
func (s *Server) groupItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	it, err := s.Store.GroupItem(r.Context(), id, 24)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, it)
}

func (s *Server) imageFaces(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, nonNil(s.Store.ImageFaces(r.Context(), id)))
}

func storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		httpError(w, 404, err)
	case errors.Is(err, store.ErrNameRequired):
		httpError(w, 400, err)
	default:
		httpError(w, 500, err)
	}
}

func nonNil[T any](v []T) []T {
	if v == nil {
		return []T{}
	}
	return v
}
