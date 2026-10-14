package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/lkarlslund/imageorganizer/internal/analysis"
	"github.com/lkarlslund/imageorganizer/internal/faces"
	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/pipeline"
	"github.com/lkarlslund/imageorganizer/internal/source"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// maxFileSize guards against reading huge non-photo files into memory.
const maxFileSize = 512 << 20

// Importer turns pending files into images: it reads each file once, hashes
// it, links it to a known image (moves, copies) or extracts metadata and a
// thumbnail for a new one.
type Importer struct {
	Store  *store.Store
	Pool   *source.Pool
	Thumbs *ThumbCache
	Faces  *faces.Engine // nil disables face analysis
	// FaceCrops stores small face images for the People view.
	FaceCrops *ThumbCache
	Log       *slog.Logger
	// Embed computes the search embedding from the fresh thumbnail while it
	// is in memory, asynchronously so the worker doesn't wait on the
	// embedding server: at most EmbedSlots requests are in flight, and when
	// all are busy (or EmbedReady says the server is down) the embed stage
	// backfills later. Optional.
	Embed      func(ctx context.Context, jpeg []byte) ([]float32, error)
	EmbedReady func() bool
	EmbedSlots int

	embedOnce sync.Once
	embedSem  chan struct{}
	inflight  sync.WaitGroup
	// OnImage is called for each image that was created or changed, to wake
	// downstream stages (embeddings, faces).
	OnImage func(imageID int64)
}

// Stage returns the pipeline stage for importing.
func (im *Importer) Stage(workers int) pipeline.Stage[store.FileJob] {
	var since time.Time
	return pipeline.Stage[store.FileJob]{
		Name:    "import",
		Workers: workers,
		Next: func(ctx context.Context, n int, skip map[int64]bool) []store.FileJob {
			if since.IsZero() {
				since = time.Now()
			}
			return im.Store.PendingFiles(ctx, n, skip)
		},
		Key:       func(j store.FileJob) int64 { return j.FileID },
		Do:        im.importFile,
		Transient: isNetErr,
		Drained: func(ctx context.Context) {
			// Files moved *and* edited across folders can only be matched
			// once every folder's missing files are known.
			if n, err := im.Store.RelinkNew(ctx, since.Add(-time.Minute).Unix()); err != nil {
				im.Log.Error("relink", "err", err)
			} else if n > 0 {
				im.Log.Info("relinked moved+edited photos", "count", n)
			}
			since = time.Time{}
		},
		Log: im.Log,
	}
}

func (im *Importer) importFile(ctx context.Context, j store.FileJob) error {
	video := media.IsVideo(j.Path)
	if j.Size > maxFileSize && !video { // a video is read in pieces
		return fmt.Errorf("%s: file too large (%d bytes)", j.Path, j.Size)
	}
	folder, err := im.Store.Folder(ctx, j.FolderID)
	if err != nil {
		return err
	}
	spec, err := Spec(folder)
	if err != nil {
		return err
	}
	var (
		data []byte
		hash string
		size int64
	)
	if err := im.Pool.Do(ctx, folder.ID, spec, func(fsys fs.FS) error {
		if video {
			f, err := fsys.Open(j.Path)
			if err != nil {
				return err
			}
			defer f.Close()
			hash, size, err = media.HashReader(f)
			return err
		}
		data, err = fs.ReadFile(fsys, j.Path)
		return err
	}); err != nil {
		return err
	}
	if !video {
		hash, size = fmt.Sprintf("%016x%08x", xxhash.Sum64(data), len(data)), int64(len(data))
	}
	ref := store.FileRef{FolderID: j.FolderID, Path: j.Path, Size: j.Size, MTime: j.MTime}

	// Known content (a move, rename or copy): link it without decoding
	// again, unless it still needs (re)analysis.
	if ok, _ := im.Store.HashKnown(ctx, hash); ok && !im.Store.NeedsOriginal(hash, im.Faces != nil) {
		_, _, err := im.Store.RecordFile(ctx, ref, store.ImageData{ContentHash: hash})
		return err
	}

	rot := im.Store.Rotation(hash) // a correction the user (or an earlier import) made
	var (
		r    media.Result
		perr error
	)
	switch local, ok := source.LocalPath(spec, j.Path); {
	case !video:
		r, perr = media.Process(j.Path, data, im.Faces != nil, rot)
	case ok:
		r, perr = media.ProcessVideo(ctx, local, im.Faces != nil, rot)
	default:
		perr = errors.New("videos are only read from folders on this computer")
		r.Info.Format = strings.TrimPrefix(strings.ToLower(path.Ext(j.Path)), ".")
	}
	if perr != nil {
		// Still index it (metadata may be fine); just without a thumbnail.
		im.Log.Debug("no thumbnail", "path", j.Path, "err", perr)
	}
	d := store.ImageData{
		ContentHash: hash, PHash: r.PHash, Width: r.Info.Width, Height: r.Info.Height,
		FileSize: size, Format: r.Info.Format, TakenAt: r.Info.TakenAt,
		Lat: r.Info.Lat, Lon: r.Info.Lon, CameraMake: r.Info.CameraMake, CameraModel: r.Info.CameraModel,
		Lens: r.Info.Lens, ISO: r.Info.ISO, FNumber: r.Info.FNumber, Exposure: r.Info.Exposure,
		Focal: r.Info.Focal, Rotated: rot, Video: video, Duration: r.Info.Duration,
	}
	if d.TakenAt == nil {
		// No EXIF date: fall back to the file's modification time.
		t := time.Unix(j.MTime, 0)
		d.TakenAt = &t
	}
	d.Analysis = analysis.Versions{analysis.CMedia: analysis.Media}
	var found []faces.Face
	if im.Faces != nil && r.Work != nil {
		if found, err = im.Faces.Analyze(r.Work); err != nil {
			return fmt.Errorf("faces: %w", err)
		}
		// The camera may have recorded the wrong orientation: when every
		// face lies on its side, try the photo turned upright and keep it
		// if faces are found at least as well.
		if b := r.Work.Bounds(); rot == 0 && !video { // a video isn't turned by its faces
			if deg := faces.Sideways(found, b.Dx(), b.Dy()); deg != 0 {
				turned := media.RotateCW(r.Work, deg)
				again, err := im.Faces.Analyze(turned)
				if err != nil {
					return fmt.Errorf("faces: %w", err)
				}
				if confident(again) >= confident(found) {
					im.Log.Info("turned a photo whose faces were on their side", "path", j.Path, "degrees", deg)
					found, r.Work, d.AutoRotate = again, turned, deg
					if deg != 180 {
						d.Width, d.Height = d.Height, d.Width
					}
					if r.Thumb, d.PHash, err = media.Thumbnail(turned); err != nil {
						return err
					}
				}
			}
		}
	}
	if r.Thumb != nil {
		if err := im.Thumbs.Put(hash, r.Thumb); err != nil {
			return err
		}
		d.HasThumb = true
		d.ThumbRot = (rot + d.AutoRotate) % 360 // as drawn
	}
	if im.Faces != nil && r.Work != nil {
		for i, f := range found {
			if f.Crop != nil {
				if err := im.FaceCrops.Put(faces.CropKey(hash, i), f.Crop); err != nil {
					return err
				}
			}
			fd := store.FaceData{Box: f.Box, Landmarks: f.Landmarks, Score: f.Score, Embedding: f.Embedding, CropBox: f.CropBox}
			if f.AgeDone {
				fd.Age, fd.AgeVersion, fd.Female = f.Age, analysis.AgeFull, f.Female
			}
			d.Faces = append(d.Faces, fd)
		}
		d.Analysis[analysis.CFaces] = analysis.Faces
	}
	id, _, err := im.Store.RecordFile(ctx, ref, d)
	if err != nil {
		return err
	}
	if r.Thumb != nil {
		im.embedAsync(ctx, id, hash, r.Thumb)
	}
	if im.OnImage != nil {
		im.OnImage(id)
	}
	return nil
}

// embedAsync sends the thumbnail to the embedding server without making the
// import worker wait. The embedding server runs on another device, so this
// overlaps with the CPU work of the next files.
func (im *Importer) embedAsync(ctx context.Context, id int64, hash string, thumb []byte) {
	if im.Embed == nil || (im.EmbedReady != nil && !im.EmbedReady()) {
		return
	}
	im.embedOnce.Do(func() { im.embedSem = make(chan struct{}, max(1, im.EmbedSlots)) })
	select {
	case im.embedSem <- struct{}{}:
	default:
		return // all slots busy: the embed stage picks it up
	}
	im.inflight.Go(func() {
		defer func() { <-im.embedSem }()
		ectx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		v, err := im.Embed(ectx, thumb)
		if err != nil {
			im.Log.Debug("inline embedding failed; embed stage will retry", "id", id, "err", err)
			return
		}
		if err := im.Store.SetVector(ctx, id, hash, v); err != nil && ctx.Err() == nil {
			im.Log.Warn("store embedding", "id", id, "err", err)
		}
	})
}

// Wait blocks until in-flight background embeddings have finished.
func (im *Importer) Wait() { im.inflight.Wait() }

// isNetErr reports errors meaning the source is unreachable (NAS down,
// session dropped) rather than a problem with the file itself.
func isNetErr(err error) bool {
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return false
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	msg := err.Error()
	for _, m := range []string{"connection refused", "connection reset", "broken pipe", "EOF", "unavailable"} {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}

// confident counts faces the detector is fairly sure of.
func confident(fs []faces.Face) int {
	n := 0
	for _, f := range fs {
		if f.Score >= 0.6 {
			n++
		}
	}
	return n
}
