// Package server exposes the library over HTTP: a JSON API, thumbnails,
// originals, and the web UI.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lkarlslund/imageorganizer/internal/faces"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/embed"
	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/scanner"
	"github.com/lkarlslund/imageorganizer/internal/source"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Config wires the server's dependencies.
type Config struct {
	Store      *store.Store
	Scanner    *scanner.Scanner
	Thumbs     *scanner.ThumbCache
	FaceCrops  *scanner.ThumbCache
	Pool       *source.Pool
	Embed      *embed.Client                       // nil disables semantic search
	Faces      *faces.Engine                       // nil disables finding a face from a picture
	Settings   *SettingsHooks                      // nil: no Settings page
	Jobs       func() map[string]any               // background stage progress, for the UI
	Control    func(stage string, pause bool) bool // pause/resume a stage
	WakeImport func()                              // there is a file to (re)import now, e.g. a photo just turned
	PreviewDir string
	DataDir    string // the library's own files (database, caches); "" in tests
	Web        fs.FS  // UI assets; os.DirFS in dev mode so edits are picked up live
	Dev        bool
	BuildID    string
	Log        *slog.Logger
	// Restart shuts the server down to be started again (POST
	// /debug/restart, dev mode): nil if it can't be.
	Restart func()
}

// RestartCode is the exit code of a server that was asked to restart: the
// dev supervisor starts it again at once (scripts/dev.sh).
const RestartCode = 75

type Server struct {
	Config
	assets  *assetWatcher
	events  *broker
	scanCtx context.Context
	disc    discoverQueue
	export  exporter // copying a person's photos out, one at a time
}

func New(ctx context.Context, c Config) *Server {
	// Fail loudly at startup rather than serving with features silently
	// missing (a nil pool once broke file serving without any error).
	switch {
	case c.Store == nil, c.Scanner == nil, c.Thumbs == nil, c.Pool == nil, c.Jobs == nil, c.Web == nil:
		panic("server.New: Store, Scanner, Thumbs, Pool, Jobs and Web are required")
	}
	s := &Server{Config: c, events: newBroker(), scanCtx: ctx}
	s.assets = newAssetWatcher(c.Web, c.Dev, func() { s.events.publish("hello", s.hello()) })
	go s.assets.run(ctx)
	go s.pumpStatus(ctx)
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.index)
	mux.Handle("GET /static/", http.StripPrefix("/static/", s.static()))
	mux.HandleFunc("GET /thumb/{hash}", s.thumb)
	mux.HandleFunc("GET /face/{key}", s.faceCrop)
	mux.HandleFunc("GET /original/{id}", s.original)
	mux.HandleFunc("GET /preview/{id}", s.preview)

	mux.HandleFunc("GET /api/events", s.eventStream)
	mux.HandleFunc("GET /api/stats", s.stats)
	mux.HandleFunc("GET /api/folders", s.listFolders)
	mux.HandleFunc("POST /api/folders", s.addFolder)
	mux.HandleFunc("DELETE /api/folders/{id}", s.removeFolder)
	mux.HandleFunc("POST /api/scan", s.startScan)
	mux.HandleFunc("POST /api/jobs/{stage}/{action}", s.controlJob)
	s.peopleRoutes(mux)
	s.exportRoutes(mux)
	s.tidyRoutes(mux)
	s.fetchRoutes(mux)
	s.albumRoutes(mux)
	s.confidentRoutes(mux)
	s.deleteRoutes(mux)
	s.geoRoutes(mux)
	s.settingsRoutes(mux)
	mux.HandleFunc("POST /api/browse", s.browse)
	mux.HandleFunc("GET /api/images", s.listImages)
	mux.HandleFunc("GET /api/images/{id}", s.getImage)
	mux.HandleFunc("POST /api/face-search", s.faceSearch)
	mux.HandleFunc("GET /api/images/{id}/around", s.aroundImage)
	mux.HandleFunc("POST /api/images/{id}/rotate", s.rotateImage)
	mux.HandleFunc("PATCH /api/images/{id}", s.editImage)
	mux.HandleFunc("GET /api/geo", s.geo)
	mux.HandleFunc("GET /api/timeline", s.timeline)
	if s.Dev {
		// Profiling in dev mode only: go tool pprof http://host/debug/pprof/profile
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		// A consistent database copy for offline experiments. It holds
		// folder passwords, so only this machine may fetch it.
		mux.HandleFunc("GET /debug/db", func(w http.ResponseWriter, r *http.Request) {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
				http.Error(w, "local only", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			if err := s.Store.Snapshot(w); err != nil {
				s.Log.Warn("database snapshot failed", "err", err)
			}
		})
		// Restart now: shut down gracefully and exit with RestartCode, which
		// the dev supervisor (scripts/dev.sh) answers by rebuilding and
		// starting again at once, not waiting for edits to settle. This
		// machine only.
		mux.HandleFunc("POST /debug/restart", func(w http.ResponseWriter, r *http.Request) {
			host, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
				http.Error(w, "local only", http.StatusForbidden)
				return
			}
			if s.Restart == nil {
				http.Error(w, "can't restart", http.StatusNotImplemented)
				return
			}
			writeJSON(w, map[string]bool{"ok": true})
			s.Log.Info("restart requested")
			go s.Restart()
		})
	}
	return s.logRequests(s.countChanges(mux))
}

func (s *Server) logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/events" {
			s.Log.Debug("http", "method", r.Method, "path", r.URL.Path, "dur", time.Since(start))
		}
	})
}

// --- helpers ---

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// --- UI ---

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	html, err := s.assets.index()
	if err != nil {
		httpError(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(html)
}

func (s *Server) static() http.Handler {
	fsrv := http.FileServerFS(s.Web)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.Dev || r.URL.Query().Get("v") == "" {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		fsrv.ServeHTTP(w, r)
	})
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{24}$`)

func (s *Server) thumb(w http.ResponseWriter, r *http.Request) {
	hash := strings.TrimSuffix(r.PathValue("hash"), ".jpg")
	if !hashRe.MatchString(hash) {
		http.NotFound(w, r)
		return
	}
	// A photo turned but not drawn again yet: turn the old thumbnail, and
	// don't let the browser keep it.
	if deg := s.Store.ThumbRotation(hash); deg != 0 {
		if data, err := turnJPEG(s.Thumbs.Path(hash), deg); err == nil {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write(data)
			return
		}
	}
	// Content-addressed (and rotation in the URL), so it can be cached forever.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, s.Thumbs.Path(hash))
}

// withFile runs fn with the file system holding a present copy of image id.
func (s *Server) withFile(r *http.Request, fn func(fsys fs.FS, f store.ImageFile) error) error {
	id, err := pathID(r)
	if err != nil {
		return fs.ErrNotExist
	}
	f, err := s.Store.ImageFile(r.Context(), id)
	if err != nil {
		return fs.ErrNotExist
	}
	folder, err := s.Store.Folder(r.Context(), f.FolderID)
	if err != nil {
		return fs.ErrNotExist
	}
	spec, err := scanner.Spec(folder)
	if err != nil {
		return err
	}
	return s.Pool.Do(r.Context(), folder.ID, spec, func(fsys fs.FS) error { return fn(fsys, f) })
}

func (s *Server) fileError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, fs.ErrNotExist) {
		http.NotFound(w, r)
		return
	}
	httpError(w, http.StatusBadGateway, err)
}

var faceKeyRe = regexp.MustCompile(`^[0-9a-f]{24}_[0-9]{1,3}$`)

// faceCrop serves a face crop named by content hash and face index;
// content-addressed, so cacheable forever.
func (s *Server) faceCrop(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSuffix(r.PathValue("key"), ".jpg")
	if !faceKeyRe.MatchString(key) || s.FaceCrops == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	if r.URL.Query().Has("level") { // light evened out, for comparing faces
		data, err := levelFace(s.FaceCrops.Path(key))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(data)
		return
	}
	http.ServeFile(w, r, s.FaceCrops.Path(key))
}

func (s *Server) original(w http.ResponseWriter, r *http.Request) {
	err := s.withFile(r, func(fsys fs.FS, f store.ImageFile) error {
		if _, err := fs.Stat(fsys, f.Path); err != nil {
			return err
		}
		if r.URL.Query().Has("download") {
			w.Header().Set("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(path.Base(f.Path), `"`, "")+`"`)
		}
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeFileFS(w, r, fsys, f.Path)
		return nil
	})
	if err != nil {
		s.fileError(w, r, err)
	}
}

// preview serves a large display version: the original when the browser can
// show it, otherwise a cached JPEG rendering.
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	err := s.withFile(r, func(fsys fs.FS, f store.ImageFile) error {
		w.Header().Set("Cache-Control", "private, max-age=86400")
		if media.BrowserNative(f.Format) && f.Rotate == 0 {
			if _, err := fs.Stat(fsys, f.Path); err != nil {
				return err
			}
			http.ServeFileFS(w, r, fsys, f.Path)
			return nil
		}
		name := f.Hash + ".jpg"
		if f.Rotate != 0 {
			name = fmt.Sprintf("%s_r%d.jpg", f.Hash, f.Rotate) // a correction: rendered turned
		}
		cached := filepath.Join(s.PreviewDir, f.Hash[:2], name)
		if _, err := os.Stat(cached); err != nil {
			data, err := fs.ReadFile(fsys, f.Path)
			if err != nil {
				return err
			}
			jpg, err := media.Render(f.Path, data, 2560, f.Rotate)
			if err != nil {
				httpError(w, http.StatusUnsupportedMediaType, err)
				return nil
			}
			_ = os.MkdirAll(filepath.Dir(cached), 0o755)
			if err := os.WriteFile(cached+".tmp", jpg, 0o644); err == nil {
				_ = os.Rename(cached+".tmp", cached)
			}
		}
		http.ServeFile(w, r, cached)
		return nil
	})
	if err != nil {
		s.fileError(w, r, err)
	}
}

// --- API ---

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Stats(r.Context())
	if err != nil {
		httpError(w, 500, err)
		return
	}
	out := map[string]any{"library": st, "scan": s.Scanner.Status(), "issues": s.issueCount(), "discover": s.discoverCount()}
	if s.Jobs != nil {
		out["jobs"] = s.Jobs()
	}
	writeJSON(w, out)
}

func (s *Server) listFolders(w http.ResponseWriter, r *http.Request) {
	fl, err := s.Store.Folders(r.Context())
	if err != nil {
		httpError(w, 500, err)
		return
	}
	// Views never include the stored password.
	out := make([]store.FolderView, 0, len(fl))
	for _, f := range fl {
		out = append(out, f.View())
	}
	writeJSON(w, out)
}

type locationReq struct {
	Location string `json:"location"`
	Password string `json:"password"`
}

// spec parses the request location. Without a password it reuses the one
// stored for another folder on the same server and account, so a second
// folder on a NAS doesn't need the password typed again.
func (s *Server) spec(ctx context.Context, req locationReq) (source.Spec, error) {
	sp, err := source.Parse(req.Location)
	if err != nil {
		return sp, err
	}
	if req.Password != "" {
		sp.Password = req.Password
	}
	if sp.Password == "" && sp.Scheme != "file" {
		folders, _ := s.Store.Folders(ctx)
		for _, f := range folders {
			if o, err := source.Parse(f.Location); err == nil && o.Scheme == sp.Scheme &&
				strings.EqualFold(o.Host, sp.Host) && o.User == sp.User {
				sp.Password = f.Secret
				break
			}
		}
	}
	return sp, nil
}

func (s *Server) addFolder(w http.ResponseWriter, r *http.Request) {
	var req locationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	sp, err := s.spec(r.Context(), req)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	// Check it opens before saving, so typos and bad passwords surface now.
	c, err := source.Open(r.Context(), sp)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	c.Close()
	// Refuse nesting: a folder inside another would index files twice.
	existing, _ := s.Store.Folders(r.Context())
	for _, f := range existing {
		if o, err := source.Parse(f.Location); err == nil && (source.Within(sp, o) || source.Within(o, sp)) {
			httpError(w, 409, errors.New("overlaps existing folder "+f.Location))
			return
		}
	}
	id, err := s.Store.AddFolder(r.Context(), sp.Location(), sp.Password)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	go s.Scanner.ScanAll(s.scanCtx)
	writeJSON(w, map[string]int64{"id": id})
}

func (s *Server) removeFolder(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	s.Pool.Drop(id)
	if err := s.Store.RemoveFolder(r.Context(), id); err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) controlJob(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	if s.Control == nil || (action != "pause" && action != "resume") ||
		!s.Control(r.PathValue("stage"), action == "pause") {
		http.NotFound(w, r)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) startScan(w http.ResponseWriter, r *http.Request) {
	go s.Scanner.ScanAll(s.scanCtx)
	writeJSON(w, map[string]bool{"ok": true})
}

// browse lists subdirectories (or SMB shares) so folders can be picked in the UI.
func (s *Server) browse(w http.ResponseWriter, r *http.Request) {
	var req locationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	sp, err := s.spec(r.Context(), req)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	l, err := source.Browse(r.Context(), sp)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	writeJSON(w, l)
}

// parseFilter builds a filter from query parameters. Years in the search
// text ("2026", "2000-2010") become a date range; see extractDates.
func (s *Server) parseFilter(r *http.Request) store.Filter {
	f, _ := s.parseQuery(r)
	return f
}

// parsedQuery is what was recognised in the search text, for the UI.
type parsedQuery struct {
	Range   *DateRange
	People  []PersonName
	Special []string // "known", "one-unknown", "alone", "nsfw", "blurry", "picture", ...
}

func (s *Server) parseQuery(r *http.Request) (store.Filter, parsedQuery) {
	q := r.URL.Query()
	f := store.Filter{
		Text:     q.Get("q"),
		Sort:     q.Get("sort"),
		Favorite: q.Get("fav") == "1",
	}
	f.FolderID, _ = strconv.ParseInt(q.Get("folder"), 10, 64)
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	// restrict narrows f.IDs to ids; several restrictions intersect.
	restrict := func(ids []int64) {
		if ids == nil {
			ids = []int64{}
		}
		if f.IDs == nil {
			f.IDs = ids
			return
		}
		keep := make(map[int64]bool, len(ids))
		for _, id := range ids {
			keep[id] = true
		}
		f.IDs = slices.DeleteFunc(f.IDs, func(id int64) bool { return !keep[id] })
	}
	if aid, err := strconv.ParseInt(q.Get("album"), 10, 64); err == nil && aid > 0 {
		restrict(s.Store.AlbumImageIDs(aid))
	}
	if pid, err := strconv.ParseInt(q.Get("person"), 10, 64); err == nil && pid > 0 {
		restrict(s.Store.PersonImageIDs(pid))
	}
	if ids := q.Get("ids"); ids != "" {
		var list []int64
		for p := range strings.SplitSeq(ids, ",") {
			if id, err := strconv.ParseInt(p, 10, 64); err == nil {
				list = append(list, id)
			}
		}
		restrict(list)
	}
	if t, err := time.Parse("2006-01", q.Get("month")); err == nil {
		end := t.AddDate(0, 1, 0)
		f.From, f.To = &t, &end
	}
	var pq parsedQuery
	f.Text, pq.Range = extractDates(f.Text)
	if dr := pq.Range; dr != nil {
		// Intersect with any explicit month.
		if f.From == nil || dr.From.After(*f.From) {
			f.From = &dr.From
		}
		if f.To == nil || dr.To.Before(*f.To) {
			f.To = &dr.To
		}
	}
	// People picked in the search box (people=1,2), and names typed in
	// the text, mean "photos with all of them".
	names := s.personNames()
	// $known, one($unknown), $nsfw, ..., typed or picked as chips.
	f.Text, pq.Special = extractSpecial(f.Text)
	for v := range strings.SplitSeq(q.Get("people"), ",") {
		if isSpecial(v) && !slices.Contains(pq.Special, v) {
			pq.Special = append(pq.Special, v)
		}
	}
	pictures := slices.Contains(pq.Special, "picture")
	for _, v := range pq.Special {
		switch v {
		case "picture":
			// With people, pictures of them (below); alone, of anyone.
		case "alone":
			restrict(s.Store.AloneImageIDs())
		case "nsfw":
			f.OnlyNsfw = true
		case "blurry":
			f.Blurry = true
		default:
			if kind, count, ok := faceFilter(v); ok {
				restrict(s.Store.FaceImageIDs(kind, count))
			}
		}
	}
	// Photos not safe for work are left out unless asked for ($nsfw) or
	// shown in Settings.
	f.HideNsfw = !s.Store.Settings().ShowNsfw
	for v := range strings.SplitSeq(q.Get("people"), ",") {
		id, err := strconv.ParseInt(v, 10, 64)
		if i := slices.IndexFunc(names, func(p PersonName) bool { return p.ID == id }); err == nil && i >= 0 {
			pq.People = append(pq.People, names[i])
		}
	}
	var typed []PersonName
	f.Text, typed = extractPeople(f.Text, names)
	for _, p := range typed {
		if !slices.ContainsFunc(pq.People, func(o PersonName) bool { return o.ID == p.ID }) {
			pq.People = append(pq.People, p)
		}
	}
	for _, p := range pq.People {
		if pictures {
			restrict(s.Store.PictureImageIDs(p.ID)) // a picture of them, not them there
		} else {
			restrict(s.Store.PersonImageIDs(p.ID))
		}
	}
	if pictures && len(pq.People) == 0 {
		restrict(s.Store.PictureImageIDs(0))
	}
	return f, pq
}

func (s *Server) personNames() []PersonName {
	var out []PersonName
	for _, p := range s.Store.People(context.Background()) {
		out = append(out, PersonName{p.ID, p.Name})
	}
	return out
}

func (s *Server) listImages(w http.ResponseWriter, r *http.Request) {
	f, pq := s.parseQuery(r)
	resp := map[string]any{"text": f.Text, "people": recognised(pq.People), "special": nonNil(pq.Special)}
	if pq.Range != nil {
		resp["range"] = pq.Range.Label
	}
	var (
		items any
		total int
		err   error
	)
	if strings.TrimSpace(f.Text) != "" && s.Embed != nil {
		// Semantic search: rank by similarity between the query and each
		// photo's EmbeddingGemma vector. Falls back to plain text matching
		// if the embedding server is unavailable.
		if q, qerr := s.Embed.Query(r.Context(), f.Text); qerr == nil {
			items, total, err = s.Store.Search(r.Context(), q, f)
			resp["semantic"] = true
		} else {
			s.Log.Warn("semantic search unavailable; using text match", "err", qerr)
		}
	}
	if items == nil && err == nil {
		items, total, err = s.Store.Images(r.Context(), f)
	}
	if err != nil {
		httpError(w, 500, err)
		return
	}
	resp["items"], resp["total"] = items, total
	writeJSON(w, resp)
}

// recognised lists people found in a query for the UI's chips.
func recognised(ps []PersonName) []map[string]any {
	out := []map[string]any{}
	for _, p := range ps {
		out = append(out, map[string]any{"id": p.ID, "name": p.Name})
	}
	return out
}

// rotateImage turns a photo whose camera recorded the wrong orientation
// ({"by": 90 or -90}); it is then read and analysed again upright.
func (s *Server) rotateImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		By int `json:"by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || (req.By != 90 && req.By != -90 && req.By != 180) {
		httpError(w, 400, fmt.Errorf("by must be 90, -90 or 180"))
		return
	}
	rot, err := s.Store.RotateImage(r.Context(), id, req.By)
	if err != nil {
		httpError(w, 404, err)
		return
	}
	if s.WakeImport != nil {
		s.WakeImport() // draw it again and find its faces upright now, not at the next round
	}
	writeJSON(w, map[string]int{"rot": rot})
}

func (s *Server) getImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	d, err := s.Store.Image(r.Context(), id)
	if err != nil {
		httpError(w, 404, err)
		return
	}
	writeJSON(w, d)
}

// aroundImage lists the photos taken just before and after one.
func (s *Server) aroundImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	a, err := s.Store.Around(r.Context(), id, min(max(n, 1), 200))
	if err != nil {
		httpError(w, 404, err)
		return
	}
	writeJSON(w, a)
}

func (s *Server) editImage(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var e store.UserEdit
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		httpError(w, 400, err)
		return
	}
	if err := s.Store.EditImage(r.Context(), id, e); err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) geo(w http.ResponseWriter, r *http.Request) {
	f := s.parseFilter(r)
	if s.Embed != nil {
		// Smart search ranks by meaning rather than filtering, so a map
		// can't show "relevant" photos; keep only the hard filters
		// (people, years, favorites).
		f.Text = ""
	}
	pts, err := s.Store.GeoPoints(r.Context(), f)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, pts)
}

func (s *Server) timeline(w http.ResponseWriter, r *http.Request) {
	tl, err := s.Store.Timeline(r.Context(), s.parseFilter(r))
	if err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, tl)
}
