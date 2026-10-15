package store

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/places"
	bolt "go.etcd.io/bbolt"
)

// Photo books (web/book.html): what a book is made from (Source, the
// query its photos come by, e.g. "album=3" or "person=2&kind=years") and
// how it was laid out (Data, the page's own JSON: order, pages, texts,
// design). Kept in the library so they last and go into its backups.
// Books are few and only read when asked for, so they are not kept in
// memory.

var bucketBooks = []byte("books")

// Book is a stored photo book.
type Book struct {
	ID      int64           `json:"id"`
	Title   string          `json:"title"`
	Source  string          `json:"source"`
	Created int64           `json:"created"`
	Updated int64           `json:"updated"`
	Data    json.RawMessage `json:"data,omitempty"`
	// For the list of books, from Data: how many photos and pages, and
	// the cover photo.
	Photos    int    `json:"photos"`
	Pages     int    `json:"pages"`
	Cover     int64  `json:"cover,omitempty"`
	CoverHash string `json:"cover_hash,omitempty"` // in the list, for its thumbnail
	CoverRot  int    `json:"cover_rot,omitempty"`
}

var errNoSource = errors.New("a book needs photos to come from")

// Books lists the books, the last changed first, without their data.
func (s *Store) Books(ctx context.Context) ([]Book, error) {
	var out []Book
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketBooks)
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var bk Book
			if err := json.Unmarshal(v, &bk); err != nil {
				return err
			}
			bk.Data = nil
			out = append(out, bk)
			return nil
		})
	})
	slices.SortFunc(out, func(a, b Book) int { return cmp.Or(cmp.Compare(b.Updated, a.Updated), cmp.Compare(b.ID, a.ID)) })
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := range out {
		if im, ok := s.images[out[i].Cover]; ok && im.HasThumb {
			out[i].CoverHash, out[i].CoverRot = im.Hash, im.Rotate
		}
	}
	return out, err
}

// GetBook reads one book with its data.
func (s *Store) GetBook(ctx context.Context, id int64) (Book, error) {
	var bk Book
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketBooks)
		if b == nil {
			return ErrNotFound
		}
		v := b.Get(key(id))
		if v == nil {
			return ErrNotFound
		}
		return json.Unmarshal(v, &bk)
	})
	return bk, err
}

// SaveBook stores a book: a new one when bk.ID is 0, else over the one
// with its id. It returns the book as stored, without its data.
func (s *Store) SaveBook(ctx context.Context, bk Book) (Book, error) {
	bk.Source = strings.TrimSpace(bk.Source)
	bk.Title = strings.TrimSpace(bk.Title)
	if bk.Source == "" {
		return Book{}, errNoSource
	}
	if len(bk.Data) > 0 && !json.Valid(bk.Data) {
		return Book{}, errors.New("the book's data is not JSON")
	}
	bk.Photos, bk.Pages, bk.Cover = bookSummary(bk.Data)
	now := time.Now().Unix()
	err := s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists(bucketBooks)
		if err != nil {
			return err
		}
		if bk.ID == 0 {
			if bk.ID, err = nextID(tx, bucketBooks); err != nil {
				return err
			}
			bk.Created = now
		} else {
			v := b.Get(key(bk.ID))
			if v == nil {
				return ErrNotFound
			}
			var old Book
			if err := json.Unmarshal(v, &old); err != nil {
				return err
			}
			bk.Created = old.Created
		}
		bk.Updated = now
		return put(tx, bucketBooks, bk.ID, &bk)
	})
	bk.Data = nil
	return bk, err
}

// DeleteBook removes a book; its photos are not touched.
func (s *Store) DeleteBook(ctx context.Context, id int64) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketBooks)
		if b == nil || b.Get(key(id)) == nil {
			return ErrNotFound
		}
		return del(tx, bucketBooks, id)
	})
}

// bookSummary reads from a book's data the counts the list shows. The
// data is the page's; only these fields are looked at.
func bookSummary(data json.RawMessage) (photos, pages int, cover int64) {
	var d struct {
		Order []int64 `json:"order"` // the earlier page's: photos in order
		Out   []int64 `json:"out"`
		Pages []struct {
			N   int     `json:"n"`
			IDs []int64 `json:"ids"` // the photos on the page
		} `json:"pages"`
		Cover int64           `json:"cover"`
		Back  json.RawMessage `json:"back"`
	}
	if json.Unmarshal(data, &d) != nil {
		return 0, 0, 0
	}
	if d.Order == nil {
		for _, p := range d.Pages {
			for _, id := range p.IDs {
				if cover == 0 || id == d.Cover {
					cover = id
				}
				photos++
			}
		}
		pages = len(d.Pages) + 1 // and the cover
		if len(d.Back) > 0 && string(d.Back) != "null" {
			pages++
		}
		return photos, pages, cover
	}
	out := map[int64]bool{}
	for _, id := range d.Out {
		out[id] = true
	}
	for _, id := range d.Order {
		if !out[id] {
			if photos == 0 {
				cover = id
			}
			photos++
		}
	}
	return photos, len(d.Pages) + 1, cover // and the cover page
}

// ImagesByID returns the known images among ids, in that order.
func (s *Store) ImagesByID(ctx context.Context, ids []int64) []Image {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Image, 0, len(ids))
	for _, id := range ids {
		if im, ok := s.images[id]; ok {
			out = append(out, s.view(im))
		}
	}
	return out
}

// NearImageIDs lists the photos with a position within km of a place.
func (s *Store) NearImageIDs(lat, lon, km float64) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []int64{}
	for id, im := range s.images {
		if im.Lat != nil && im.Lon != nil && places.Distance(lat, lon, *im.Lat, *im.Lon) <= km {
			out = append(out, id)
		}
	}
	return out
}
