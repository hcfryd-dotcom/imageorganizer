package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/scanner"
	"github.com/lkarlslund/imageorganizer/internal/source"
)

// Saving a still from a video: the frame where the user paused, at full
// size, as a JPEG next to the video, named by the user. It is dated as
// the moment it shows (the video's date plus the time into it), so it
// sorts beside the video, and the folder is scanned to take it in. The
// video itself isn't touched.

func (s *Server) frameRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/images/{id}/frame", s.saveFrame)
}

// stillName makes a file name of what the user typed: without folders or
// characters Windows doesn't allow, ending in .jpg.
func stillName(name string) string {
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.Trim(name, ". ")
	if ext := strings.ToLower(filepath.Ext(name)); ext == ".jpg" || ext == ".jpeg" {
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	if name == "" {
		return ""
	}
	return name + ".jpg"
}

func (s *Server) saveFrame(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		At   float64 `json:"at"`
		Name string  `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	name := stillName(req.Name)
	if name == "" {
		httpError(w, 400, errors.New("give the picture a name"))
		return
	}
	f, err := s.Store.ImageFile(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	if !media.VideoFormat(f.Format) {
		httpError(w, 400, errors.New("that isn’t a video"))
		return
	}
	folder, err := s.Store.Folder(r.Context(), f.FolderID)
	if err != nil {
		storeError(w, err)
		return
	}
	spec, err := scanner.Spec(folder)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	video, ok := source.LocalPath(spec, f.Path)
	if !ok {
		httpError(w, 400, errors.New("pictures can only be saved from videos on this computer"))
		return
	}
	dest := freeName(filepath.Join(filepath.Dir(video), name))
	if err := media.SaveFrame(r.Context(), video, req.At, dest); err != nil {
		httpError(w, 500, err)
		return
	}
	// Dated as the moment it shows; the import falls back to the file's
	// time for a picture without EXIF.
	if d, err := s.Store.Image(r.Context(), id); err == nil && d.TakenAt != nil {
		t := time.Unix(*d.TakenAt, 0).Add(time.Duration(req.At * float64(time.Second)))
		_ = os.Chtimes(dest, t, t)
	}
	if s.Scanner != nil {
		go s.Scanner.ScanAll(s.scanCtx)
	}
	rel, _ := filepath.Rel(filepath.Dir(video), dest)
	writeJSON(w, map[string]string{"path": dest, "name": rel})
}
