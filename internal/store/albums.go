package store

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	bolt "go.etcd.io/bbolt"
)

// Albums: photos the user put together under a name ("Summer 2024"),
// from any folders. A photo can be in several albums; taking it out of
// one, or deleting an album, never touches the photo.

var bucketAlbums = []byte("albums")

type albumRec struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	Created int64   `json:"created"`
	Images  []int64 `json:"images"` // in the order added
	// Marks: a photo's number (its place in the album, for a photo book)
	// and title in this album, by photo id. Only in the library.
	Marks map[int64]AlbumMark `json:"marks,omitempty"`
}

// AlbumMark is a photo's number and title in an album; a title starting
// with "Chapter" or "Kapitel" begins a chapter in a photo book.
type AlbumMark struct {
	N     int    `json:"n,omitempty"`
	Title string `json:"title,omitempty"`
}

// Album is the API representation: its photos that are present, and the
// first of them as the cover.
type Album struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Created   int64  `json:"created"`
	Photos    int    `json:"photos"`
	CoverHash string `json:"cover,omitempty"`
	CoverRot  int    `json:"cover_rot,omitempty"`
}

// AlbumRef names an album a photo is in.
type AlbumRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

var errNoName = errors.New("give the album a name")

func (s *Store) loadAlbums(tx *bolt.Tx) error {
	b, err := tx.CreateBucketIfNotExists(bucketAlbums)
	if err != nil {
		return err
	}
	return b.ForEach(func(_, v []byte) error {
		var a albumRec
		if err := json.Unmarshal(v, &a); err != nil {
			return err
		}
		s.albums[a.ID] = &a
		return nil
	})
}

// view builds the API album; call with s.mu held.
func (s *Store) albumView(a *albumRec) Album {
	v := Album{ID: a.ID, Name: a.Name, Created: a.Created}
	for _, id := range a.Images {
		im, ok := s.images[id]
		if !ok || !s.present(id) {
			continue
		}
		if v.Photos == 0 && im.HasThumb {
			v.CoverHash, v.CoverRot = im.Hash, im.Rotate
		}
		v.Photos++
	}
	return v
}

// Albums lists the albums by name.
func (s *Store) Albums(ctx context.Context) []Album {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Album, 0, len(s.albums))
	for _, a := range s.albums {
		out = append(out, s.albumView(a))
	}
	slices.SortFunc(out, func(a, b Album) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)), cmp.Compare(a.ID, b.ID))
	})
	return out
}

// saveAlbum stores a changed copy and puts it in place; call with s.mu held.
func (s *Store) saveAlbum(a albumRec) error {
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketAlbums, a.ID, &a) }); err != nil {
		return err
	}
	if cur, ok := s.albums[a.ID]; ok {
		*cur = a
	} else {
		s.albums[a.ID] = &a
	}
	return nil
}

// CreateAlbum makes an album with the photos ids (may be none).
func (s *Store) CreateAlbum(ctx context.Context, name string, ids []int64) (Album, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Album{}, errNoName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a := albumRec{Name: name, Created: time.Now().Unix()}
	err := s.db.Update(func(tx *bolt.Tx) error {
		id, err := nextID(tx, bucketAlbums)
		if err != nil {
			return err
		}
		a.ID = id
		a.Images = s.knownNew(nil, ids)
		return put(tx, bucketAlbums, a.ID, &a)
	})
	if err != nil {
		return Album{}, err
	}
	s.albums[a.ID] = &a
	return s.albumView(&a), nil
}

// knownNew is ids that are images and not in have, once each, in order.
func (s *Store) knownNew(have, ids []int64) []int64 {
	in := make(map[int64]bool, len(have))
	for _, id := range have {
		in[id] = true
	}
	var out []int64
	for _, id := range ids {
		if _, ok := s.images[id]; ok && !in[id] {
			in[id] = true
			out = append(out, id)
		}
	}
	return out
}

// RenameAlbum names an album anew.
func (s *Store) RenameAlbum(ctx context.Context, id int64, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errNoName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.albums[id]
	if !ok {
		return ErrNotFound
	}
	c := *a
	c.Name = name
	return s.saveAlbum(c)
}

// DeleteAlbum removes an album; its photos stay where they are.
func (s *Store) DeleteAlbum(ctx context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.albums[id]; !ok {
		return ErrNotFound
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return del(tx, bucketAlbums, id) }); err != nil {
		return err
	}
	delete(s.albums, id)
	return nil
}

// AddToAlbum puts photos in an album, after those already there; it
// returns how many were new to it.
func (s *Store) AddToAlbum(ctx context.Context, id int64, ids []int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.albums[id]
	if !ok {
		return 0, ErrNotFound
	}
	add := s.knownNew(a.Images, ids)
	if len(add) == 0 {
		return 0, nil
	}
	c := *a
	c.Images = append(slices.Clone(a.Images), add...)
	return len(add), s.saveAlbum(c)
}

// RemoveFromAlbum takes photos out of an album; it returns how many were in it.
func (s *Store) RemoveFromAlbum(ctx context.Context, id int64, ids []int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.albums[id]
	if !ok {
		return 0, ErrNotFound
	}
	out := make(map[int64]bool, len(ids))
	for _, i := range ids {
		out[i] = true
	}
	c := *a
	c.Images = slices.DeleteFunc(slices.Clone(a.Images), func(i int64) bool { return out[i] })
	n := len(a.Images) - len(c.Images)
	if n == 0 {
		return 0, nil
	}
	if len(a.Marks) > 0 {
		c.Marks = maps.Clone(a.Marks)
		for i := range out {
			delete(c.Marks, i)
		}
	}
	return n, s.saveAlbum(c)
}

// SetAlbumMark gives a photo in an album its number (0: none) and title
// (empty: none).
func (s *Store) SetAlbumMark(ctx context.Context, id, imageID int64, m AlbumMark) error {
	m.Title = strings.TrimSpace(m.Title)
	if m.N < 0 {
		m.N = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.albums[id]
	if !ok || !slices.Contains(a.Images, imageID) {
		return ErrNotFound
	}
	c := *a
	c.Marks = maps.Clone(a.Marks)
	if c.Marks == nil {
		c.Marks = map[int64]AlbumMark{}
	}
	if m == (AlbumMark{}) {
		delete(c.Marks, imageID)
	} else {
		c.Marks[imageID] = m
	}
	return s.saveAlbum(c)
}

// AlbumMarks lists an album's marks by photo id (nil: none, or no album).
func (s *Store) AlbumMarks(id int64) map[int64]AlbumMark {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if a, ok := s.albums[id]; ok {
		return maps.Clone(a.Marks)
	}
	return nil
}

// AlbumOrder is an album's numbered photos by their numbers (photos with
// one number by the order they were added): the order a photo list
// sorted "album" puts first, the rest after them oldest first.
func (s *Store) AlbumOrder(id int64) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.albums[id]
	if !ok {
		return nil
	}
	var out []int64
	for _, im := range a.Images {
		if a.Marks[im].N > 0 {
			out = append(out, im)
		}
	}
	slices.SortStableFunc(out, func(x, y int64) int { return cmp.Compare(a.Marks[x].N, a.Marks[y].N) })
	return out
}

// AlbumImageIDs lists an album's photos (nil: no such album), for
// filtering the photo list.
func (s *Store) AlbumImageIDs(id int64) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.albums[id]
	if !ok {
		return nil
	}
	return slices.Clone(a.Images)
}

// AlbumsOf lists the albums a photo is in, by name.
func (s *Store) AlbumsOf(ctx context.Context, imageID int64) []AlbumRef {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []AlbumRef{}
	for _, a := range s.albums {
		if slices.Contains(a.Images, imageID) {
			out = append(out, AlbumRef{a.ID, a.Name})
		}
	}
	slices.SortFunc(out, func(a, b AlbumRef) int { return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return out
}

// albumsMerged follows an image merged into another (store.merge): src's
// places in albums become dst's. Call with s.mu held.
func (s *Store) albumsMerged(src, dst int64) error {
	for _, a := range s.albums {
		if !slices.Contains(a.Images, src) {
			continue
		}
		c := *a
		c.Images = slices.Clone(a.Images)
		for i, id := range c.Images {
			if id == src {
				c.Images[i] = dst
			}
		}
		if m, ok := a.Marks[src]; ok { // its number and title go with it
			c.Marks = maps.Clone(a.Marks)
			delete(c.Marks, src)
			if _, has := c.Marks[dst]; !has {
				c.Marks[dst] = m
			}
		}
		c.Images = s.knownNewKeep(c.Images)
		if err := s.saveAlbum(c); err != nil {
			return err
		}
	}
	return nil
}

// knownNewKeep drops repeats, keeping the first of each.
func (s *Store) knownNewKeep(ids []int64) []int64 {
	seen := map[int64]bool{}
	return slices.DeleteFunc(ids, func(id int64) bool {
		if seen[id] {
			return true
		}
		seen[id] = true
		return false
	})
}
