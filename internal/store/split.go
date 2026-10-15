package store

import (
	"context"
	"time"

	bolt "go.etcd.io/bbolt"
)

// NotBefore takes out of a person every face of theirs in a photo taken
// before a date (their birth, say) and puts those faces together in one
// new group that is known not to be them, so it can be named in Review.
// Faces filed through a group and faces tagged one by one are both taken
// out; it returns how many.
func (s *Store) NotBefore(ctx context.Context, pid int64, before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.people[pid]; !ok {
		return 0, ErrNotFound
	}
	cut := before.Unix()
	var take []*faceRec
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if !(c != nil && c.PersonID == pid || f.Tagged == pid) || f.NotFace {
			continue
		}
		if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil && *im.TakenAt < cut {
			take = append(take, f)
		}
	}
	if len(take) == 0 {
		return 0, nil
	}
	s.nextCluster++
	nc := &cluster{id: s.nextCluster, Rejected: []int64{pid}}
	recs := make([]faceRec, len(take))
	for i, f := range take {
		recs[i] = *f
		recs[i].ClusterID = nc.id
		if recs[i].Tagged == pid {
			recs[i].Tagged = 0
		}
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		for i := range recs {
			if err := put(tx, bucketFaces, recs[i].ID, &recs[i]); err != nil {
				return err
			}
		}
		return putGroup(tx, nc)
	}); err != nil {
		return 0, err
	}
	s.clusters[nc.id] = nc
	for i, f := range take {
		if old := s.clusters[f.ClusterID]; old != nil {
			s.clusterRemove(old, f)
		}
		f.ClusterID, f.Tagged = recs[i].ClusterID, recs[i].Tagged
		s.clusterAdd(nc, f)
	}
	return len(take), nil
}

// CountBefore is how many faces NotBefore would take out.
func (s *Store) CountBefore(ctx context.Context, pid int64, before time.Time) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cut, n := before.Unix(), 0
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if !(c != nil && c.PersonID == pid || f.Tagged == pid) || f.NotFace {
			continue
		}
		if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil && *im.TakenAt < cut {
			n++
		}
	}
	return n
}
