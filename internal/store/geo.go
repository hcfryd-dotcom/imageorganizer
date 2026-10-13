package store

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/places"
	bolt "go.etcd.io/bbolt"
)

// Places for photos whose files have none: set by the user, or borrowed
// from a photo taken close in time. Only the library records them; the
// files are never written.

// noPlaceFormats are never given a borrowed place: screenshots and
// graphics, not photos taken somewhere.
var noPlaceFormats = map[string]bool{"png": true, "gif": true, "bmp": true}

// fileGeo reports whether the image's position comes from its file.
func fileGeo(im *imageRec) bool { return im.Lat != nil && im.GeoSrc == "" }

// SetPlace gives the images a position the user picked. Images whose file
// has a position keep it; skipped counts them.
func (s *Store) SetPlace(ctx context.Context, ids []int64, lat, lon float64) (set, skipped int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := map[int64]imageRec{}
	for _, id := range ids {
		im, ok := s.images[id]
		if !ok {
			continue
		}
		if fileGeo(im) {
			skipped++
			continue
		}
		c := *im
		c.Lat, c.Lon, c.GeoSrc = &lat, &lon, GeoUser
		changed[id] = c
	}
	return len(changed), skipped, s.saveImages(changed)
}

// ClearPlace takes away positions that didn't come from the files: of the
// images ids, or with ids nil, every one from src (GeoNear or GeoUser).
func (s *Store) ClearPlace(ctx context.Context, ids []int64, src string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := map[int64]imageRec{}
	clear := func(im *imageRec) {
		if im.GeoSrc == "" || ids == nil && im.GeoSrc != src {
			return
		}
		c := *im
		c.Lat, c.Lon, c.GeoSrc = nil, nil, ""
		changed[im.ID] = c
	}
	if ids == nil {
		for _, im := range s.images {
			clear(im)
		}
	} else {
		for _, id := range ids {
			if im, ok := s.images[id]; ok {
				clear(im)
			}
		}
	}
	return len(changed), s.saveImages(changed)
}

// BorrowPlaces gives photos without a position the position of a photo
// taken within window of them, the nearest in time, from its file or set
// by the user. When the photos just before and just after were taken far
// apart (on the move), the nearer one is used only if it is clearly
// nearer. With apply false it only counts.
func (s *Store) BorrowPlaces(ctx context.Context, window time.Duration, apply bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	type src struct {
		t        int64
		lat, lon float64
	}
	var srcs []src
	for id, im := range s.images {
		if im.Lat != nil && im.GeoSrc != GeoNear && im.TakenAt != nil && s.present(id) {
			srcs = append(srcs, src{*im.TakenAt, *im.Lat, *im.Lon})
		}
	}
	slices.SortFunc(srcs, func(a, b src) int { return cmp.Compare(a.t, b.t) })
	w := int64(window / time.Second)
	changed := map[int64]imageRec{}
	n := 0
	for id, im := range s.images {
		if im.Lat != nil || im.TakenAt == nil || noPlaceFormats[im.Format] || !s.present(id) {
			continue
		}
		t := *im.TakenAt
		i, _ := slices.BinarySearchFunc(srcs, t, func(a src, t int64) int { return cmp.Compare(a.t, t) })
		var prev, next *src
		if i > 0 && t-srcs[i-1].t <= w {
			prev = &srcs[i-1]
		}
		if i < len(srcs) && srcs[i].t-t <= w {
			next = &srcs[i]
		}
		var use *src
		switch {
		case prev == nil:
			use = next
		case next == nil:
			use = prev
		default:
			dp, dn := t-prev.t, next.t-t
			near, far := prev, next
			if dn < dp {
				near, far, dp, dn = next, prev, dn, dp
			}
			// Same place (within 2 km) either side, or clearly nearer in
			// time: within half an hour, or at most half the other's gap.
			if places.Distance(near.lat, near.lon, far.lat, far.lon) <= 2 || dp <= 1800 || dp*2 <= dn {
				use = near
			}
		}
		if use == nil {
			continue
		}
		n++
		if apply {
			c := *im
			lat, lon := use.lat, use.lon
			c.Lat, c.Lon, c.GeoSrc = &lat, &lon, GeoNear
			changed[id] = c
		}
	}
	if !apply {
		return n, nil
	}
	return n, s.saveImages(changed)
}

// saveImages stores changed copies of image records, then puts them in
// place; call with s.mu held. Only for fields no index depends on.
func (s *Store) saveImages(changed map[int64]imageRec) error {
	if len(changed) == 0 {
		return nil
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for id, c := range changed {
			if err := put(tx, bucketImages, id, &c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for id, c := range changed {
		*s.images[id] = c
	}
	return nil
}
