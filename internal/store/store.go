// Package store persists the library in bbolt and serves queries from
// in-memory indexes.
//
// Images are identified by content, not by path: an image record holds all
// metadata (EXIF and user edits) and one or more file records point at it.
// When a file moves, the scanner links the new path to the existing image,
// so nothing the user has done to it is lost.
//
// bbolt is the durable source of truth (one bucket per record type, keyed by
// big-endian id, JSON values). The whole library is loaded into memory at
// startup; a personal photo library is small enough that sorting and
// filtering in memory takes milliseconds and needs no on-disk indexes.
package store

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/analysis"
	bolt "go.etcd.io/bbolt"
)

var (
	bucketFolders = []byte("folders")
	bucketImages  = []byte("images")
	bucketFiles   = []byte("files")
)

// ErrNotFound is returned for unknown ids.
var ErrNotFound = errors.New("not found")

// Folder is a library root on any source (see package source).
type Folder struct {
	ID       int64  `json:"id"`
	Location string `json:"location"`
	Secret   string `json:"secret,omitempty"` // password for remote locations; never sent to the UI
	AddedAt  int64  `json:"added_at"`
	LastScan *int64 `json:"last_scan"`
	Images   int64  `json:"-"`
}

// FolderView is the API representation: no secret, with an image count.
type FolderView struct {
	ID       int64  `json:"id"`
	Location string `json:"location"`
	AddedAt  int64  `json:"added_at"`
	LastScan *int64 `json:"last_scan"`
	Images   int64  `json:"images"`
}

func (f Folder) View() FolderView {
	return FolderView{ID: f.ID, Location: f.Location, AddedAt: f.AddedAt, LastScan: f.LastScan, Images: f.Images}
}

// imageRec is the persisted image record.
type imageRec struct {
	ID          int64    `json:"id"`
	Hash        string   `json:"hash"`
	PHash       uint64   `json:"phash"`
	Width       int      `json:"w"`
	Height      int      `json:"h"`
	FileSize    int64    `json:"size"`
	Format      string   `json:"format"`
	TakenAt     *int64   `json:"taken,omitempty"`
	Lat         *float64 `json:"lat,omitempty"`
	Lon         *float64 `json:"lon,omitempty"`
	CameraMake  string   `json:"make,omitempty"`
	CameraModel string   `json:"model,omitempty"`
	Lens        string   `json:"lens,omitempty"`
	ISO         int      `json:"iso,omitempty"`
	FNumber     float64  `json:"fnumber,omitempty"`
	Exposure    string   `json:"exposure,omitempty"`
	Focal       float64  `json:"focal,omitempty"`
	HasThumb    bool     `json:"thumb,omitempty"`
	Video       bool     `json:"video,omitempty"` // a video file; its thumbnail is a frame
	Duration    float32  `json:"dur,omitempty"`   // a video's length in seconds
	// Analysis records which version of each analysis component produced
	// this image's data (see package analysis).
	Analysis      analysis.Versions `json:"av,omitempty"`
	LegacyFaceVer int               `json:"facever,omitempty"` // pre-versioning records; migrated at load
	AddedAt       int64             `json:"added"`
	UpdatedAt     int64             `json:"updated"`
	// user metadata, preserved across moves and edits
	Favorite bool   `json:"fav,omitempty"`
	Rating   int    `json:"rating,omitempty"`
	Title    string `json:"title,omitempty"`
	Notes    string `json:"notes,omitempty"`
	Caption  string `json:"caption,omitempty"`
	// Rotate corrects a wrong camera orientation: clockwise degrees on top
	// of EXIF (0, 90, 180, 270), set by the user or, when RotateAuto, by
	// the face detector finding everyone on their side.
	Rotate     int  `json:"rot,omitempty"`
	RotateAuto bool `json:"rotauto,omitempty"`
	// Nsfw is how likely the photo is not safe for work (nudity), from
	// its thumbnail (-1: couldn't judge); UserNsfw is the user's say,
	// which wins.
	Nsfw     float32 `json:"nsfw,omitempty"`
	UserNsfw *bool   `json:"unsfw,omitempty"`
	// ThumbRot is the rotation the stored thumbnail was drawn with; nil
	// (older records): the current one. Until a turned photo is analysed
	// again, its thumbnail is turned when served.
	ThumbRot *int `json:"trot,omitempty"`
	// Sharp is the fine detail in the thumbnail's sharpest parts
	// (media.Sharpness), -1 if it couldn't be judged.
	Sharp float32 `json:"sharp,omitempty"`
	// UserTaken is a capture date the user set because the camera's was
	// wrong; TakenAt then holds it, and CameraTaken what the file says.
	UserTaken   *int64 `json:"utaken,omitempty"`
	CameraTaken *int64 `json:"ctaken,omitempty"`
	// DateOK: the user said the date is right, though the faces in the
	// photo seemed to disagree.
	DateOK bool `json:"dok,omitempty"`
	// GeoSrc says where Lat/Lon came from when the file has no position:
	// GeoUser (set by the user) or GeoNear (borrowed from a photo taken
	// close in time). Empty: from the file, or none. A position the file
	// gets later replaces either.
	GeoSrc string `json:"geosrc,omitempty"`
}

// Where a position not in the file came from (imageRec.GeoSrc).
const (
	GeoUser = "user"
	GeoNear = "near"
)

// fileTaken is the capture date the file itself gives (for matching a
// moved file to its image), whatever date the user set.
func (im *imageRec) fileTaken() *int64 {
	if im.UserTaken != nil {
		return im.CameraTaken
	}
	return im.TakenAt
}

// fileRec is the persisted file record.
type fileRec struct {
	ID           int64  `json:"id"`
	ImageID      int64  `json:"image"`
	FolderID     int64  `json:"folder"`
	Path         string `json:"path"` // slash-separated, relative to the folder
	Size         int64  `json:"size"`
	MTime        int64  `json:"mtime"`
	MissingSince *int64 `json:"missing,omitempty"`
	// Pending means the file is new or changed on disk and waits for the
	// import stage. A new file has ImageID 0 until it is imported.
	Pending bool `json:"pending,omitempty"`

	seenScan int64 // in-memory only: last scan that saw this file
}

// Store is the library database.
type Store struct {
	db *bolt.DB

	mu      sync.RWMutex
	folders map[int64]*Folder
	images  map[int64]*imageRec
	files   map[int64]*fileRec
	byHash  map[string]int64
	byPath  map[int64]map[string]int64 // folder id -> relative path -> file id
	byImage map[int64][]int64          // image id -> file ids
	byTaken map[int64][]int64          // capture time -> image ids (for relinking)

	vectors  map[int64][]float32 // image id -> normalized embedding
	vecModel string              // model that produced vectors

	faces        map[int64]*faceRec
	facesByImage map[int64][]int64
	people       map[int64]*personRec
	clusters     map[int64]*cluster
	albums       map[int64]*albumRec // see albums.go
	junkDir      []float32           // see junk.go
	tune         tuning              // people settings in effect (settings.go)
	// CropPath gives the file of a face crop by key (hash_index); set by
	// the program so pictures can be judged on their pixels.
	CropPath func(key string) string
	// Gender: the age model also estimates gender, so faces without an
	// estimate wait for the age stage. Set before the stages start.
	Gender bool
	// NsfwModel: an NSFW classifier is loaded, so photos without a score
	// wait for the look stage. Set before the stages start.
	NsfwModel bool
	// NoAgeModel: no age model is loaded, so faces are grouped without an
	// age instead of waiting for an age stage that never runs. Set before
	// the stages start.
	NoAgeModel  bool
	qmu         sync.Mutex
	qcache      map[int64]faceQuality
	shotMu      sync.Mutex // see shots.go
	shots       map[int64][]*faceRec
	shotAt      shotKey
	nextCluster int64
}

// Open opens (creating if needed) the database file and loads it.
func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	s := &Store{
		db:      db,
		folders: map[int64]*Folder{},
		images:  map[int64]*imageRec{},
		files:   map[int64]*fileRec{},
		byHash:  map[string]int64{},
		byPath:  map[int64]map[string]int64{},
		byImage: map[int64][]int64{},
		byTaken: map[int64][]int64{},
		vectors: map[int64][]float32{},

		faces:        map[int64]*faceRec{},
		facesByImage: map[int64][]int64{},
		people:       map[int64]*personRec{},
		clusters:     map[int64]*cluster{},
		albums:       map[int64]*albumRec{},
	}
	if err := s.load(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) load() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketFolders, bucketImages, bucketFiles} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		if err := tx.Bucket(bucketFolders).ForEach(func(_, v []byte) error {
			var f Folder
			if err := json.Unmarshal(v, &f); err != nil {
				return err
			}
			s.folders[f.ID] = &f
			return nil
		}); err != nil {
			return err
		}
		if err := tx.Bucket(bucketImages).ForEach(func(_, v []byte) error {
			var im imageRec
			if err := json.Unmarshal(v, &im); err != nil {
				return err
			}
			// Records from before per-component versions: the media data
			// was produced by version 1, faces by the old face version.
			if im.Analysis == nil {
				im.Analysis = analysis.Versions{analysis.CMedia: 1}
				if im.LegacyFaceVer > 0 {
					im.Analysis[analysis.CFaces] = im.LegacyFaceVer
				}
			}
			s.setImage(&im)
			return nil
		}); err != nil {
			return err
		}
		if err := tx.Bucket(bucketFiles).ForEach(func(_, v []byte) error {
			var f fileRec
			if err := json.Unmarshal(v, &f); err != nil {
				return err
			}
			s.indexFile(&f)
			return nil
		}); err != nil {
			return err
		}
		if err := s.loadVectors(tx); err != nil {
			return err
		}
		// Files of a folder that is gone (removed while its scan ran, before
		// Discover checked): forgotten, as removing the folder would have.
		for id, f := range s.files {
			if _, ok := s.folders[f.FolderID]; !ok {
				if err := del(tx, bucketFiles, id); err != nil {
					return err
				}
				s.unindexFile(f)
			}
		}
		if err := s.loadAlbums(tx); err != nil {
			return err
		}
		return s.loadFaces(tx)
	})
}

// --- persistence helpers (call inside a bolt transaction) ---

func key(id int64) []byte {
	var k [8]byte
	binary.BigEndian.PutUint64(k[:], uint64(id))
	return k[:]
}

func put(tx *bolt.Tx, bucket []byte, id int64, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return tx.Bucket(bucket).Put(key(id), data)
}

func del(tx *bolt.Tx, bucket []byte, id int64) error {
	return tx.Bucket(bucket).Delete(key(id))
}

func nextID(tx *bolt.Tx, bucket []byte) (int64, error) {
	n, err := tx.Bucket(bucket).NextSequence()
	return int64(n), err
}

// --- in-memory index maintenance (call with s.mu held) ---

func (s *Store) indexFile(f *fileRec) {
	s.files[f.ID] = f
	m := s.byPath[f.FolderID]
	if m == nil {
		m = map[string]int64{}
		s.byPath[f.FolderID] = m
	}
	m[f.Path] = f.ID
	if f.ImageID != 0 {
		s.byImage[f.ImageID] = append(s.byImage[f.ImageID], f.ID)
	}
}

func (s *Store) unindexFile(f *fileRec) {
	delete(s.files, f.ID)
	if m := s.byPath[f.FolderID]; m != nil && m[f.Path] == f.ID {
		delete(m, f.Path)
	}
	if f.ImageID == 0 {
		return
	}
	ids := s.byImage[f.ImageID]
	for i, id := range ids {
		if id == f.ID {
			ids = append(ids[:i:i], ids[i+1:]...)
			break
		}
	}
	if len(ids) == 0 {
		delete(s.byImage, f.ImageID)
	} else {
		s.byImage[f.ImageID] = ids
	}
}

// setImage adds or replaces an image in the in-memory indexes.
func (s *Store) setImage(im *imageRec) {
	if old, ok := s.images[im.ID]; ok {
		s.unsetImage(old)
	}
	s.images[im.ID] = im
	s.byHash[im.Hash] = im.ID
	if t := im.fileTaken(); t != nil {
		s.byTaken[*t] = append(s.byTaken[*t], im.ID)
	}
}

// unsetImage removes an image from the in-memory indexes.
func (s *Store) unsetImage(im *imageRec) {
	delete(s.images, im.ID)
	if s.byHash[im.Hash] == im.ID {
		delete(s.byHash, im.Hash)
	}
	if t := im.fileTaken(); t != nil {
		ids := slices.DeleteFunc(s.byTaken[*t], func(id int64) bool { return id == im.ID })
		if len(ids) == 0 {
			delete(s.byTaken, *t)
		} else {
			s.byTaken[*t] = ids
		}
	}
}

// present reports whether the image has at least one file on disk.
func (s *Store) present(imageID int64) bool {
	for _, fid := range s.byImage[imageID] {
		if s.files[fid].MissingSince == nil {
			return true
		}
	}
	return false
}

// --- folders ---

func (s *Store) Folders(ctx context.Context) ([]Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := map[int64]map[int64]struct{}{}
	for _, f := range s.files {
		if f.MissingSince != nil {
			continue
		}
		if counts[f.FolderID] == nil {
			counts[f.FolderID] = map[int64]struct{}{}
		}
		counts[f.FolderID][f.ImageID] = struct{}{}
	}
	out := make([]Folder, 0, len(s.folders))
	for _, f := range s.folders {
		c := *f
		c.Images = int64(len(counts[f.ID]))
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b Folder) int { return strings.Compare(a.Location, b.Location) })
	return out, nil
}

func (s *Store) Folder(ctx context.Context, id int64) (Folder, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f, ok := s.folders[id]
	if !ok {
		return Folder{}, ErrNotFound
	}
	return *f, nil
}

func (s *Store) AddFolder(ctx context.Context, location, secret string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.folders {
		if f.Location == location {
			return 0, fmt.Errorf("folder %s already exists", location)
		}
	}
	f := &Folder{Location: location, Secret: secret, AddedAt: time.Now().Unix()}
	err := s.db.Update(func(tx *bolt.Tx) error {
		id, err := nextID(tx, bucketFolders)
		if err != nil {
			return err
		}
		f.ID = id
		return put(tx, bucketFolders, id, f)
	})
	if err != nil {
		return 0, err
	}
	s.folders[f.ID] = f
	return f.ID, nil
}

// RemoveFolder forgets a folder and its file records. Images that no longer
// have any file are kept (with their user metadata) so that re-adding the
// folder, or finding the files elsewhere, restores everything.
func (s *Store) RemoveFolder(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var drop []*fileRec
	for _, fid := range s.byPath[id] {
		drop = append(drop, s.files[fid])
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range drop {
			if err := del(tx, bucketFiles, f.ID); err != nil {
				return err
			}
		}
		return del(tx, bucketFolders, id)
	})
	if err != nil {
		return err
	}
	for _, f := range drop {
		s.unindexFile(f)
	}
	delete(s.byPath, id)
	delete(s.folders, id)
	return nil
}

func (s *Store) SetFolderScanned(ctx context.Context, id, at int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.folders[id]
	if !ok {
		return ErrNotFound
	}
	c := *f
	c.LastScan = &at
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFolders, id, &c) }); err != nil {
		return err
	}
	*f = c
	return nil
}

// --- scanning ---

// Discovered is a file seen by a folder walk.
type Discovered struct {
	Path  string // relative to the folder
	Size  int64
	MTime int64
}

// Discover records a batch of walked files. Unchanged files are just marked
// seen (no write unless they had been missing); new or changed files are
// flagged pending for the import stage. It returns how many are pending.
func (s *Store) Discover(ctx context.Context, folderID, scan int64, batch []Discovered) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.folders[folderID]; !ok {
		return 0, nil // removed while its scan ran: nothing of it is kept
	}
	type change struct {
		old *fileRec // nil for a new file
		rec fileRec
	}
	var changes []change
	pending := 0
	for _, d := range batch {
		old := s.fileAt(folderID, d.Path)
		if old != nil {
			old.seenScan = scan
			unchanged := old.Size == d.Size && old.MTime == d.MTime
			if unchanged && old.MissingSince == nil {
				if old.Pending {
					pending++
				}
				continue
			}
			rec := *old
			rec.MissingSince = nil
			if !unchanged {
				rec.Size, rec.MTime, rec.Pending = d.Size, d.MTime, true
			}
			if rec.Pending {
				pending++
			}
			changes = append(changes, change{old, rec})
			continue
		}
		pending++
		changes = append(changes, change{nil, fileRec{FolderID: folderID, Path: d.Path, Size: d.Size, MTime: d.MTime, Pending: true}})
	}
	if len(changes) == 0 {
		return pending, nil
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for i := range changes {
			c := &changes[i]
			if c.rec.ID == 0 {
				id, err := nextID(tx, bucketFiles)
				if err != nil {
					return err
				}
				c.rec.ID = id
			}
			if err := put(tx, bucketFiles, c.rec.ID, &c.rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, c := range changes {
		if c.old != nil {
			*c.old = c.rec
			c.old.seenScan = scan
			continue
		}
		f := c.rec
		f.seenScan = scan
		s.indexFile(&f)
	}
	return pending, nil
}

// FileJob is a pending file for the import stage.
type FileJob struct {
	FileID   int64
	FolderID int64
	Path     string
	Size     int64
	MTime    int64
}

// PendingFiles returns up to n files waiting for import, skipping ids in skip.
func (s *Store) PendingFiles(ctx context.Context, n int, skip map[int64]bool) []FileJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FileJob
	for _, f := range s.files {
		if f.Pending && f.MissingSince == nil && !skip[f.ID] {
			out = append(out, FileJob{f.ID, f.FolderID, f.Path, f.Size, f.MTime})
			if len(out) == n {
				break
			}
		}
	}
	return out
}

// PendingCount is how many files wait for import.
func (s *Store) PendingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, f := range s.files {
		if f.Pending && f.MissingSince == nil {
			n++
		}
	}
	return n
}

// MarkMissing flags files in folder that were not seen in this scan.
func (s *Store) MarkMissing(ctx context.Context, folderID, scan int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	var gone []*fileRec
	for _, fid := range s.byPath[folderID] {
		if f := s.files[fid]; f.seenScan < scan && f.MissingSince == nil {
			gone = append(gone, f)
		}
	}
	if len(gone) == 0 {
		return 0, nil
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range gone {
			c := *f
			c.MissingSince = &now
			if err := put(tx, bucketFiles, c.ID, &c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, f := range gone {
		f.MissingSince = &now
	}
	return int64(len(gone)), nil
}

// PruneMovedFiles drops missing file records whose image is present at
// another path: the move has been resolved, the old path is history.
func (s *Store) PruneMovedFiles(ctx context.Context) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var drop []*fileRec
	for _, f := range s.files {
		if f.MissingSince != nil && (f.ImageID == 0 || s.present(f.ImageID)) {
			drop = append(drop, f)
		}
	}
	if len(drop) == 0 {
		return 0, nil
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range drop {
			if err := del(tx, bucketFiles, f.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	for _, f := range drop {
		s.unindexFile(f)
	}
	return int64(len(drop)), nil
}

// HashKnown reports whether an image with this content hash exists.
func (s *Store) HashKnown(ctx context.Context, hash string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.byHash[hash]
	return ok, nil
}

// ImageData is the extracted (non-user) metadata for an image.
type ImageData struct {
	ContentHash string
	PHash       uint64
	Width       int
	Height      int
	FileSize    int64
	Format      string
	TakenAt     *time.Time
	Lat, Lon    *float64
	CameraMake  string
	CameraModel string
	Lens        string
	ISO         int
	FNumber     float64
	Exposure    string
	Focal       float64
	HasThumb    bool
	Video       bool
	Duration    float32

	// Analysis lists the components this import performed (empty for a
	// hash-only link). Faces is meaningful when it includes faces; Vector
	// when it includes embed.
	Analysis analysis.Versions
	Faces    []FaceData
	Vector   []float32
	// AutoRotate is a rotation the import applied because the faces were
	// all on their side; it is kept unless the user has set one.
	AutoRotate int
	// Rotated is the rotation correction the import worked with: if the
	// user turned the photo again meanwhile, its analysis is out of date.
	Rotated int
	// ThumbRot is the rotation the thumbnail (with HasThumb) was drawn with.
	ThumbRot int
}

func (d ImageData) apply(im *imageRec) {
	im.Hash, im.PHash, im.Width, im.Height = d.ContentHash, d.PHash, d.Width, d.Height
	im.FileSize, im.Format = d.FileSize, d.Format
	im.TakenAt = nil
	if d.TakenAt != nil {
		t := d.TakenAt.Unix()
		im.TakenAt = &t
	}
	if im.UserTaken != nil { // the user's date stays
		im.CameraTaken, im.TakenAt = im.TakenAt, im.UserTaken
	}
	if d.Lat != nil || im.GeoSrc == "" { // a place set or borrowed stays until the file has one
		im.Lat, im.Lon, im.GeoSrc = d.Lat, d.Lon, ""
	}
	im.CameraMake, im.CameraModel, im.Lens = d.CameraMake, d.CameraModel, d.Lens
	im.ISO, im.FNumber, im.Exposure, im.Focal = d.ISO, d.FNumber, d.Exposure, d.Focal
	im.HasThumb = d.HasThumb
	im.Video, im.Duration = d.Video, d.Duration
	if d.AutoRotate != 0 && (im.Rotate == 0 || im.RotateAuto) {
		im.Rotate, im.RotateAuto = d.AutoRotate, true
	}
}

// FileRef identifies a file for RecordFile.
type FileRef struct {
	FolderID int64
	Path     string // relative to the folder
	Size     int64
	MTime    int64
}

// Resolution says how RecordFile matched a file to an image.
type Resolution int

const (
	ResNew      Resolution = iota // never seen this content before
	ResSameHash                   // identical content already known (move, copy or unchanged)
	ResRelinked                   // content changed but matched a lost image by perceptual hash
	ResUpdated                    // same path, content changed in place
)

// RecordFile links a scanned file to an image, creating or updating the image
// as needed. Matching order:
//  1. exact content hash: a move, rename or duplicate keeps the image;
//  2. same path whose content changed (e.g. EXIF edited): update in place;
//  3. a lost image (no present files) with the same capture time, aspect
//     ratio and a near-identical perceptual hash: a move combined with an edit.
func (s *Store) RecordFile(ctx context.Context, ref FileRef, d ImageData) (int64, Resolution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	var im imageRec // the image as it should be persisted
	res := ResNew
	existingFile := s.fileAt(ref.FolderID, ref.Path)

	if id, ok := s.byHash[d.ContentHash]; ok {
		res, im = ResSameHash, *s.images[id]
	} else if existingFile != nil && existingFile.ImageID != 0 {
		res, im = ResUpdated, *s.images[existingFile.ImageID]
	} else if id, ok := s.findLost(d); ok {
		res, im = ResRelinked, *s.images[id]
	}
	if res != ResSameHash {
		d.apply(&im)
		im.UpdatedAt = now
		// New content: earlier analysis no longer applies.
		im.Analysis = analysis.Versions{}
	}
	writeImage := res != ResSameHash
	// Turned again while this import ran: its thumbnail and faces are for
	// the old orientation. Keep the file waiting to be analysed again.
	turned := res == ResSameHash && len(d.Analysis) > 0 && d.Rotated != im.Rotate
	if turned {
		d.Analysis, d.Faces, d.Vector = nil, nil, nil
	}
	_, didFaces := d.Analysis[analysis.CFaces]
	if len(d.Analysis) > 0 {
		im.Analysis = maps.Clone(im.Analysis)
		if im.Analysis == nil {
			im.Analysis = analysis.Versions{}
		}
		maps.Copy(im.Analysis, d.Analysis)
		im.HasThumb = im.HasThumb || d.HasThumb
		writeImage = true
	}
	if d.HasThumb { // the file on disk is now drawn this way, whatever else is kept
		tr := d.ThumbRot
		im.ThumbRot = &tr
		writeImage = true
	}
	var applyFaces func()

	f := fileRec{FolderID: ref.FolderID, Path: ref.Path, Size: ref.Size, MTime: ref.MTime, Pending: turned}
	if existingFile != nil {
		f.ID = existingFile.ID
	}
	var stale []*fileRec // a relinked image's old, missing paths
	if res == ResRelinked {
		for _, fid := range s.byImage[im.ID] {
			if s.files[fid].MissingSince != nil {
				stale = append(stale, s.files[fid])
			}
		}
	}

	err := s.db.Update(func(tx *bolt.Tx) error {
		var err error
		if res == ResNew {
			if im.ID, err = nextID(tx, bucketImages); err != nil {
				return err
			}
			im.AddedAt = now
		}
		if writeImage {
			if err := put(tx, bucketImages, im.ID, &im); err != nil {
				return err
			}
		}
		if didFaces {
			if applyFaces, err = s.replaceFaces(tx, im.ID, d.Faces); err != nil {
				return err
			}
		}
		if res == ResUpdated || res == ResRelinked {
			// Content changed: the embedding must be recomputed.
			if err := s.dropVector(tx, im.ID); err != nil {
				return err
			}
		}
		if d.Vector != nil {
			if err := tx.Bucket(bucketVectors).Put(key(im.ID), encodeVec(d.Vector)); err != nil {
				return err
			}
		}
		f.ImageID = im.ID
		if f.ID == 0 {
			if f.ID, err = nextID(tx, bucketFiles); err != nil {
				return err
			}
		}
		if err := put(tx, bucketFiles, f.ID, &f); err != nil {
			return err
		}
		for _, sf := range stale {
			if err := del(tx, bucketFiles, sf.ID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, 0, err
	}

	// Commit to memory only after the transaction succeeded.
	if writeImage {
		rec := im
		s.setImage(&rec)
	}
	if applyFaces != nil {
		applyFaces()
	}
	if d.Vector != nil {
		s.vectors[im.ID] = d.Vector
	}
	if existingFile != nil {
		s.unindexFile(existingFile)
	}
	for _, sf := range stale {
		s.unindexFile(sf)
	}
	if existingFile != nil {
		f.seenScan = existingFile.seenScan
	}
	s.indexFile(&f)
	return im.ID, res, nil
}

func (s *Store) fileAt(folderID int64, path string) *fileRec {
	if id, ok := s.byPath[folderID][path]; ok {
		return s.files[id]
	}
	return nil
}

// findLost looks for an image whose files have all vanished and that looks
// like d: same capture time, same aspect ratio (it may have been resized)
// and a perceptual hash within a few bits.
func (s *Store) findLost(d ImageData) (int64, bool) {
	if d.PHash == 0 || d.TakenAt == nil || d.Width == 0 || d.Height == 0 {
		return 0, false
	}
	return s.findLostLike(d.PHash, d.TakenAt.Unix(), d.Width, d.Height, 0)
}

func (s *Store) findLostLike(phash uint64, taken int64, w, h int, exclude int64) (int64, bool) {
	ar := float64(w) / float64(h)
	for _, id := range s.byTaken[taken] {
		im := s.images[id]
		if id == exclude || im.Width == 0 || im.Height == 0 {
			continue
		}
		if math.Abs(float64(im.Width)/float64(im.Height)/ar-1) > 0.01 || hamming(im.PHash, phash) > 6 {
			continue
		}
		if !s.present(id) {
			return id, true
		}
	}
	return 0, false
}

// RelinkNew handles files that were moved *and* changed (re-saved, EXIF
// edited, resized) in the same scan. While walking, the old path still
// looks present, so the new file gets a fresh image. Once missing files are
// marked, each image added since `since` that matches a now-lost image is
// merged into the lost one, which keeps its id and all user metadata.
func (s *Store) RelinkNew(ctx context.Context, since int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var fresh []int64
	for id, im := range s.images {
		if im.AddedAt >= since && im.TakenAt != nil && im.PHash != 0 && im.Width > 0 && im.Height > 0 {
			fresh = append(fresh, id)
		}
	}
	merged := 0
	for _, id := range fresh {
		im := s.images[id]
		lostID, ok := s.findLostLike(im.PHash, *im.TakenAt, im.Width, im.Height, id)
		if !ok {
			continue
		}
		if err := s.merge(id, lostID); err != nil {
			return merged, err
		}
		merged++
	}
	return merged, nil
}

// merge folds image src (new) into dst (lost): dst takes src's extracted
// metadata and files, keeps its own id and user metadata; src is deleted.
func (s *Store) merge(srcID, dstID int64) error {
	src, dst := s.images[srcID], s.images[dstID]
	nd := *dst
	nd.Hash, nd.PHash, nd.Width, nd.Height = src.Hash, src.PHash, src.Width, src.Height
	nd.FileSize, nd.Format, nd.TakenAt = src.FileSize, src.Format, src.TakenAt
	if src.Lat != nil && src.GeoSrc == "" || dst.GeoSrc == "" { // a place dst was given stays unless the file has one
		nd.Lat, nd.Lon, nd.GeoSrc = src.Lat, src.Lon, src.GeoSrc
	}
	nd.CameraMake, nd.CameraModel, nd.Lens = src.CameraMake, src.CameraModel, src.Lens
	nd.ISO, nd.FNumber, nd.Exposure, nd.Focal, nd.HasThumb = src.ISO, src.FNumber, src.Exposure, src.Focal, src.HasThumb
	nd.Video, nd.Duration = src.Video, src.Duration
	nd.Analysis = maps.Clone(src.Analysis)
	nd.UpdatedAt = time.Now().Unix()
	// The new image's faces replace the old one's, keeping the old person
	// tags where faces overlap.
	var srcFaces []FaceData
	for _, fid := range s.facesByImage[srcID] {
		f := s.faces[fid]
		srcFaces = append(srcFaces, FaceData{Box: f.Box, Landmarks: f.Landmarks, Score: f.Score, Embedding: f.emb, CropBox: f.CropBox})
	}
	var applyDst, applySrc func()

	var moveFiles, dropFiles []*fileRec
	for _, fid := range s.byImage[srcID] {
		moveFiles = append(moveFiles, s.files[fid])
	}
	for _, fid := range s.byImage[dstID] {
		if s.files[fid].MissingSince != nil {
			dropFiles = append(dropFiles, s.files[fid])
		}
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := put(tx, bucketImages, dstID, &nd); err != nil {
			return err
		}
		for _, f := range moveFiles {
			c := *f
			c.ImageID = dstID
			if err := put(tx, bucketFiles, c.ID, &c); err != nil {
				return err
			}
		}
		for _, f := range dropFiles {
			if err := del(tx, bucketFiles, f.ID); err != nil {
				return err
			}
		}
		if err := s.dropVector(tx, srcID); err != nil {
			return err
		}
		var err error
		if applySrc, err = s.dropFaces(tx, srcID); err != nil {
			return err
		}
		if applyDst, err = s.replaceFaces(tx, dstID, srcFaces); err != nil {
			return err
		}
		if err := s.dropVector(tx, dstID); err != nil {
			return err
		}
		return del(tx, bucketImages, srcID)
	})
	if err != nil {
		return err
	}
	for _, f := range dropFiles {
		s.unindexFile(f)
	}
	for _, f := range moveFiles {
		s.unindexFile(f)
		f.ImageID = dstID
		s.indexFile(f)
	}
	s.unsetImage(src)
	s.setImage(&nd)
	applySrc()
	applyDst()
	return s.albumsMerged(srcID, dstID) // the photo stays in its albums
}

func hamming(a, b uint64) int {
	x, n := a^b, 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}

// Snapshot writes a consistent copy of the whole database to w, while it
// stays in use. It contains folder passwords: never expose it remotely.
func (s *Store) Snapshot(w io.Writer) error {
	return s.db.View(func(tx *bolt.Tx) error {
		_, err := tx.WriteTo(w)
		return err
	})
}

// Rotation is the rotation correction of the image with this content hash
// (0 if none or unknown).
// ThumbRotation is how much more the stored thumbnail of the image with
// this content must be turned to show it as it should be now: non-zero
// for a photo turned but not drawn again yet.
func (s *Store) ThumbRotation(hash string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byHash[hash]
	if !ok || s.images[id].ThumbRot == nil {
		return 0
	}
	im := s.images[id]
	return ((im.Rotate-*im.ThumbRot)%360 + 360) % 360
}

func (s *Store) Rotation(hash string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if id, ok := s.byHash[hash]; ok {
		return s.images[id].Rotate
	}
	return 0
}

// RotateImage turns a photo by deg (90 or -90) on top of its current
// correction, and queues it to be read and analysed again upright (new
// thumbnail, faces found on the upright picture). Sources are untouched:
// the correction is metadata. It returns the new rotation.
func (s *Store) RotateImage(ctx context.Context, id int64, deg int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	im, ok := s.images[id]
	if !ok {
		return 0, ErrNotFound
	}
	f := s.primaryFile(id)
	if f == nil {
		return 0, ErrNotFound
	}
	c := *im
	if c.ThumbRot == nil { // the thumbnail is as the photo was until now
		tr := c.Rotate
		c.ThumbRot = &tr
	}
	c.Rotate, c.RotateAuto = ((c.Rotate+deg)%360+360)%360, false
	c.Analysis = analysis.Versions{} // read and analyse it again
	fc := *f
	fc.Pending = true
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := put(tx, bucketImages, c.ID, &c); err != nil {
			return err
		}
		return put(tx, bucketFiles, fc.ID, &fc)
	}); err != nil {
		return 0, err
	}
	*im = c
	f.Pending = true
	return c.Rotate, nil
}
