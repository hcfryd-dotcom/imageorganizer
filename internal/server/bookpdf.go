package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/places"
)

// Print files for a print shop: a photo book (web/book.html) printed to
// PDF by Microsoft Edge (or Chrome) without a window, as the page itself
// prints it: the pages inside (book.html?pdf=inner) and the cover in one
// piece with its spine (pdf=cover). The browser on the machine is used,
// so nothing has to be installed.

type pdfJob struct {
	Running bool     `json:"running"`
	Step    string   `json:"step,omitempty"` // "inner" or "cover" while running
	Files   []string `json:"files,omitempty"`
	Error   string   `json:"error,omitempty"`
	Dir     string   `json:"dir,omitempty"`
}

var (
	pdfMu   sync.Mutex
	pdfJobs = map[int64]*pdfJob{}
)

func (s *Server) bookPDFRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/books/pdf-dir", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"dir": s.defaultBookDir(r.Context())})
	})
	mux.HandleFunc("GET /api/books/{id}/pdf", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		pdfMu.Lock()
		j := pdfJobs[id]
		var out pdfJob
		if j != nil {
			out = *j
		}
		pdfMu.Unlock()
		writeJSON(w, out)
	})
	mux.HandleFunc("POST /api/books/{id}/pdf", s.startBookPDF)
	mux.HandleFunc("POST /api/open-folder", s.openFolder)
}

// defaultBookDir is where print files go unless told: a folder
// "Fotobøger" on the drive of the first library folder (the photos'
// drive has room; the system drive may not).
func (s *Server) defaultBookDir(ctx context.Context) string {
	if fl, err := s.Store.Folders(ctx); err == nil {
		for _, f := range fl {
			if v := filepath.VolumeName(f.Location); v != "" {
				return v + string(filepath.Separator) + "Fotobøger"
			}
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Fotobøger")
}

// browserPath finds Edge, else Chrome.
func browserPath() (string, error) {
	var tries []string
	if runtime.GOOS == "windows" {
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
			if d := os.Getenv(env); d != "" {
				tries = append(tries, filepath.Join(d, `Microsoft\Edge\Application\msedge.exe`), filepath.Join(d, `Google\Chrome\Application\chrome.exe`))
			}
		}
	}
	for _, n := range []string{"msedge", "microsoft-edge", "google-chrome", "chromium", "chrome"} {
		if p, err := exec.LookPath(n); err == nil {
			tries = append(tries, p)
		}
	}
	for _, p := range tries {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("neither Microsoft Edge nor Google Chrome was found, which the program prints PDFs with")
}

func (s *Server) startBookPDF(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	var req struct {
		Dir   string `json:"dir"`
		Cover bool   `json:"cover"` // also the cover in one piece
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	bk, err := s.Store.GetBook(r.Context(), id)
	if err != nil {
		storeError(w, err)
		return
	}
	dir := strings.TrimSpace(req.Dir)
	if dir == "" {
		dir = s.defaultBookDir(r.Context())
	}
	if !filepath.IsAbs(dir) {
		httpError(w, 400, errors.New("give the whole path of a folder"))
		return
	}
	browser, err := browserPath()
	if err != nil {
		httpError(w, 400, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		httpError(w, 400, err)
		return
	}
	pdfMu.Lock()
	if j := pdfJobs[id]; j != nil && j.Running {
		pdfMu.Unlock()
		httpError(w, http.StatusConflict, errors.New("the print files are being made already"))
		return
	}
	job := &pdfJob{Running: true, Dir: dir}
	pdfJobs[id] = job
	pdfMu.Unlock()

	name := places.SafeName(bk.Title)
	if name == "" {
		name = fmt.Sprintf("Fotobog %d", id)
	}
	base := "http://" + r.Host + "/static/book.html?book=" + fmt.Sprint(id) + "&pdf="
	steps := []string{"inner"}
	if req.Cover {
		steps = append(steps, "cover")
	}
	go func() {
		var files []string
		var failed error
		for _, step := range steps {
			pdfMu.Lock()
			job.Step = step
			pdfMu.Unlock()
			word := map[string]string{"inner": "indmad", "cover": "omslag"}[step]
			out := filepath.Join(dir, name+" - "+word+".pdf")
			if err := s.printPDF(browser, base+step, out); err != nil {
				failed = fmt.Errorf("%s: %w", word, err)
				break
			}
			files = append(files, out)
		}
		pdfMu.Lock()
		job.Running, job.Step, job.Files = false, "", files
		if failed != nil {
			job.Error = failed.Error()
		}
		pdfMu.Unlock()
	}()
	writeJSON(w, job)
}

// printPDF has the browser print a page to a PDF file. The page needs a
// while to fetch its photos; the virtual time budget lets it, without
// waiting that long in real time when it is done sooner.
func (s *Server) printPDF(browser, url, out string) error {
	// A browser profile of its own, thrown away after: the user's own
	// browser is not touched, and a profile grows to many megabytes. It
	// lies next to the PDF (the photos' drive), not on the system drive.
	profile, err := os.MkdirTemp(filepath.Dir(out), ".print-browser-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(profile)
	tmp := out + ".tmp.pdf"
	_ = os.Remove(tmp)
	ctx, cancel := context.WithTimeout(s.scanCtx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, browser, "--headless=new", "--disable-gpu", "--no-first-run", "--hide-scrollbars",
		"--no-pdf-header-footer", "--disable-extensions", "--disable-sync", "--no-default-browser-check",
		"--user-data-dir="+profile, "--virtual-time-budget=120000",
		"--print-to-pdf="+tmp, url)
	if msg, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(msg)))
	}
	if fi, err := os.Stat(tmp); err != nil || fi.Size() == 0 {
		return errors.New("the browser made no PDF")
	}
	_ = os.Remove(out)
	return os.Rename(tmp, out)
}

// openFolder shows a folder in Explorer (on Windows), for the files just
// made there.
func (s *Server) openFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dir string `json:"dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if fi, err := os.Stat(req.Dir); err != nil || !fi.IsDir() {
		httpError(w, 400, errors.New("no such folder"))
		return
	}
	if runtime.GOOS != "windows" {
		httpError(w, 400, errors.New("only on Windows"))
		return
	}
	if err := exec.Command("explorer.exe", req.Dir).Start(); err != nil {
		httpError(w, 500, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
