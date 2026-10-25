package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// An album can have a folder of its own on this computer: each photo put
// in the album is copied there as well, sorted by year and month like a
// tidy copy (<dir>/2024/05 May/IMG_1234.HEIC; a file of the same name and
// size already there is taken to be it).
// Taking a photo out of the album, or deleting the album, leaves the
// copies alone. The library's folders are only read.

// setAlbumCopyTo sets the album's folder ({"folder": ""} stops copying)
// and copies the photos already in the album there, in the background.
func (s *Server) setAlbumCopyTo(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Folder string `json:"folder"`
		Lang   string `json:"lang"` // of the year and month folder names
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	dir := ""
	if strings.TrimSpace(req.Folder) != "" {
		if dir, err = exportDir(req.Folder); err != nil {
			httpError(w, 400, err)
			return
		}
		if f := s.libraryFolderAround(r.Context(), dir); f != "" {
			httpError(w, 400, fmt.Errorf("choose a folder outside the library folder %s: it would read its own copies", f))
			return
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			httpError(w, 400, fmt.Errorf("can’t create the folder: %w", err))
			return
		}
	}
	if err := s.Store.SetAlbumCopyTo(r.Context(), id, dir, req.Lang); err != nil {
		storeError(w, err)
		return
	}
	n := s.copyAlbumPhotos(id, nil)
	writeJSON(w, map[string]any{"folder": dir, "copying": n})
}

// copyAlbumPhotos copies photos ids of album id (nil: all of them) to the
// album's folder, if it has one, in the background, one album copy at a
// time. It returns how many photos it will look at.
func (s *Server) copyAlbumPhotos(id int64, ids []int64) int {
	dir, lang, items := s.Store.AlbumCopyItems(context.Background(), id, ids)
	names, ok := tidyLang[lang]
	if !ok {
		names = tidyLang["en"]
	}
	if dir == "" || len(items) == 0 {
		return 0
	}
	go func() {
		s.albumCopy.Lock()
		defer s.albumCopy.Unlock()
		copied, failed := 0, 0
		for _, it := range items {
			if s.scanCtx.Err() != nil {
				return
			}
			ok, err := s.copyInto(s.scanCtx, names.dated(dir, it.Taken), it)
			if err != nil {
				failed++
				s.Log.Warn("copying a photo to the album's folder failed", "path", it.Path, "dir", dir, "err", err)
			} else if ok {
				copied++
			}
		}
		if copied > 0 || failed > 0 {
			s.Log.Info("copied album photos", "album", id, "dir", dir, "copied", copied, "failed", failed)
		}
	}()
	return len(items)
}
