package store

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Image is the API representation of an image.
type Image struct {
	ID          int64    `json:"id"`
	Hash        string   `json:"hash"`
	Width       int      `json:"w"`
	Height      int      `json:"h"`
	FileSize    int64    `json:"size"`
	Format      string   `json:"format"`
	TakenAt     *int64   `json:"taken"`
	CameraTaken *int64   `json:"camera_taken,omitempty"` // the file's date, when the user set another
	Lat         *float64 `json:"lat,omitempty"`
	Lon         *float64 `json:"lon,omitempty"`
	CameraMake  string   `json:"make,omitempty"`
	CameraModel string   `json:"model,omitempty"`
	Lens        string   `json:"lens,omitempty"`
	ISO         int      `json:"iso,omitempty"`
	FNumber     float64  `json:"fnumber,omitempty"`
	Exposure    string   `json:"exposure,omitempty"`
	Focal       float64  `json:"focal,omitempty"`
	HasThumb    bool     `json:"thumb"`
	Favorite    bool     `json:"fav"`
	Rating      int      `json:"rating"`
	Title       string   `json:"title,omitempty"`
	Notes       string   `json:"notes,omitempty"`
	Caption     string   `json:"caption,omitempty"`
	Rot         int      `json:"rot,omitempty"`        // rotation correction, also busts cached thumbnails
	Nsfw        bool     `json:"nsfw,omitempty"`       // not safe for work (the model's call, or the user's)
	NsfwScore   float32  `json:"nsfw_score,omitempty"` // the model's estimate, 0..1
	NsfwUser    *bool    `json:"nsfw_user,omitempty"`  // the user's say, when given
	Blurry      bool     `json:"blurry,omitempty"`     // looks blurry (little fine detail)
	Sharp       float32  `json:"sharp,omitempty"`      // the sharpness measured (media.Sharpness)
	Path        string   `json:"path"`
	Score       float32  `json:"score,omitempty"` // search relevance
}

// view builds the API image; call with s.mu held.
func (s *Store) view(im *imageRec) Image {
	v := Image{
		ID: im.ID, Hash: im.Hash, Width: im.Width, Height: im.Height, FileSize: im.FileSize, Format: im.Format,
		TakenAt: im.TakenAt, Lat: im.Lat, Lon: im.Lon, CameraMake: im.CameraMake, CameraModel: im.CameraModel,
		Lens: im.Lens, ISO: im.ISO, FNumber: im.FNumber, Exposure: im.Exposure, Focal: im.Focal,
		HasThumb: im.HasThumb, Favorite: im.Favorite, Rating: im.Rating, Title: im.Title, Notes: im.Notes,
		Caption: im.Caption, Rot: im.Rotate,
		Nsfw: s.isNsfw(im), NsfwScore: max(im.Nsfw, 0), NsfwUser: im.UserNsfw, Blurry: isBlurry(im), Sharp: max(im.Sharp, 0),
	}
	if im.UserTaken != nil {
		v.CameraTaken = im.CameraTaken
		if v.CameraTaken == nil {
			v.CameraTaken = new(int64) // the file had no date
		}
	}
	if f := s.primaryFile(im.ID); f != nil {
		v.Path = s.displayPath(f)
	}
	return v
}

// primaryFile is the first present file of an image (lowest id).
func (s *Store) primaryFile(imageID int64) *fileRec {
	var best *fileRec
	for _, fid := range s.byImage[imageID] {
		f := s.files[fid]
		if f.MissingSince == nil && (best == nil || f.ID < best.ID) {
			best = f
		}
	}
	return best
}

func (s *Store) displayPath(f *fileRec) string {
	loc := ""
	if fo, ok := s.folders[f.FolderID]; ok {
		loc = fo.Location
	}
	return strings.TrimSuffix(loc, "/") + "/" + f.Path
}

// Filter selects images for listing.
type Filter struct {
	Text     string // case-insensitive substring match on path, title, notes, caption, camera
	FolderID int64
	Favorite bool
	HasGeo   bool
	From, To *time.Time
	Sort     string // "taken_desc" (default), "taken_asc", "added_desc", "size_desc", "rating", "camera"
	Offset   int
	Limit    int
	IDs      []int64 // restrict to these ids
	// HideNsfw leaves out photos not safe for work; OnlyNsfw lists just them.
	HideNsfw, OnlyNsfw bool
	Blurry             bool // only photos that look blurry
}

// match reports whether im passes f; call with s.mu held.
func (s *Store) match(im *imageRec, f *Filter, words []string, ids map[int64]bool) bool {
	if ids != nil && !ids[im.ID] {
		return false
	}
	if f.Favorite && !im.Favorite {
		return false
	}
	if f.HasGeo && im.Lat == nil {
		return false
	}
	if f.OnlyNsfw && !s.isNsfw(im) || f.HideNsfw && !f.OnlyNsfw && s.isNsfw(im) {
		return false
	}
	if f.Blurry && !isBlurry(im) {
		return false
	}
	if f.From != nil && (im.TakenAt == nil || *im.TakenAt < f.From.Unix()) {
		return false
	}
	if f.To != nil && (im.TakenAt == nil || *im.TakenAt >= f.To.Unix()) {
		return false
	}
	// Only images that currently exist (in the folder, if given) are listed.
	var paths []string
	present := false
	for _, fid := range s.byImage[im.ID] {
		fr := s.files[fid]
		if fr.MissingSince != nil || (f.FolderID != 0 && fr.FolderID != f.FolderID) {
			continue
		}
		present = true
		if len(words) > 0 {
			paths = append(paths, strings.ToLower(fr.Path))
		}
	}
	if !present {
		return false
	}
	if len(words) > 0 {
		hay := strings.ToLower(im.Title+"\x00"+im.Notes+"\x00"+im.Caption+"\x00"+im.CameraMake+" "+im.CameraModel) +
			"\x00" + strings.Join(paths, "\x00")
		for _, w := range words {
			if !strings.Contains(hay, w) {
				return false
			}
		}
	}
	return true
}

// selectImages returns matching image records; call with s.mu held.
func (s *Store) selectImages(f Filter) []*imageRec {
	words := strings.Fields(strings.ToLower(f.Text))
	var ids map[int64]bool
	if f.IDs != nil {
		ids = make(map[int64]bool, len(f.IDs))
		for _, id := range f.IDs {
			ids[id] = true
		}
	}
	out := make([]*imageRec, 0, 1024)
	for _, im := range s.images {
		if s.match(im, &f, words, ids) {
			out = append(out, im)
		}
	}
	return out
}

func takenOr(im *imageRec, def int64) int64 {
	if im.TakenAt == nil {
		return def
	}
	return *im.TakenAt
}

func sortImages(list []*imageRec, by string) {
	byID := func(a, b *imageRec) int { return cmp.Compare(b.ID, a.ID) }
	var fn func(a, b *imageRec) int
	switch by {
	case "taken_asc":
		fn = func(a, b *imageRec) int {
			return cmp.Or(cmp.Compare(takenOr(a, 1<<62), takenOr(b, 1<<62)), cmp.Compare(a.ID, b.ID))
		}
	case "added_desc":
		fn = func(a, b *imageRec) int { return cmp.Or(cmp.Compare(b.AddedAt, a.AddedAt), byID(a, b)) }
	case "size_desc":
		fn = func(a, b *imageRec) int { return cmp.Or(cmp.Compare(b.FileSize, a.FileSize), byID(a, b)) }
	case "rating":
		fn = func(a, b *imageRec) int {
			return cmp.Or(cmp.Compare(b.Rating, a.Rating), cmp.Compare(takenOr(b, 0), takenOr(a, 0)), byID(a, b))
		}
	case "camera":
		fn = func(a, b *imageRec) int {
			return cmp.Or(cmp.Compare(a.CameraModel, b.CameraModel), cmp.Compare(takenOr(b, 0), takenOr(a, 0)), byID(a, b))
		}
	default: // taken_desc; undated last
		fn = func(a, b *imageRec) int { return cmp.Or(cmp.Compare(takenOr(b, -1), takenOr(a, -1)), byID(a, b)) }
	}
	slices.SortFunc(list, fn)
}

// Images lists images matching f and the total match count.
func (s *Store) Images(ctx context.Context, f Filter) ([]Image, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := s.selectImages(f)
	sortImages(list, f.Sort)
	if f.Limit <= 0 || f.Limit > 2000 {
		f.Limit = 200
	}
	start := min(max(f.Offset, 0), len(list))
	end := min(start+f.Limit, len(list))
	out := make([]Image, 0, end-start)
	for _, im := range list[start:end] {
		out = append(out, s.view(im))
	}
	return out, len(list), nil
}

// File is one on-disk location of an image.
type File struct {
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	MTime        int64  `json:"mtime"`
	MissingSince *int64 `json:"missing_since"`
}

// ImageDetail is an image with all its file locations.
type ImageDetail struct {
	Image
	Files []File `json:"files"`
}

func (s *Store) Image(ctx context.Context, id int64) (ImageDetail, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	im, ok := s.images[id]
	if !ok {
		return ImageDetail{}, ErrNotFound
	}
	d := ImageDetail{Image: s.view(im), Files: []File{}}
	for _, fid := range s.byImage[id] {
		f := s.files[fid]
		d.Files = append(d.Files, File{Path: s.displayPath(f), Size: f.Size, MTime: f.MTime, MissingSince: f.MissingSince})
	}
	slices.SortFunc(d.Files, func(a, b File) int {
		return cmp.Or(cmp.Compare(boolInt(a.MissingSince != nil), boolInt(b.MissingSince != nil)), cmp.Compare(a.Path, b.Path))
	})
	return d, nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ImageFile locates a present copy of an image for serving.
type ImageFile struct {
	FolderID int64
	Path     string // relative to the folder
	Hash     string
	Format   string
	Rotate   int // rotation correction to apply when rendering
}

func (s *Store) ImageFile(ctx context.Context, id int64) (ImageFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	im, ok := s.images[id]
	if !ok {
		return ImageFile{}, ErrNotFound
	}
	f := s.primaryFile(id)
	if f == nil {
		return ImageFile{}, ErrNotFound
	}
	return ImageFile{FolderID: f.FolderID, Path: f.Path, Hash: im.Hash, Format: im.Format, Rotate: im.Rotate}, nil
}

// UserEdit holds the user-editable fields; nil means unchanged.
type UserEdit struct {
	Favorite *bool   `json:"fav"`
	Rating   *int    `json:"rating"`
	Title    *string `json:"title"`
	Notes    *string `json:"notes"`
	// Taken sets the capture date (Unix seconds) when the camera's is
	// wrong; ResetTaken goes back to the file's date.
	Taken      *int64 `json:"taken"`
	ResetTaken bool   `json:"reset_taken"`
	// DateOK says the date is right, though the faces seem to disagree.
	DateOK *bool `json:"date_ok"`
	// Nsfw says whether the photo is not safe for work, over the model;
	// ResetNsfw leaves it to the model again.
	Nsfw      *bool `json:"nsfw"`
	ResetNsfw bool  `json:"reset_nsfw"`
}

func (s *Store) EditImage(ctx context.Context, id int64, e UserEdit) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	im, ok := s.images[id]
	if !ok {
		return ErrNotFound
	}
	c := *im
	if e.Favorite != nil {
		c.Favorite = *e.Favorite
	}
	if e.Rating != nil {
		c.Rating = max(0, min(5, *e.Rating))
	}
	if e.Title != nil {
		c.Title = *e.Title
	}
	if e.Notes != nil {
		c.Notes = *e.Notes
	}
	switch {
	case e.Taken != nil:
		if c.UserTaken == nil {
			c.CameraTaken = c.TakenAt
		}
		t := *e.Taken
		c.UserTaken, c.TakenAt = &t, &t
	case e.ResetTaken && c.UserTaken != nil:
		c.TakenAt, c.UserTaken, c.CameraTaken = c.CameraTaken, nil, nil
	}
	if e.DateOK != nil {
		c.DateOK = *e.DateOK
	}
	switch {
	case e.Nsfw != nil:
		v := *e.Nsfw
		c.UserNsfw = &v
	case e.ResetNsfw:
		c.UserNsfw = nil
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketImages, id, &c) }); err != nil {
		return err
	}
	if !ptrEq(c.TakenAt, im.TakenAt) {
		// The faces' groups track capture dates: take them out under the
		// old date and back in under the new one.
		s.regroupFacesOf(id, func() {
			s.unsetImage(im)
			*im = c
			s.setImage(im)
		})
		return nil
	}
	*im = c
	return nil
}

// GeoPoint is a compact map marker.
type GeoPoint struct {
	ID   int64   `json:"i"`
	Lat  float64 `json:"a"`
	Lon  float64 `json:"o"`
	Hash string  `json:"h"`
}

func (s *Store) GeoPoints(ctx context.Context, f Filter) ([]GeoPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f.HasGeo = true
	list := s.selectImages(f)
	sortImages(list, "taken_desc")
	out := make([]GeoPoint, 0, len(list))
	for _, im := range list {
		out = append(out, GeoPoint{ID: im.ID, Lat: *im.Lat, Lon: *im.Lon, Hash: im.Hash})
	}
	return out, nil
}

// MonthCount is used by the timeline scrubber.
type MonthCount struct {
	Month string `json:"m"` // YYYY-MM
	Count int    `json:"n"`
}

func (s *Store) Timeline(ctx context.Context, f Filter) ([]MonthCount, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	counts := map[string]int{}
	for _, im := range s.selectImages(f) {
		if im.TakenAt != nil {
			counts[time.Unix(*im.TakenAt, 0).UTC().Format("2006-01")]++
		}
	}
	out := make([]MonthCount, 0, len(counts))
	for m, n := range counts {
		out = append(out, MonthCount{m, n})
	}
	slices.SortFunc(out, func(a, b MonthCount) int { return strings.Compare(b.Month, a.Month) })
	return out, nil
}

// Stats summarises the library.
type Stats struct {
	Images  int   `json:"images"`
	Files   int   `json:"files"`
	Missing int   `json:"missing"`
	Geo     int   `json:"geo"`
	Fav     int   `json:"favorites"`
	Bytes   int64 `json:"bytes"`
	Lost    int   `json:"lost"` // images with no present file (metadata kept)
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var st Stats
	for _, f := range s.files {
		if f.MissingSince != nil {
			st.Missing++
		} else {
			st.Files++
			st.Bytes += f.Size
		}
	}
	for id, im := range s.images {
		if s.present(id) {
			st.Images++
			if im.Lat != nil {
				st.Geo++
			}
			if im.Favorite {
				st.Fav++
			}
		} else {
			st.Lost++
		}
	}
	return st, nil
}
