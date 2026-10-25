package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Albums (see store/albums.go). The photos of one are listed with
// /api/images?album=<id>.

func (s *Server) albumRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/albums", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, s.Store.Albums(r.Context()))
	})
	mux.HandleFunc("POST /api/albums", s.createAlbum)
	mux.HandleFunc("POST /api/albums/from-folder", s.albumFromFolder)
	mux.HandleFunc("PATCH /api/albums/{id}", s.renameAlbum)
	mux.HandleFunc("DELETE /api/albums/{id}", s.deleteAlbum)
	mux.HandleFunc("POST /api/albums/{id}/add", s.albumPhotos(true))
	mux.HandleFunc("POST /api/albums/{id}/remove", s.albumPhotos(false))
	mux.HandleFunc("POST /api/albums/{id}/copy-to", s.setAlbumCopyTo)
	// A photo's number and title in the album: {"image": id, "n": 3, "title": "…"}.
	mux.HandleFunc("POST /api/albums/{id}/mark", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		var req struct {
			Image int64 `json:"image"`
			store.AlbumMark
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpError(w, 400, err)
			return
		}
		if err := s.Store.SetAlbumMark(r.Context(), id, req.Image, req.AlbumMark); err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
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

// albumFromFolder makes an album of the photos in a folder on this
// computer (and its sub-folders), e.g. one the owner sorted by hand in
// OneDrive: each file is known by its content (the importer's hash), so
// the photo the library has is found wherever its own file lies. The
// folder is only read. An album of that name already there gets them
// added. Photos the library doesn't have are counted, not imported.
func (s *Server) albumFromFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Folder string `json:"folder"`
		Name   string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	dir := filepath.Clean(strings.TrimSpace(req.Folder))
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		httpError(w, 400, fmt.Errorf("not a folder: %s", req.Folder))
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = filepath.Base(dir)
	}
	var ids []int64
	seen := map[int64]bool{}
	files, missing := 0, 0
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable part is skipped, not the whole folder
		}
		if r.Context().Err() != nil {
			return r.Context().Err()
		}
		if d.IsDir() || !media.Supported(p) {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		hash, _, err := media.HashReader(f)
		f.Close()
		if err != nil {
			return nil
		}
		files++
		if id, ok := s.Store.ImageByHash(hash); ok {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		} else {
			missing++
		}
		return nil
	})
	if err != nil {
		httpError(w, 500, err)
		return
	}
	res := map[string]any{"name": name, "files": files, "found": len(ids), "missing": missing}
	if len(ids) == 0 {
		writeJSON(w, res)
		return
	}
	var album store.Album
	for _, a := range s.Store.Albums(r.Context()) {
		if strings.EqualFold(a.Name, name) {
			album = a
		}
	}
	if album.ID != 0 {
		if _, err := s.Store.AddToAlbum(r.Context(), album.ID, ids); err != nil {
			storeError(w, err)
			return
		}
	} else if album, err = s.Store.CreateAlbum(r.Context(), name, ids); err != nil {
		httpError(w, 400, err)
		return
	}
	res["album"] = album.ID
	writeJSON(w, res)
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
		if add && n > 0 {
			s.copyAlbumPhotos(id, req.IDs) // to the album's own folder, if it has one
		}
		writeJSON(w, map[string]int{"changed": n})
	}
}
