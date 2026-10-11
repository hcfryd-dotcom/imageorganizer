package store

import (
	"context"
	"log/slog"
	"time"

	bolt "go.etcd.io/bbolt"
)

// StartOver forgets every decision about who is who, keeping all analysis
// (faces, their prints, ages, genders, crops): no people, no faces named,
// rejected, hidden or marked as pictures, relationships gone; the faces
// are grouped afresh. Photos keep what isn't about people (ratings, titles,
// dates, turns). For trying a different way of naming on a copy of a
// library: it can't be undone.
func (s *Store) StartOver(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	start := time.Now()
	for _, c := range s.clusters {
		c.PersonID, c.Auto, c.Hidden, c.Rejected, c.SkippedAt = 0, false, false, nil, 0
	}
	for _, f := range s.faces {
		f.Tagged, f.Not, f.Picture, f.Live, f.Skipped, f.NotFace = 0, nil, false, false, 0, false
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketPeople); err != nil && err != bolt.ErrBucketNotFound {
			return err
		}
		if _, err := tx.CreateBucket(bucketPeople); err != nil {
			return err
		}
		s.people = map[int64]*personRec{}
		return s.regroup(tx) // with no decisions left, nothing carries over
	})
	if err != nil {
		return err
	}
	slog.Info("started over: no one named, faces grouped afresh", "faces", len(s.faces), "groups", len(s.clusters), "took", time.Since(start).Round(time.Millisecond))
	return nil
}
