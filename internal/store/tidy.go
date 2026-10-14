package store

import (
	"cmp"
	"context"
	"math/bits"
	"slices"
	"strings"
)

// A tidy copy of the whole library: every photo once, sorted by when it
// was taken and what took it (see server/tidy.go). Copies of the same
// photo, left by copying a folder in twice, are left out.

// TidyItem is a photo to copy, with where it came from.
type TidyItem struct {
	ExportItem
	Source string // what took it, see photoSource; "" unknown, "screenshot" for screenshots
	Size   int64
}

// TidyExport lists every present photo of the library once, oldest first,
// and how many copies of photos it left out.
func (s *Store) TidyExport(ctx context.Context) ([]TidyItem, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var all []*imageRec
	for id, im := range s.images {
		if s.primaryFile(id) != nil {
			all = append(all, im)
		}
	}
	// Copies: taken the same second, the same size in pixels and (to
	// within 2%) in bytes, and the same picture by its perceptual hash.
	// Bursts differ in bytes, so they stay. The biggest file of each set
	// is kept, then the oldest record.
	byTaken := map[int64][]*imageRec{}
	for _, im := range all {
		if im.TakenAt != nil && im.PHash != 0 {
			byTaken[*im.TakenAt] = append(byTaken[*im.TakenAt], im)
		}
	}
	drop := map[int64]bool{}
	for _, l := range byTaken {
		if len(l) < 2 {
			continue
		}
		slices.SortFunc(l, func(a, b *imageRec) int { return cmp.Or(cmp.Compare(b.FileSize, a.FileSize), cmp.Compare(a.ID, b.ID)) })
		for i, a := range l {
			if drop[a.ID] {
				continue
			}
			for _, b := range l[i+1:] {
				if !drop[b.ID] && sameCopy(a, b) {
					drop[b.ID] = true
				}
			}
		}
	}
	out := make([]TidyItem, 0, len(all)-len(drop))
	for _, im := range all {
		if drop[im.ID] {
			continue
		}
		f := s.primaryFile(im.ID)
		out = append(out, TidyItem{
			ExportItem: ExportItem{ImageID: im.ID, FolderID: f.FolderID, Path: f.Path, Taken: im.TakenAt, Lat: im.Lat, Lon: im.Lon},
			Source:     photoSource(im, f.Path),
			Size:       im.FileSize,
		})
	}
	slices.SortFunc(out, func(a, b TidyItem) int {
		return cmp.Or(cmp.Compare(takenKey(a.Taken), takenKey(b.Taken)), cmp.Compare(a.ImageID, b.ImageID))
	})
	return out, len(drop)
}

func takenKey(t *int64) int64 {
	if t == nil {
		return 1 << 62 // undated last
	}
	return *t
}

// sameCopy reports whether two photos taken the same second are copies
// of one file: same pixels, same size within 2%, near-identical picture.
func sameCopy(a, b *imageRec) bool {
	if a.Width != b.Width || a.Height != b.Height || a.Width == 0 {
		return false
	}
	big, small := max(a.FileSize, b.FileSize), min(a.FileSize, b.FileSize)
	if small <= 0 || float64(big-small)/float64(big) > 0.02 {
		return false
	}
	return bits.OnesCount64(a.PHash^b.PHash) <= 6
}

// cameraNames tidies the makes cameras write ("OLYMPUS IMAGING CORP.",
// "NIKON CORPORATION") into a brand, by what the make contains.
var cameraNames = []struct{ has, name string }{
	{"canon", "Canon"}, {"fuji", "Fujifilm"}, {"olympus", "Olympus"}, {"om digital", "Olympus"},
	{"samsung", "Samsung"}, {"nokia", "Nokia"}, {"hewlett", "HP"}, {"sony", "Sony"},
	{"panasonic", "Panasonic"}, {"nikon", "Nikon"}, {"pentax", "Pentax"}, {"asahi", "Pentax"},
	{"konica", "Konica Minolta"}, {"minolta", "Konica Minolta"}, {"htc", "HTC"}, {"epson", "Epson"},
	{"kodak", "Kodak"}, {"casio", "Casio"}, {"ricoh", "Ricoh"}, {"leica", "Leica"}, {"huawei", "Huawei"},
	{"xiaomi", "Xiaomi"}, {"google", "Google"}, {"oneplus", "OnePlus"}, {"motorola", "Motorola"},
	{"lg electronics", "LG"}, {"sony ericsson", "Sony Ericsson"}, {"gopro", "GoPro"}, {"dji", "DJI"},
}

// photoSource names what took a photo: "iPhone", "iPad", a camera brand,
// "screenshot" for screen pictures (no camera, a PNG or named so), or ""
// when nothing says.
func photoSource(im *imageRec, path string) string {
	mk, model := strings.ToLower(strings.TrimSpace(im.CameraMake)), strings.TrimSpace(im.CameraModel)
	lower := strings.ToLower(path)
	if mk == "" && (im.Format == "png" || strings.Contains(lower, "screenshot") || strings.Contains(lower, "skærmbillede")) {
		return "screenshot"
	}
	switch {
	case mk == "":
		return ""
	case mk == "apple":
		for _, d := range []string{"iPhone", "iPad", "iPod"} {
			if strings.HasPrefix(model, d) {
				return d
			}
		}
		return "Apple"
	case mk == "hp" || strings.HasPrefix(mk, "hp "):
		return "HP"
	}
	// "sony ericsson" before "sony": the longer name wins.
	best := ""
	for _, c := range cameraNames {
		if strings.Contains(mk, c.has) && len(c.has) > len(best) {
			best = c.has
		}
	}
	for _, c := range cameraNames {
		if c.has == best && best != "" {
			return c.name
		}
	}
	// Unknown brand: its first word, capitalised, without characters a
	// folder name can't have.
	w := strings.Fields(im.CameraMake)[0]
	w = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*.`, r) || r < 32 {
			return -1
		}
		return r
	}, w)
	r := []rune(strings.ToLower(w))
	if len(r) == 0 {
		return ""
	}
	return strings.ToUpper(string(r[:1])) + string(r[1:])
}
