package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Fetching new photos into the tidy folder: from a folder outside the
// library (the phone's OneDrive camera folder, say), every photo the
// library doesn't have yet is copied into the tidy folder, sorted like the
// tidy copy (tidy.go), and the library is scanned to take them in. The
// source folder is only read.
//
// A file is known by its content, as the importer knows it. To spare
// reading every file each time, the files already looked at (path, size,
// modification time) are remembered in the data folder.

func (s *Server) fetchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tidy/fetch", s.startFetch)
}

// seenFile is how a source file looked when it was last looked at.
type seenFile struct {
	Size  int64 `json:"s"`
	MTime int64 `json:"m"`
}

// seenPath is the file remembering what was looked at in the folder from.
func (s *Server) seenPath(from string) string {
	h := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(from))))
	return filepath.Join(s.DataDir, "fetch-seen-"+hex.EncodeToString(h[:6])+".json")
}

func (s *Server) loadSeen(from string) map[string]seenFile {
	seen := map[string]seenFile{}
	if data, err := os.ReadFile(s.seenPath(from)); err == nil {
		_ = json.Unmarshal(data, &seen)
	}
	return seen
}

func (s *Server) saveSeen(from string, seen map[string]seenFile) {
	data, err := json.Marshal(seen)
	if err == nil {
		err = writeFileAtomic(s.seenPath(from), bytes.NewReader(data))
	}
	if err != nil {
		s.Log.Warn("remembering fetched files", "err", err)
	}
}

func (s *Server) startFetch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From string `json:"from"`
		Dir  string `json:"dir"`
		Lang string `json:"lang"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	names, ok := tidyLang[req.Lang]
	if !ok {
		names = tidyLang["en"]
	}
	from, err := exportDir(req.From)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	dir, err := exportDir(req.Dir)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if fi, err := os.Stat(from); err != nil || !fi.IsDir() {
		httpError(w, 400, fmt.Errorf("can’t find the folder %s", from))
		return
	}
	if within(from, dir) || within(dir, from) {
		httpError(w, 400, errors.New("the folder to fetch from and the tidy folder can’t be inside each other"))
		return
	}
	if s.DataDir == "" {
		httpError(w, 500, errors.New("no data folder to remember fetched files in"))
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
	s.export.st = exportStatus{Running: true, Tidy: true, Fetch: true, From: from, Dir: dir, StartedAt: time.Now().Unix()}
	st := s.export.st
	s.export.mu.Unlock()
	go s.runFetch(ctx, cancel, from, dir, names)
	writeJSON(w, st)
}

// within reports whether path a is b or inside it, ignoring case.
func within(a, b string) bool {
	a, b = strings.ToLower(filepath.Clean(a)), strings.ToLower(filepath.Clean(b))
	return a == b || strings.HasPrefix(a, strings.TrimSuffix(b, string(filepath.Separator))+string(filepath.Separator))
}

func (s *Server) runFetch(ctx context.Context, cancel context.CancelFunc, from, dir string, names tidyNames) {
	defer cancel()
	step := func(f func(st *exportStatus)) {
		s.export.mu.Lock()
		f(&s.export.st)
		s.export.mu.Unlock()
	}
	// The photos in the folder first, so the progress has a total.
	type file struct {
		rel  string
		size int64
		mod  time.Time
	}
	var files []file
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil {
			return ctx.Err() // a folder that can't be read is passed over
		}
		if d.IsDir() || !media.Supported(p) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(from, p)
		files = append(files, file{rel, info.Size(), info.ModTime()})
		return nil
	})
	if err != nil {
		step(func(st *exportStatus) { st.LastError = err.Error() })
	}
	step(func(st *exportStatus) { st.Total = len(files) })
	seen := s.loadSeen(from)
	done := map[string]bool{}
	sinceSave := 0
	for _, f := range files {
		if ctx.Err() != nil {
			step(func(st *exportStatus) { st.LastError = "stopped" })
			break
		}
		key := filepath.ToSlash(f.rel)
		was, looked := seen[key]
		copied, err := false, error(nil)
		if !looked || was.Size != f.size || was.MTime != f.mod.Unix() {
			copied, err = s.fetchOne(ctx, filepath.Join(from, f.rel), f.mod, dir, names, done)
			if err == nil {
				seen[key] = seenFile{f.size, f.mod.Unix()}
				sinceSave++
			}
		}
		step(func(st *exportStatus) {
			st.Done++
			switch {
			case err != nil:
				st.Failed++
				st.LastError = fmt.Sprintf("%s: %v", filepath.Base(f.rel), err)
			case copied:
				st.Copied++
			default:
				st.Skipped++
			}
		})
		if err != nil {
			s.Log.Warn("fetching a photo failed", "path", f.rel, "err", err)
		}
		if sinceSave >= 500 { // a stop or a crash keeps most of the work
			s.saveSeen(from, seen)
			sinceSave = 0
		}
	}
	s.saveSeen(from, seen)
	s.export.mu.Lock()
	s.export.st.Running, s.export.st.FinishedAt = false, time.Now().Unix()
	st := s.export.st
	s.export.mu.Unlock()
	s.Log.Info("fetched new photos", "from", from, "dir", dir, "copied", st.Copied, "known", st.Skipped, "failed", st.Failed)
	if st.Copied > 0 && s.Scanner != nil {
		go s.Scanner.ScanAll(s.scanCtx) // take the new photos in
	}
}

// fetchOne copies the photo at path into its tidy folder in dir, unless
// the library has it already (its content, by the importer's hash).
func (s *Server) fetchOne(ctx context.Context, path string, mod time.Time, dir string, names tidyNames, done map[string]bool) (bool, error) {
	// The content hash, and what took it when: a photo is read whole, a
	// video in pieces (it may be gigabytes) and then asked with ffprobe.
	var (
		hash string
		size int64
		data []byte
		info media.Info
	)
	if media.IsVideo(path) {
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		hash, size, err = media.HashReader(f)
		f.Close()
		if err != nil {
			return false, err
		}
	} else {
		var err error
		if data, err = os.ReadFile(path); err != nil {
			return false, err
		}
		hash, size = fmt.Sprintf("%016x%08x", xxhash.Sum64(data), len(data)), int64(len(data))
	}
	if known, _ := s.Store.HashKnown(ctx, hash); known || done[hash] { // done: fetched in this run, not imported yet
		return false, nil
	}
	if data != nil {
		r, _ := media.Process(path, data, false, 0) // metadata even when it can't be decoded
		info = r.Info
	} else {
		info, _ = media.VideoInfo(ctx, path)
	}
	taken := mod.UTC()
	if info.TakenAt != nil {
		taken = info.TakenAt.UTC()
	}
	t := taken.Unix()
	it := store.TidyItem{
		ExportItem: store.ExportItem{Taken: &t},
		Source:     store.PhotoSource(info.CameraMake, info.CameraModel, info.Format, path),
	}
	folder := names.folder(dir, it)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return false, err
	}
	dest, exists := exportTarget(folder, filepath.Base(path), size)
	if exists {
		return false, nil
	}
	var src io.Reader = bytes.NewReader(data)
	if data == nil {
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer f.Close()
		src = f
	}
	if err := writeFileAtomic(dest, src); err != nil {
		return false, err
	}
	_ = os.Chtimes(dest, taken, taken)
	done[hash] = true
	return true, nil
}
