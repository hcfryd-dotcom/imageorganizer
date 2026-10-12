// Command imageorganizer indexes photo folders and serves a web UI to browse them.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/embed"
	"github.com/lkarlslund/imageorganizer/internal/faces"
	"github.com/lkarlslund/imageorganizer/internal/pipeline"
	"github.com/lkarlslund/imageorganizer/internal/scanner"
	"github.com/lkarlslund/imageorganizer/internal/server"
	"github.com/lkarlslund/imageorganizer/internal/source"
	"github.com/lkarlslund/imageorganizer/internal/store"
	"github.com/lkarlslund/imageorganizer/web"
)

// restart is set when the server was asked to restart (POST
// /debug/restart): it exits with server.RestartCode once shut down.
var restart atomic.Bool

// shutdownGrace bounds how long in-flight work may take after a signal.
// The dev supervisor waits longer than this before resorting to SIGKILL.
const shutdownGrace = 8 * time.Second

// config is how the program runs: flags, filled in from the saved
// settings for anything not given on the command line.
type config struct {
	listen, dataDir, dev, paused string
	embedURL, embedModel         string
	workers, embedWorkers, conns int
	rescan                       time.Duration
	noScan, startOver            bool
	faceModels, ortLib, faceDev  string
	faceGPUMem                   int64
}

// settingFlags maps settings to the flags that override them.
var settingFlags = map[string]string{
	"embed_url": "embed-url", "embed_model": "embed-model", "embed_workers": "embed-workers",
	"face_models": "face-models", "onnxruntime": "onnxruntime", "face_device": "face-device",
	"face_gpu_mem_mb": "face-gpu-mem", "workers": "workers", "conns": "conns", "rescan_minutes": "rescan",
}

// withSettings fills in c from saved settings where no flag was given.
// An embedding URL of "off" disables search by meaning.
func (c config) withSettings(s store.Settings, explicit map[string]bool) config {
	set := func(flag string, ok bool, apply func()) {
		if ok && !explicit[flag] {
			apply()
		}
	}
	set("embed-url", s.EmbedURL != "", func() {
		c.embedURL = s.EmbedURL
		if s.EmbedURL == "off" {
			c.embedURL = ""
		}
	})
	set("embed-model", s.EmbedModel != "", func() { c.embedModel = s.EmbedModel })
	set("embed-workers", s.EmbedWorkers > 0, func() { c.embedWorkers = s.EmbedWorkers })
	set("face-models", s.FaceModels != "", func() { c.faceModels = s.FaceModels })
	set("onnxruntime", s.ONNXRuntime != "", func() { c.ortLib = s.ONNXRuntime })
	set("face-device", s.FaceDevice != "", func() { c.faceDev = s.FaceDevice })
	set("face-gpu-mem", s.FaceGPUMemMB > 0, func() { c.faceGPUMem = int64(s.FaceGPUMemMB) << 20 })
	set("workers", s.Workers > 0, func() { c.workers = s.Workers })
	set("conns", s.Conns > 0, func() { c.conns = s.Conns })
	set("rescan", s.RescanMinutes != 0, func() {
		c.rescan = time.Duration(max(0, s.RescanMinutes)) * time.Minute // -1: never
	})
	return c
}

// asSettings shows a config as settings, for the Settings page.
func (c config) asSettings() store.Settings {
	rescan := int(c.rescan / time.Minute)
	if c.rescan <= 0 {
		rescan = -1
	}
	url := c.embedURL
	if url == "" {
		url = "off"
	}
	return store.Settings{
		EmbedURL: url, EmbedModel: c.embedModel, EmbedWorkers: c.embedWorkers,
		FaceModels: c.faceModels, ONNXRuntime: c.ortLib, FaceDevice: c.faceDev, FaceGPUMemMB: int(c.faceGPUMem >> 20),
		Workers: c.workers, Conns: c.conns, RescanMinutes: rescan,
	}
}

// needsRestart reports whether settings that only apply at start differ.
func (c config) needsRestart(running config) bool {
	return (c.embedURL == "") != (running.embedURL == "") || c.embedWorkers != running.embedWorkers ||
		c.workers != running.workers || c.conns != running.conns || c.rescan != running.rescan ||
		c.faceModels != running.faceModels || c.ortLib != running.ortLib || c.faceDev != running.faceDev ||
		c.faceGPUMem != running.faceGPUMem
}

func main() {
	home, _ := os.UserHomeDir()
	var c config
	flag.StringVar(&c.listen, "listen", "127.0.0.1:8420", "HTTP listen address")
	flag.StringVar(&c.dataDir, "data", filepath.Join(home, ".local", "share", "imageorganizer"), "database and cache directory")
	flag.StringVar(&c.dev, "dev", "", "serve UI assets from this directory and live-reload browsers on change")
	flag.IntVar(&c.workers, "workers", min(16, runtime.NumCPU()), "parallel imports (read, hash, thumbnail)")
	flag.DurationVar(&c.rescan, "rescan", 30*time.Minute, "interval between automatic rescans (0 disables)")
	flag.StringVar(&c.embedURL, "embed-url", "http://127.0.0.1:8095", "llama.cpp server running EmbeddingGemma 2 with --mmproj (empty disables search embeddings)")
	flag.StringVar(&c.embedModel, "embed-model", "", "model to ask the embedding server for (empty: whatever it has loaded)")
	flag.IntVar(&c.conns, "conns", 4, "connections per remote folder (SMB etc.)")
	flag.IntVar(&c.embedWorkers, "embed-workers", 8, "parallel embedding requests (match llama-server --parallel)")
	flag.StringVar(&c.faceModels, "face-models", "", "directory with scrfd_10g.onnx, arcface_w600k_r50.onnx, mivolo_v2_age.onnx (default <data>/models; empty files disable faces)")
	flag.StringVar(&c.ortLib, "onnxruntime", defaultORTLib(), "ONNX Runtime shared library (a GPU build enables -face-device cuda)")
	flag.StringVar(&c.faceDev, "face-device", "auto", "where face models run: auto, cuda or cpu")
	flag.Int64Var(&c.faceGPUMem, "face-gpu-mem", 1<<30, "GPU memory arena cap per face model in bytes")
	flag.StringVar(&c.paused, "paused", "", "comma-separated stages to start paused (e.g. import)")
	debug := flag.Bool("debug", false, "verbose logging")
	flag.BoolVar(&c.noScan, "no-scan", false, "don't scan folders at all (not at startup either): work with the library as it is")
	flag.BoolVar(&c.startOver, "start-over", false, "forget who is who (people, names, relationships) and group the faces afresh, keeping all analysis; refused on the default data directory")
	flag.Parse()
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	if err := run(log, c, explicit); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
	if restart.Load() {
		os.Exit(server.RestartCode) // asked to restart: the supervisor starts it again
	}
}

func run(log *slog.Logger, flags config, explicit map[string]bool) error {
	listen, dataDir, dev, paused := flags.listen, flags.dataDir, flags.dev, flags.paused
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		// First signal: graceful shutdown (ctx is cancelled). Second: give up.
		<-ctx.Done()
		log.Info("shutting down; send the signal again to exit immediately")
		again := make(chan os.Signal, 1)
		signal.Notify(again, os.Interrupt, syscall.SIGTERM)
		<-again
		log.Warn("second signal, exiting now")
		os.Exit(130)
	}()

	// The database holds passwords for remote folders: keep it private.
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return err
	}
	_ = os.Chmod(dataDir, 0o700)
	st, err := store.Open(filepath.Join(dataDir, "library.bolt"))
	if err != nil {
		return err
	}
	if flags.startOver {
		// Only on a copy: it forgets every naming decision for good.
		if home, _ := os.UserHomeDir(); filepath.Clean(dataDir) == filepath.Join(home, ".local", "share", "imageorganizer") {
			st.Close()
			return fmt.Errorf("-start-over refused on the default data directory %s: use it on a copy (-data)", dataDir)
		}
		if err := st.StartOver(ctx); err != nil {
			st.Close()
			return err
		}
	}
	// Saved settings fill in whatever the command line didn't set.
	saved := st.Settings()
	st.SetTuning(saved)
	cfg := flags.withSettings(saved, explicit)
	embedURL, embedModel, workers, embedWorkers, conns, rescan := cfg.embedURL, cfg.embedModel, cfg.workers, cfg.embedWorkers, cfg.conns, cfg.rescan
	faceModels, ortLib, faceDevice, faceGPUMem := cfg.faceModels, cfg.ortLib, cfg.faceDev, cfg.faceGPUMem
	thumbs, err := scanner.NewThumbCache(filepath.Join(dataDir, "thumbs"))
	if err != nil {
		st.Close()
		return err
	}
	faceCrops, err := scanner.NewThumbCache(filepath.Join(dataDir, "faces"))
	if err != nil {
		st.Close()
		return err
	}
	st.CropPath = faceCrops.Path // pictures are judged on their crops
	// Face analysis runs inside import. Without the models the app works,
	// just without people.
	if faceModels == "" {
		faceModels = filepath.Join(dataDir, "models")
	}
	fcfg := faces.DefaultConfig(faceModels)
	fcfg.Library, fcfg.Device, fcfg.GPUMemory = ortLib, faceDevice, faceGPUMem
	faceEngine, err := faces.Open(fcfg)
	var faceErr error // why face recognition is off, for the Settings page
	if err != nil {
		log.Warn("face analysis disabled", "err", err)
		faceEngine, faceErr = nil, err
	} else {
		log.Info("face analysis ready", "device", faceEngine.Device(), "gender", faceEngine.HasGender())
		defer faceEngine.Close()
		st.Gender = faceEngine.HasGender()   // faces without a gender estimate get one from the age stage
		st.NoAgeModel = !faceEngine.HasAge() // then faces are grouped without waiting for an age
	}
	// Images whose original-dependent analysis is out of date (a version in
	// package analysis was bumped) are read once more by import.
	if n, err := st.RequeueForAnalysis(ctx, faceEngine != nil); err != nil {
		log.Error("requeue for analysis", "err", err)
	} else if n > 0 {
		log.Info("queued images for re-analysis", "count", n)
	}

	pool := source.NewPool(conns)
	// File reads have no context: closing the pool on shutdown aborts
	// in-flight remote reads so imports don't hold up the exit.
	context.AfterFunc(ctx, pool.Close)

	// Background work. Every goroutine that touches the store is tracked so
	// the database is only closed once they have stopped.
	var bg sync.WaitGroup

	// Pipeline stages: each pulls its work from the store.
	var embedRunner *pipeline.Runner[store.VectorJob]
	var emb *embed.Client
	if embedURL != "" {
		emb = embed.New(embedURL, embedModel)
		embedRunner = pipeline.New(embed.Stage(emb, st, thumbs.Path, embedWorkers, log))
		bg.Go(func() { embedRunner.Run(ctx) })
	}
	importer := &scanner.Importer{Store: st, Pool: pool, Thumbs: thumbs, FaceCrops: faceCrops, Log: log}
	if faceEngine != nil {
		importer.Faces = faceEngine
	}
	if embedRunner != nil {
		importer.OnImage = func(int64) { embedRunner.Wake() }
		importer.Embed = emb.Image
		importer.EmbedSlots = embedWorkers
		importer.EmbedReady = func() bool { return embedRunner.Status().Online }
	}
	importRunner := pipeline.New(importer.Stage(workers))
	bg.Go(func() {
		importRunner.Run(ctx)
		importer.Wait() // background embeddings started by import
	})

	var ageRunner *pipeline.Runner[store.AgeJob]
	var peopleRunner *pipeline.Runner[int64]
	if faceEngine != nil {
		// People matching and clustering run one face at a time so clusters
		// grow consistently; it is cheap in-memory work.
		peopleRunner = pipeline.New(pipeline.Stage[int64]{
			Name: "people", Workers: 1, Batch: 256,
			Next: st.NeedPeople,
			Key:  func(id int64) int64 { return id },
			Do:   st.ProcessFace,
			Log:  log,
		})
		bg.Go(func() { peopleRunner.Run(ctx) })
		if faceEngine.HasAge() {
			ageStage := faces.AgeStage(faceEngine, st, faceCrops.Path, 8, log)
			ageStage.Progress = peopleRunner.Wake
			ageRunner = pipeline.New(ageStage)
			importer.OnImage = chain(importer.OnImage, func(int64) { ageRunner.Wake() })
			bg.Go(func() { ageRunner.Run(ctx) })
		} else {
			// Without an age model faces are grouped as soon as they are found.
			importer.OnImage = chain(importer.OnImage, func(int64) { peopleRunner.Wake() })
		}
	}
	// Looking at thumbnails: sharpness, and NSFW with the classifier.
	st.NsfwModel = faceEngine.HasNSFW()
	lookRunner := pipeline.New(faces.LookStage(faceEngine, st, thumbs.Path, 4, log))
	importer.OnImage = chain(importer.OnImage, func(int64) { lookRunner.Wake() })
	bg.Go(func() { lookRunner.Run(ctx) })

	control := func(stage string, pause bool) bool {
		set := func(p, r func()) bool {
			if pause {
				p()
			} else {
				r()
			}
			return true
		}
		switch {
		case stage == "import":
			return set(importRunner.Pause, importRunner.Resume)
		case stage == "embed" && embedRunner != nil:
			return set(embedRunner.Pause, embedRunner.Resume)
		case stage == "age" && ageRunner != nil:
			return set(ageRunner.Pause, ageRunner.Resume)
		case stage == "look":
			return set(lookRunner.Pause, lookRunner.Resume)
		}
		return false
	}
	for stage := range strings.SplitSeq(paused, ",") {
		if stage = strings.TrimSpace(stage); stage != "" && !control(stage, true) {
			return fmt.Errorf("-paused: unknown stage %q", stage)
		}
	}

	sc := scanner.New(st, log)
	sc.OnDiscovered = importRunner.Wake
	bg.Go(func() {
		if flags.noScan {
			return
		}
		sc.ScanAll(ctx)
		if rescan <= 0 {
			return
		}
		t := time.NewTicker(rescan)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				sc.ScanAll(ctx)
			}
		}
	})
	bg.Go(func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				pool.Reap(5 * time.Minute)
			}
		}
	})

	var webFS fs.FS = web.FS
	if dev != "" {
		webFS = os.DirFS(dev)
		log.Info("dev mode: serving UI from disk", "dir", dev)
	}
	// The Settings page: what is saved and in effect, what needs attention.
	defaults := flags.withSettings(store.Settings{}, map[string]bool{}) // flags' own defaults
	settingsState := func() server.SettingsState {
		saved := st.Settings()
		want := flags.withSettings(saved, explicit)
		eff := cfg.asSettings()
		eff.EmbedModel = want.embedModel
		if emb != nil {
			eff.EmbedURL = emb.URL()
		}
		eff.AutoFileOff, eff.AutoFileMin, eff.ReviewMin, eff.MapTiles = saved.AutoFileOff, saved.AutoFileMin, st.ReviewMin(), saved.MapTiles
		def := defaults.asSettings()
		def.AutoFileMin, def.ReviewMin = 0.6, 2
		state := server.SettingsState{
			Saved: saved, Effective: eff, Defaults: def, Locked: map[string]string{},
			RestartNeeded: want.needsRestart(cfg), DataDir: dataDir, Issues: []server.Issue{},
		}
		for setting, fl := range settingFlags {
			if explicit[fl] {
				state.Locked[setting] = "-" + fl
			}
		}
		switch {
		case emb == nil:
			state.Issues = append(state.Issues, server.Issue{Level: "warning", Text: "Search by meaning is off: no embedding server is set."})
		case embedRunner != nil && !embedRunner.Status().Online:
			text := "Can't reach the embedding server at " + emb.URL() + "."
			if e := embedRunner.Status().LastError; e != "" {
				text += " " + e
			}
			state.Issues = append(state.Issues, server.Issue{Level: "error", Text: text})
		}
		if faceErr != nil {
			state.Issues = append(state.Issues, server.Issue{Level: "error", Text: "Face recognition is off: " + faceErr.Error()})
		}
		if state.RestartNeeded {
			state.Issues = append(state.Issues, server.Issue{Level: "info", Text: "Some changes take effect after a restart."})
		}
		return state
	}
	settingsHooks := &server.SettingsHooks{
		State: settingsState,
		Save: func(s store.Settings) error {
			if err := st.SaveSettings(s); err != nil {
				return err
			}
			// The embedding server and model change at once.
			if want := flags.withSettings(s, explicit); emb != nil && want.embedURL != "" {
				emb.Configure(want.embedURL, want.embedModel)
				if embedRunner != nil {
					embedRunner.Wake()
				}
			}
			return nil
		},
		Restart: func() {
			log.Info("restarting from the Settings page")
			time.Sleep(300 * time.Millisecond) // let the response go out
			stop()
		},
	}

	srv := server.New(ctx, server.Config{
		Store: st, Scanner: sc, Thumbs: thumbs, FaceCrops: faceCrops,
		Pool: pool, Embed: emb, Control: control, Settings: settingsHooks,
		Faces: faceEngine, WakeImport: importRunner.Wake,
		Jobs: func() map[string]any {
			j := map[string]any{"import": importRunner.Status(), "pending": st.PendingCount()}
			if faceEngine != nil {
				j["faces"] = st.FaceStats()
				j["review"] = st.ReviewCount(st.ReviewMin())
			}
			if ageRunner != nil {
				j["age"] = ageRunner.Status()
				j["aged"], j["ageable"] = st.AgeStats()
			}
			if peopleRunner != nil {
				j["people"] = peopleRunner.Status()
			}
			if embedRunner != nil {
				done, total := st.VectorStats()
				j["embed"] = embedRunner.Status()
				j["embedded"], j["embeddable"] = done, total
			}
			j["look"] = lookRunner.Status()
			j["look_done"], j["look_total"], j["nsfw_marked"], j["blurry"] = st.LookStats()
			return j
		},
		PreviewDir: filepath.Join(dataDir, "previews"),
		Web:        webFS, Dev: dev != "",
		Restart: func() { restart.Store(true); stop() }, // shuts down as on a signal
		BuildID: strconv.FormatInt(time.Now().UnixNano(), 36),
		Log:     log,
	})

	hs := &http.Server{Addr: listen, Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", listen)
		served <- hs.ListenAndServe()
	}()

	select {
	case err := <-served: // failed to listen
		stop()
		bg.Wait()
		st.Close()
		return err
	case <-ctx.Done():
	}

	// Graceful shutdown: stop accepting requests (event streams end on ctx),
	// let background work finish its current item, then close the database.
	deadline := time.Now().Add(shutdownGrace)
	sctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	if err := hs.Shutdown(sctx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		log.Warn("http shutdown", "err", err)
	}
	done := make(chan struct{})
	go func() { bg.Wait(); close(done) }()
	select {
	case <-done:
		if err := st.Close(); err != nil {
			return err
		}
		log.Info("shutdown complete")
	case <-time.After(time.Until(deadline)):
		// bbolt is crash-safe; exiting with work in flight loses nothing
		// that a rescan won't redo.
		log.Warn("background work did not stop in time; exiting anyway")
	}
	return nil
}

// chain combines two optional callbacks.
func chain(a, b func(int64)) func(int64) {
	if a == nil {
		return b
	}
	return func(id int64) { a(id); b(id) }
}

// defaultORTLib is where ONNX Runtime is looked for unless configured: the
// system library on Linux, onnxruntime.dll next to the program on Windows.
func defaultORTLib() string {
	if runtime.GOOS == "windows" {
		if exe, err := os.Executable(); err == nil {
			return filepath.Join(filepath.Dir(exe), "onnxruntime.dll")
		}
		return "onnxruntime.dll"
	}
	return "/usr/lib/libonnxruntime.so"
}
