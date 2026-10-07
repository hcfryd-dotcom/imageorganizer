// Package scanner discovers files in library folders and imports them.
//
// Discovery (this file) only walks folders and records which files are new,
// changed or gone; it is as fast as directory listings. Importing — reading,
// hashing, metadata, thumbnails, move detection — is a pipeline stage
// (import.go) that pulls pending files from the store.
package scanner

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/source"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Status is a snapshot of discovery progress for the UI.
type Status struct {
	Running   bool   `json:"running"`
	Folder    string `json:"folder"`
	StartedAt int64  `json:"started_at"`
	Found     int64  `json:"found"`
	Pending   int64  `json:"pending"` // new or changed, handed to import
	Missing   int64  `json:"missing"`
	Errors    int64  `json:"errors"`
	LastError string `json:"last_error"`
}

// Scanner walks folders. Only one scan runs at a time.
type Scanner struct {
	store *store.Store
	log   *slog.Logger

	// OnDiscovered is called when new or changed files were recorded, to
	// wake the import stage.
	OnDiscovered func()

	running atomic.Bool
	mu      sync.Mutex // guards status
	status  Status
}

func New(st *store.Store, log *slog.Logger) *Scanner {
	return &Scanner{store: st, log: log}
}

func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *Scanner) update(f func(*Status)) {
	s.mu.Lock()
	f(&s.status)
	s.mu.Unlock()
}

// ScanAll scans every folder. It returns false if a scan was already running.
func (s *Scanner) ScanAll(ctx context.Context) bool {
	if !s.running.CompareAndSwap(false, true) {
		return false
	}
	defer s.running.Store(false)
	s.update(func(st *Status) { *st = Status{Running: true, StartedAt: time.Now().Unix()} })
	defer s.update(func(st *Status) { st.Running, st.Folder = false, "" })

	folders, err := s.store.Folders(ctx)
	if err != nil {
		s.log.Error("list folders", "err", err)
		return true
	}
	for _, f := range folders {
		if ctx.Err() != nil {
			break
		}
		if err := s.scanFolder(ctx, f); err != nil && ctx.Err() == nil {
			s.log.Error("scan folder", "folder", f.Location, "err", err)
			s.update(func(st *Status) { st.Errors++; st.LastError = err.Error() })
		}
	}
	st := s.Status()
	s.log.Info("scan done", "found", st.Found, "pending", st.Pending, "missing", st.Missing,
		"errors", st.Errors, "took", time.Since(time.Unix(st.StartedAt, 0)).Round(time.Millisecond))
	return true
}

// Spec turns a stored folder into a source spec with its credentials.
func Spec(f store.Folder) (source.Spec, error) {
	sp, err := source.Parse(f.Location)
	if err != nil {
		return sp, err
	}
	sp.Password = f.Secret
	return sp, nil
}

// discoverBatch is how many walked files are recorded per transaction.
const discoverBatch = 1000

func (s *Scanner) scanFolder(ctx context.Context, f store.Folder) error {
	s.update(func(st *Status) { st.Folder = f.Location })
	scan := time.Now().UnixNano()

	spec, err := Spec(f)
	if err != nil {
		return err
	}
	// A folder that cannot be opened is most likely an unreachable NAS or an
	// unmounted disk. Never mark its files missing in that case.
	conn, err := source.Open(ctx, spec)
	if err != nil {
		return fmt.Errorf("folder unavailable, skipping: %w", err)
	}
	defer conn.Close()
	// Directory reads have no context; closing the connection on
	// cancellation aborts them so shutdown isn't held up.
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	fsys := conn.FS

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("folder unreadable, skipping: %w", err)
	}
	if len(entries) == 0 && f.Images > 0 {
		return fmt.Errorf("folder %s is empty but has %d known photos; assuming it is not mounted", f.Location, f.Images)
	}

	var batch []store.Discovered
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		n, err := s.store.Discover(ctx, f.ID, scan, batch)
		batch = batch[:0]
		if err != nil {
			return err
		}
		if n > 0 {
			s.update(func(st *Status) { st.Pending += int64(n) })
			if s.OnDiscovered != nil {
				s.OnDiscovered()
			}
		}
		return nil
	}

	walkErr := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			s.log.Warn("walk", "folder", f.Location, "path", path, "err", err)
			if d != nil && d.IsDir() && path != "." {
				return fs.SkipDir
			}
			return err // the root itself failed: abort, don't mark anything missing
		}
		if d.IsDir() {
			if path != "." && source.Hidden(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if source.Hidden(d.Name()) || !media.Supported(path) {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		s.update(func(st *Status) { st.Found++ })
		batch = append(batch, store.Discovered{Path: path, Size: info.Size(), MTime: info.ModTime().Unix()})
		if len(batch) >= discoverBatch {
			return flush()
		}
		return nil
	})
	if err := flush(); err != nil {
		return err
	}
	if walkErr != nil {
		// An interrupted walk has not seen everything; don't mark anything missing.
		return walkErr
	}
	missing, err := s.store.MarkMissing(ctx, f.ID, scan)
	if err != nil {
		return err
	}
	if _, err := s.store.PruneMovedFiles(ctx); err != nil {
		return err
	}
	s.update(func(st *Status) { st.Missing += missing })
	return s.store.SetFolderScanned(ctx, f.ID, time.Now().Unix())
}
