package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/lkarlslund/imageorganizer/internal/places"
	"github.com/lkarlslund/imageorganizer/internal/scanner"
	"github.com/lkarlslund/imageorganizer/internal/source"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Deleting photos: their files are moved into a "deleted photos" folder of
// the user's choosing, never erased, keeping the library folder's name and
// the path inside it: <trash>/<library folder>/<sub>/<file>. A photo whose
// files are gone is no longer shown, but the library keeps everything
// about it (names, ratings), so moving a file back brings it back as it
// was. Only folders on this computer can be deleted from.

func (s *Server) deleteRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/trash", s.trashInfo)
	mux.HandleFunc("POST /api/images/delete", s.deleteImages)
}

// defaultTrash is suggested the first time: a folder beside the user's
// pictures.
func defaultTrash() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Pictures", "ImageOrganizer - Deleted")
}

func (s *Server) trashInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"default": defaultTrash()})
}

type deleteResult struct {
	Deleted int      `json:"deleted"` // photos moved out
	Failed  int      `json:"failed"`
	Errors  []string `json:"errors,omitempty"`
	Trash   string   `json:"trash"`
}

func (s *Server) deleteImages(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs   []int64 `json:"ids"`
		Trash string  `json:"trash"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if len(req.IDs) == 0 {
		httpError(w, 400, errors.New("no photos chosen"))
		return
	}
	trash, err := exportDir(req.Trash)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := os.MkdirAll(trash, 0o755); err != nil {
		httpError(w, 400, fmt.Errorf("can’t create the folder: %w", err))
		return
	}
	res := deleteResult{Trash: trash}
	fail := func(err error) {
		res.Failed++
		if len(res.Errors) < 5 {
			res.Errors = append(res.Errors, err.Error())
		}
	}
	for _, id := range req.IDs {
		files, err := s.Store.PresentFiles(r.Context(), id)
		if err != nil {
			fail(fmt.Errorf("photo %d: %w", id, err))
			continue
		}
		var moved []int64
		var ferr error
		for _, f := range files {
			if err := s.moveToTrash(r, trash, f); err != nil {
				ferr = err
				break
			}
			moved = append(moved, f.FileID)
		}
		if len(moved) > 0 {
			if err := s.Store.MarkFilesGone(r.Context(), moved); err != nil {
				fail(err)
				continue
			}
		}
		if ferr != nil {
			fail(ferr)
			continue
		}
		res.Deleted++
	}
	s.Log.Info("moved photos to the deleted folder", "trash", trash, "photos", res.Deleted, "failed", res.Failed)
	writeJSON(w, res)
}

func (s *Server) moveToTrash(r *http.Request, trash string, f store.DeleteFile) error {
	folder, err := s.Store.Folder(r.Context(), f.FolderID)
	if err != nil {
		return err
	}
	spec, err := scanner.Spec(folder)
	if err != nil {
		return err
	}
	src, ok := source.LocalPath(spec, f.Path)
	if !ok {
		return fmt.Errorf("%s: photos on a network share can’t be deleted from here", filepath.Base(f.Path))
	}
	// <trash>/<library folder's name>/<path inside it>
	top := places.SafeName(filepath.Base(filepath.Clean(strings.TrimSuffix(folderPath(spec), string(filepath.Separator)))))
	dest := filepath.Join(trash, top, filepath.FromSlash(f.Path))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	dest = freeName(dest)
	return moveFile(src, dest)
}

// folderPath is a local library folder's path on this computer.
func folderPath(spec source.Spec) string {
	p, _ := source.LocalPath(spec, "")
	if p == "" || p == "." {
		return "library"
	}
	return p
}

// freeName is p, or "name (2).ext" and so on when p is taken: a deleted
// photo never replaces another.
func freeName(p string) string {
	ext := filepath.Ext(p)
	stem := strings.TrimSuffix(p, ext)
	for i := 1; ; i++ {
		q := p
		if i > 1 {
			q = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		if _, err := os.Lstat(q); errors.Is(err, fs.ErrNotExist) {
			return q
		}
	}
}

// moveFile moves src to dest: a rename on the same drive, else a copy
// (through a temporary file) and then removing the original, which is
// removed only once the copy is complete.
func moveFile(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	info, err := in.Stat()
	if err != nil {
		in.Close()
		return err
	}
	err = writeFileAtomic(dest, in)
	in.Close()
	if err != nil {
		return err
	}
	if out, err := os.Stat(dest); err != nil || out.Size() != info.Size() {
		os.Remove(dest)
		return fmt.Errorf("%s: the copy didn’t come out whole; the photo was left in place", filepath.Base(src))
	}
	_ = os.Chtimes(dest, info.ModTime(), info.ModTime())
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("%s: copied to the deleted folder, but the original couldn’t be removed: %w", filepath.Base(src), err)
	}
	return nil
}
