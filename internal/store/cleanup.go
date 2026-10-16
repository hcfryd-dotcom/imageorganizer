package store

import (
	"cmp"
	"context"
	"math/bits"
	"regexp"
	"slices"
	"strings"
)

// Cleanup: photos one may want rid of, for the Cleanup page to show and
// the user to choose from: receipts and papers (by what they look like),
// screenshots, blurry photos and near copies taken in a row. Nothing here
// moves or changes a file; deleting is the user's, by /api/images/delete.

// CleanupItem is a photo offered for cleaning up. Set groups near copies;
// Keep marks the one of a set to keep (the sharpest, or a favourite).
type CleanupItem struct {
	Image
	Set  int  `json:"set,omitempty"`
	Keep bool `json:"keep,omitempty"`
}

var shotName = regexp.MustCompile(`(?i)screenshot|skærmbillede|screen shot`)

// isShot reports whether a photo is a screenshot: a PNG no camera made,
// or one named so (the photo books use the same rule).
func isShot(im *imageRec, path string) bool {
	return strings.EqualFold(im.Format, "png") && im.CameraMake == "" || shotName.MatchString(path)
}

// CleanupByMeaning ranks photos by how much more they look like pos (the
// mean likeness to its texts' vectors) than like neg, best first, up to
// n. Screenshots and photos not embedded yet are left out.
func (s *Store) CleanupByMeaning(ctx context.Context, pos, neg [][]float32, n int) []CleanupItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mean := func(v []float32, qs [][]float32) float32 {
		if len(qs) == 0 {
			return 0
		}
		var t float32
		for _, q := range qs {
			t += dot(q, v)
		}
		return t / float32(len(qs))
	}
	var out []CleanupItem
	for _, im := range s.selectImages(Filter{}) {
		v, ok := s.vectors[im.ID]
		if !ok || im.Video {
			continue
		}
		img := s.view(im)
		if isShot(im, img.Path) {
			continue
		}
		img.Score = mean(v, pos) - mean(v, neg)
		out = append(out, CleanupItem{Image: img})
	}
	slices.SortFunc(out, func(a, b CleanupItem) int { return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(b.ID, a.ID)) })
	return out[:min(len(out), n)]
}

// CleanupShots lists the screenshots, newest first.
func (s *Store) CleanupShots(ctx context.Context) []CleanupItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []CleanupItem
	for _, im := range s.selectImages(Filter{}) {
		if im.Video {
			continue
		}
		if img := s.view(im); isShot(im, img.Path) {
			out = append(out, CleanupItem{Image: img})
		}
	}
	slices.SortFunc(out, func(a, b CleanupItem) int { return cmp.Compare(takenOrImg(b.Image), takenOrImg(a.Image)) })
	return out
}

// CleanupBlurry lists the photos that look blurry, the blurriest first
// (Score 1 for no fine detail at all, towards 0 at the limit).
func (s *Store) CleanupBlurry(ctx context.Context) []CleanupItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []CleanupItem
	for _, im := range s.selectImages(Filter{}) {
		if im.Video || !isBlurry(im) {
			continue
		}
		img := s.view(im)
		img.Score = 1 - im.Sharp/blurMax
		out = append(out, CleanupItem{Image: img})
	}
	slices.SortFunc(out, func(a, b CleanupItem) int { return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(b.ID, a.ID)) })
	return out
}

// Near copies: taken within copyGap seconds of one another and looking
// alike (perceptual hashes within copyBits bits), as a burst of the same
// moment is.
const (
	copyGap  = 60
	copyBits = 10
)

// CleanupCopies lists sets of near copies, newest set first; in each the
// photo to keep comes first (Keep): a favourite or rated one, else the
// sharpest, else the biggest file. Favourites and rated photos are always
// kept.
func (s *Store) CleanupCopies(ctx context.Context) []CleanupItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var list []*imageRec
	for _, im := range s.selectImages(Filter{}) {
		if !im.Video && im.TakenAt != nil && im.PHash != 0 {
			list = append(list, im)
		}
	}
	slices.SortFunc(list, func(a, b *imageRec) int { return cmp.Or(cmp.Compare(*a.TakenAt, *b.TakenAt), cmp.Compare(a.ID, b.ID)) })
	// Union-find over pairs close in time and look.
	parent := make([]int, len(list))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i, a := range list {
		for j := i - 1; j >= 0 && *a.TakenAt-*list[j].TakenAt <= copyGap; j-- {
			if bits.OnesCount64(a.PHash^list[j].PHash) <= copyBits {
				parent[find(i)] = find(j)
			}
		}
	}
	sets := map[int][]*imageRec{}
	for i, im := range list {
		r := find(i)
		sets[r] = append(sets[r], im)
	}
	var groups [][]*imageRec
	for _, g := range sets {
		if len(g) > 1 {
			groups = append(groups, g)
		}
	}
	slices.SortFunc(groups, func(a, b []*imageRec) int { return cmp.Compare(*b[0].TakenAt, *a[0].TakenAt) })
	valued := func(im *imageRec) bool { return im.Favorite || im.Rating > 0 }
	var out []CleanupItem
	for n, g := range groups {
		slices.SortFunc(g, func(a, b *imageRec) int {
			return cmp.Or(-cmp.Compare(boolInt(valued(a)), boolInt(valued(b))), cmp.Compare(b.Sharp, a.Sharp), cmp.Compare(b.FileSize, a.FileSize), cmp.Compare(a.ID, b.ID))
		})
		for k, im := range g {
			out = append(out, CleanupItem{Image: s.view(im), Set: n + 1, Keep: k == 0 || valued(im)})
		}
	}
	return out
}

func takenOrImg(im Image) int64 {
	if im.TakenAt == nil {
		return 0
	}
	return *im.TakenAt
}
