package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/scanner"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Copying a person's photos out of the library, into a folder of the
// user's choosing, one subfolder per year: <dir>/2015/IMG_1234.jpg. The
// library's own folders are only read.

// noYearFolder holds photos without a capture date.
const noYearFolder = "Unknown year"

type exportStatus struct {
	Running    bool   `json:"running"`
	Person     string `json:"person,omitempty"`
	Dir        string `json:"dir,omitempty"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Copied     int    `json:"copied"`
	Skipped    int    `json:"skipped"` // already there, the same size
	Failed     int    `json:"failed"`
	LastError  string `json:"last_error,omitempty"`
	StartedAt  int64  `json:"started_at,omitempty"`
	FinishedAt int64  `json:"finished_at,omitempty"`
}

type exporter struct {
	mu     sync.Mutex
	st     exportStatus
	cancel context.CancelFunc
}

func (s *Server) exportRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/people/{id}/export", s.startExport)
	mux.HandleFunc("GET /api/export", s.exportStatus)
	mux.HandleFunc("POST /api/export/cancel", s.cancelExport)
}

func (s *Server) exportStatus(w http.ResponseWriter, r *http.Request) {
	s.export.mu.Lock()
	st := s.export.st
	s.export.mu.Unlock()
	writeJSON(w, st)
}

func (s *Server) cancelExport(w http.ResponseWriter, r *http.Request) {
	s.export.mu.Lock()
	if s.export.cancel != nil {
		s.export.cancel()
	}
	s.export.mu.Unlock()
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) startExport(w http.ResponseWriter, r *http.Request) {
	pid, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Dir           string `json:"dir"`
		ConfirmedOnly bool   `json:"confirmed_only"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	dir, err := exportDir(req.Dir)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	name, items, err := s.Store.PersonExport(r.Context(), pid, req.ConfirmedOnly)
	if err != nil {
		storeError(w, err)
		return
	}
	if len(items) == 0 {
		httpError(w, 400, fmt.Errorf("%s has no photos to copy", name))
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		httpError(w, 400, fmt.Errorf("can’t create the folder: %w", err))
		return
	}
	s.export.mu.Lock()
	if s.export.st.Running {
		s.export.mu.Unlock()
		httpError(w, http.StatusConflict, fmt.Errorf("already copying %s’s photos", s.export.st.Person))
		return
	}
	ctx, cancel := context.WithCancel(s.scanCtx)
	s.export.cancel = cancel
	s.export.st = exportStatus{Running: true, Person: name, Dir: dir, Total: len(items), StartedAt: time.Now().Unix()}
	st := s.export.st
	s.export.mu.Unlock()
	go s.runExport(ctx, cancel, dir, items)
	writeJSON(w, st)
}

// exportDir checks the folder the user typed: an absolute path on this
// computer. Quotes pasted from Explorer ("Copy as path") are dropped.
func exportDir(dir string) (string, error) {
	dir = strings.Trim(strings.TrimSpace(dir), `"`)
	if dir == "" {
		return "", errors.New("choose a folder to copy to")
	}
	if !filepath.IsAbs(dir) {
		return "", fmt.Errorf("give the whole path of the folder, such as %s", examplePath())
	}
	return filepath.Clean(dir), nil
}

func examplePath() string {
	if filepath.Separator == '\\' {
		return `C:\Users\you\Pictures\Anna`
	}
	return "/home/you/Pictures/Anna"
}

func (s *Server) runExport(ctx context.Context, cancel context.CancelFunc, dir string, items []store.ExportItem) {
	defer cancel()
	step := func(f func(st *exportStatus)) {
		s.export.mu.Lock()
		f(&s.export.st)
		s.export.mu.Unlock()
	}
	for _, it := range items {
		if ctx.Err() != nil {
			step(func(st *exportStatus) { st.LastError = "stopped" })
			break
		}
		copied, err := s.exportOne(ctx, dir, it)
		step(func(st *exportStatus) {
			st.Done++
			switch {
			case err != nil:
				st.Failed++
				st.LastError = fmt.Sprintf("%s: %v", path.Base(it.Path), err)
			case copied:
				st.Copied++
			default:
				st.Skipped++
			}
		})
		if err != nil {
			s.Log.Warn("copying a photo out failed", "path", it.Path, "err", err)
		}
	}
	step(func(st *exportStatus) { st.Running, st.FinishedAt = false, time.Now().Unix() })
	s.export.mu.Lock()
	st := s.export.st
	s.export.mu.Unlock()
	s.Log.Info("copied photos out", "person", st.Person, "dir", st.Dir, "copied", st.Copied, "skipped", st.Skipped, "failed", st.Failed)
}

// exportOne copies one photo into its year's folder. A file of the same
// name and size already there is taken to be this photo (copying again
// skips it); another file of the same name gets " (2)" and so on.
func (s *Server) exportOne(ctx context.Context, dir string, it store.ExportItem) (bool, error) {
	year := noYearFolder
	var mtime time.Time
	if it.Taken != nil {
		t := time.Unix(*it.Taken, 0).UTC()
		year, mtime = strconv.Itoa(t.Year()), t
	}
	yearDir := filepath.Join(dir, year)
	if err := os.MkdirAll(yearDir, 0o755); err != nil {
		return false, err
	}
	folder, err := s.Store.Folder(ctx, it.FolderID)
	if err != nil {
		return false, err
	}
	spec, err := scanner.Spec(folder)
	if err != nil {
		return false, err
	}
	copied := false
	err = s.Pool.Do(ctx, folder.ID, spec, func(fsys fs.FS) error {
		info, err := fs.Stat(fsys, it.Path)
		if err != nil {
			return err
		}
		dest, exists := exportTarget(yearDir, path.Base(it.Path), info.Size())
		if exists {
			return nil
		}
		if mtime.IsZero() {
			mtime = info.ModTime()
		}
		src, err := fsys.Open(it.Path)
		if err != nil {
			return err
		}
		defer src.Close()
		if err := writeFileAtomic(dest, src); err != nil {
			return err
		}
		_ = os.Chtimes(dest, mtime, mtime) // dated like the photo, so folders sort by it
		copied = true
		return nil
	})
	return copied, err
}

// exportTarget picks the file name in dir for a photo called name of the
// given size: name itself, or "name (2).ext" and so on when another file
// has it. exists is true when a file of that name and size is there.
func exportTarget(dir, name string, size int64) (dest string, exists bool) {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 1; ; i++ {
		n := name
		if i > 1 {
			n = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		p := filepath.Join(dir, n)
		fi, err := os.Stat(p)
		if errors.Is(err, fs.ErrNotExist) {
			return p, false
		}
		if err == nil && fi.Mode().IsRegular() && fi.Size() == size {
			return p, true
		}
	}
}

// writeFileAtomic writes r to dest through a temporary file, so a copy cut
// short never leaves a broken photo behind.
func writeFileAtomic(dest string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".copying-*")
	if err != nil {
		return err
	}
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
