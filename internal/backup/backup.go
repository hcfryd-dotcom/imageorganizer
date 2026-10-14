// Package backup keeps daily copies of the library database: everything
// the user has done (names of faces, places, dates, albums) lives in that
// one file. A copy is made when the newest is a day old, checked every
// hour while the program runs, and the oldest beyond the number to keep
// are removed. Only files named by this package are ever removed.
package backup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// DefaultKeep is how many copies are kept when the setting is 0.
const DefaultKeep = 14

// every is how old the newest copy may get.
const every = 24 * time.Hour

// name is a copy's file name: library-2026-10-14.bolt (and -2, -3 … for
// more copies the same day, made on request).
var name = regexp.MustCompile(`^library-\d{4}-\d{2}-\d{2}(-\d+)?\.bolt$`)

// Status is what the Settings page shows.
type Status struct {
	Dir    string `json:"dir"`             // "" when off
	Last   int64  `json:"last,omitempty"`  // newest copy, Unix seconds
	Copies int    `json:"copies"`          // copies in Dir
	Keep   int    `json:"keep"`            // copies kept
	Error  string `json:"error,omitempty"` // the last attempt's
}

// Runner makes the copies.
type Runner struct {
	Store      *store.Store
	DefaultDir string // used when the setting is empty
	Log        *slog.Logger

	mu      sync.Mutex
	lastErr string
	now     func() time.Time // tests
}

// where reads the settings: the folder ("" when off) and how many to keep.
func (r *Runner) where() (string, int) {
	s := r.Store.Settings()
	keep := s.BackupKeep
	if keep <= 0 {
		keep = DefaultKeep
	}
	switch d := strings.TrimSpace(s.BackupDir); d {
	case "off":
		return "", keep
	case "":
		return r.DefaultDir, keep
	default:
		return d, keep
	}
}

// copies lists the copies in dir, newest first.
func copies(dir string) []os.FileInfo {
	entries, _ := os.ReadDir(dir)
	var out []os.FileInfo
	for _, e := range entries {
		if e.Type().IsRegular() && name.MatchString(e.Name()) {
			if fi, err := e.Info(); err == nil {
				out = append(out, fi)
			}
		}
	}
	slices.SortFunc(out, func(a, b os.FileInfo) int { return b.ModTime().Compare(a.ModTime()) })
	return out
}

func (r *Runner) clock() time.Time {
	if r.now != nil {
		return r.now()
	}
	return time.Now()
}

// Status reports the folder, the newest copy and the last error.
func (r *Runner) Status() Status {
	dir, keep := r.where()
	r.mu.Lock()
	st := Status{Dir: dir, Keep: keep, Error: r.lastErr}
	r.mu.Unlock()
	if dir == "" {
		return st
	}
	cs := copies(dir)
	st.Copies = len(cs)
	if len(cs) > 0 {
		st.Last = cs[0].ModTime().Unix()
	}
	return st
}

// Run makes a copy when one is due, now and every hour, until ctx ends.
func (r *Runner) Run(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if _, err := r.backup(false); err != nil {
			r.Log.Warn("backing up the library", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Now makes a copy at once and returns its path.
func (r *Runner) Now() (string, error) { return r.backup(true) }

// backup makes a copy if forced or the newest is a day old, then removes
// the oldest copies beyond the number to keep.
func (r *Runner) backup(force bool) (string, error) {
	dir, keep := r.where()
	if dir == "" {
		return "", nil
	}
	path, err := r.make(dir, keep, force)
	r.mu.Lock()
	r.lastErr = ""
	if err != nil {
		r.lastErr = err.Error()
	}
	r.mu.Unlock()
	return path, err
}

func (r *Runner) make(dir string, keep int, force bool) (string, error) {
	now := r.clock()
	cs := copies(dir)
	if !force && len(cs) > 0 && now.Sub(cs[0].ModTime()) < every {
		return "", nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("can’t create the backup folder: %w", err)
	}
	day := now.Format("2006-01-02")
	path := filepath.Join(dir, "library-"+day+".bolt")
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("library-%s-%d.bolt", day, i))
	}
	if err := r.Store.Backup(path); err != nil {
		return "", err
	}
	_ = os.Chtimes(path, now, now)
	r.Log.Info("backed up the library", "path", path)
	cs = copies(dir)
	for _, old := range cs[min(keep, len(cs)):] {
		if err := os.Remove(filepath.Join(dir, old.Name())); err != nil {
			r.Log.Warn("removing an old backup", "file", old.Name(), "err", err)
		}
	}
	return path, nil
}
