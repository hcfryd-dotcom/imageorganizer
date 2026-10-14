package server

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/source"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// A tidy copy of the whole library: every photo once (copies of the same
// photo left out), from all its folders, into one folder sorted by year,
// month and what took it:
//
//	<dir>/2011/05 May/Canon/IMG_0037.JPG
//	<dir>/2024/12 December/iPhone/IMG_1234.HEIC
//	<dir>/2025/03 March/Screenshots/Screenshot 2025-03-01.png
//	<dir>/Unknown year/Unknown camera/scan.jpg
//
// Folder names are in the UI's language. The library's folders are only
// read; copying again adds only what is new.

// tidyNames are the folder names a tidy copy uses, per language.
type tidyNames struct {
	months                        [12]string
	noYear, screenshots, noCamera string
}

var tidyLang = map[string]tidyNames{
	"en": {
		months: [12]string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"},
		noYear: "Unknown year", screenshots: "Screenshots", noCamera: "Unknown camera",
	},
	"da": {
		months: [12]string{"Januar", "Februar", "Marts", "April", "Maj", "Juni", "Juli", "August", "September", "Oktober", "November", "December"},
		noYear: "Ukendt år", screenshots: "Skærmbilleder", noCamera: "Ukendt kamera",
	},
}

func (s *Server) tidyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/tidy", s.startTidy)
}

// tidyFolder is the folder in dir a photo goes into.
func (n tidyNames) folder(dir string, it store.TidyItem) string {
	src := it.Source
	switch src {
	case "screenshot":
		src = n.screenshots
	case "":
		src = n.noCamera
	}
	if it.Taken == nil {
		return filepath.Join(dir, n.noYear, src)
	}
	t := time.Unix(*it.Taken, 0).UTC()
	return filepath.Join(dir, fmt.Sprint(t.Year()), fmt.Sprintf("%02d %s", int(t.Month()), n.months[t.Month()-1]), src)
}

// startTidy counts what a tidy copy would hold (dry) or starts it.
func (s *Server) startTidy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir  string `json:"dir"`
		Lang string `json:"lang"`
		Dry  bool   `json:"dry"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	names, ok := tidyLang[req.Lang]
	if !ok {
		names = tidyLang["en"]
	}
	items, left := s.Store.TidyExport(r.Context())
	if req.Dry {
		type src struct {
			Name   string `json:"name"`
			Photos int    `json:"photos"`
		}
		count := map[string]int{}
		var bytes int64
		for _, it := range items {
			count[filepath.Base(names.folder("", it))]++
			bytes += it.Size
		}
		srcs := []src{}
		for k, n := range count {
			srcs = append(srcs, src{k, n})
		}
		slices.SortFunc(srcs, func(a, b src) int { return cmp.Or(cmp.Compare(b.Photos, a.Photos), cmp.Compare(a.Name, b.Name)) })
		writeJSON(w, map[string]any{"photos": len(items), "bytes": bytes, "left_out": left, "sources": srcs})
		return
	}
	dir, err := exportDir(req.Dir)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if f := s.libraryFolderAround(r.Context(), dir); f != "" {
		httpError(w, 400, fmt.Errorf("choose a folder outside the library folder %s: it would read its own copies", f))
		return
	}
	if len(items) == 0 {
		httpError(w, 400, fmt.Errorf("the library has no photos to copy"))
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
	s.export.st = exportStatus{Running: true, Tidy: true, Dir: dir, Total: len(items), LeftOut: left, StartedAt: time.Now().Unix()}
	st := s.export.st
	s.export.mu.Unlock()
	jobs := make([]copyJob, len(items))
	for i, it := range items {
		jobs[i] = copyJob{it.ExportItem, names.folder(dir, it)}
	}
	go s.runCopies(ctx, cancel, jobs)
	writeJSON(w, st)
}

// libraryFolderAround names the library folder on this computer that dir
// is in, or that is in dir; "" if none. A tidy copy inside the library
// would be scanned in as more files of the same photos.
func (s *Server) libraryFolderAround(ctx context.Context, dir string) string {
	folders, err := s.Store.Folders(ctx)
	if err != nil {
		return ""
	}
	for _, f := range folders {
		sp, err := source.Parse(f.Location)
		if err != nil || sp.Scheme != "file" {
			continue
		}
		if within(dir, f.Location) || within(f.Location, dir) {
			return f.Location
		}
	}
	return ""
}
