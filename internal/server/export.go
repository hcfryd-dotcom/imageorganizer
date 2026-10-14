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

	"github.com/lkarlslund/imageorganizer/internal/places"
	"github.com/lkarlslund/imageorganizer/internal/scanner"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Copying a person's photos out of the library, into a folder of the
// user's choosing, one subfolder per year: <dir>/2015/IMG_1234.jpg, and
// optionally per place within the year, from the photo's GPS position:
// <dir>/2015/Skagen/IMG_1234.jpg (photos without one stay in the year's
// folder). The library's own folders are only read.

// noYearFolder holds photos without a capture date.
const noYearFolder = "Unknown year"

type exportStatus struct {
	Running    bool   `json:"running"`
	Person     string `json:"person,omitempty"`
	Tidy       bool   `json:"tidy,omitempty"`     // the whole library, into a tidy folder (tidy.go)
	LeftOut    int    `json:"left_out,omitempty"` // tidy: copies of photos not copied again
	Fetch      bool   `json:"fetch,omitempty"`    // tidy: new photos fetched from From (fetch.go)
	From       string `json:"from,omitempty"`
	Dir        string `json:"dir,omitempty"`
	ByPlace    string `json:"by_place,omitempty"` // "", "area" or "town"
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
		ByPlace       string `json:"by_place"` // "": years only; "area": kommune or the like; "town": nearest town
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
	if req.ByPlace != "" && req.ByPlace != "area" && req.ByPlace != "town" {
		httpError(w, 400, fmt.Errorf("by_place is area, town or empty"))
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
		httpError(w, http.StatusConflict, s.exportBusy())
		return
	}
	ctx, cancel := context.WithCancel(s.scanCtx)
	s.export.cancel = cancel
	s.export.st = exportStatus{Running: true, Person: name, Dir: dir, ByPlace: req.ByPlace, Total: len(items), StartedAt: time.Now().Unix()}
	st := s.export.st
	s.export.mu.Unlock()
	go s.runExport(ctx, cancel, dir, items, req.ByPlace)
	writeJSON(w, st)
}

// exportBusy says what copy is running; call with s.export.mu held.
func (s *Server) exportBusy() error {
	if s.export.st.Tidy {
		return errors.New("already copying the library into a tidy folder")
	}
	return fmt.Errorf("already copying %s’s photos", s.export.st.Person)
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

func (s *Server) runExport(ctx context.Context, cancel context.CancelFunc, dir string, items []store.ExportItem, byPlace string) {
	placeOf := placeFolders(items, byPlace)
	jobs := make([]copyJob, len(items))
	for i, it := range items {
		jobs[i] = copyJob{it, yearFolder(dir, it.Taken, placeOf[it.ImageID])}
	}
	s.runCopies(ctx, cancel, jobs)
}

// copyJob is a photo to copy into a folder.
type copyJob struct {
	it  store.ExportItem
	dir string
}

// runCopies copies the photos out one by one, keeping s.export.st up to
// date, until done or stopped.
func (s *Server) runCopies(ctx context.Context, cancel context.CancelFunc, jobs []copyJob) {
	defer cancel()
	step := func(f func(st *exportStatus)) {
		s.export.mu.Lock()
		f(&s.export.st)
		s.export.mu.Unlock()
	}
	for _, j := range jobs {
		it := j.it
		if ctx.Err() != nil {
			step(func(st *exportStatus) { st.LastError = "stopped" })
			break
		}
		copied, err := s.copyInto(ctx, j.dir, it)
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
	s.Log.Info("copied photos out", "person", st.Person, "tidy", st.Tidy, "dir", st.Dir, "copied", st.Copied, "skipped", st.Skipped, "failed", st.Failed)
}

// placeFolders names the place folder of each photo with a GPS position
// (byPlace "area" or "town"; none for ""). Places in the country most of
// the photos are from are named alone ("Skagen"), others with their
// country ("Barcelona, Spanien").
func placeFolders(items []store.ExportItem, byPlace string) map[int64]string {
	out := map[int64]string{}
	if byPlace == "" {
		return out
	}
	level := places.ByArea
	if byPlace == "town" {
		level = places.ByTown
	}
	found := map[int64]places.Place{}
	countries := map[string]int{}
	for _, it := range items {
		if it.Lat == nil || it.Lon == nil {
			continue
		}
		if p, ok := places.Nearest(*it.Lat, *it.Lon); ok {
			found[it.ImageID] = p
			countries[p.Country]++
		}
	}
	home, most := "", 0
	for cc, n := range countries {
		if n > most || n == most && cc < home {
			home, most = cc, n
		}
	}
	for id, p := range found {
		out[id] = p.Label(level, home)
	}
	return out
}

// yearFolder is a photo's year's folder in dir, and its place's within it
// when place is set.
func yearFolder(dir string, taken *int64, place string) string {
	year := noYearFolder
	if taken != nil {
		year = strconv.Itoa(time.Unix(*taken, 0).UTC().Year())
	}
	d := filepath.Join(dir, year)
	if place != "" {
		d = filepath.Join(d, place)
	}
	return d
}

// copyInto copies one photo into the folder yearDir, dated like the photo.
// A file of the same name and size already there is taken to be this photo
// (copying again skips it); another file of the same name gets " (2)" and
// so on.
func (s *Server) copyInto(ctx context.Context, yearDir string, it store.ExportItem) (bool, error) {
	var mtime time.Time
	if it.Taken != nil {
		mtime = time.Unix(*it.Taken, 0).UTC()
	}
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
