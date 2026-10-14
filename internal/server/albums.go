package server

import (
	"encoding/json"
	"net/http"
)

// Albums (see store/albums.go). The photos of one are listed with
// /api/images?album=<id>.

func (s *Server) albumRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/albums", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Store.Albums(r.Context()))
	})
	mux.HandleFunc("POST /api/albums", s.createAlbum)
	mux.HandleFunc("PATCH /api/albums/{id}", s.renameAlbum)
	mux.HandleFunc("DELETE /api/albums/{id}", s.deleteAlbum)
	mux.HandleFunc("POST /api/albums/{id}/add", s.albumPhotos(true))
	mux.HandleFunc("POST /api/albums/{id}/remove", s.albumPhotos(false))
	mux.HandleFunc("GET /api/images/{id}/albums", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		writeJSON(w, s.Store.AlbumsOf(r.Context(), id))
	})
}

func (s *Server) createAlbum(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string  `json:"name"`
		IDs  []int64 `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	a, err := s.Store.CreateAlbum(r.Context(), req.Name, req.IDs)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, a)
}

func (s *Server) renameAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.RenameAlbum(r.Context(), id, req.Name); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) deleteAlbum(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.DeleteAlbum(r.Context(), id); err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// albumPhotos adds photos to an album, or takes them out.
func (s *Server) albumPhotos(add bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, 400, err)
			return
		}
		f := s.Store.RemoveFromAlbum
		if add {
			f = s.Store.AddToAlbum
		}
		n, err := f(r.Context(), id, req.IDs)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, map[string]int{"changed": n})
	}
}
