package store

import (
	"context"

	bolt "go.etcd.io/bbolt"
)

// SetFacePicture marks a face as a picture of its person (a framed photo
// on the wall, a poster) or takes the mark off. The face keeps its name,
// but no longer tells their age at the photo's date or keeps the real
// person out of the same photo.
func (s *Store) SetFacePicture(ctx context.Context, faceID int64, on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.faces[faceID]
	if !ok {
		return ErrNotFound
	}
	if f.Picture == on {
		return nil
	}
	rec := *f
	rec.Picture = on
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, rec.ID, &rec) }); err != nil {
		return err
	}
	c := s.clusters[f.ClusterID]
	if c != nil {
		s.clusterRemove(c, f)
	}
	f.Picture = on
	if c != nil {
		s.clusterAdd(c, f)
	}
	return nil
}

// regroupFacesOf takes an image's faces out of their groups' running sums,
// runs change (which alters what the sums depend on, like the date), and
// puts them back.
func (s *Store) regroupFacesOf(imageID int64, change func()) {
	var in []*faceRec
	for _, fid := range s.facesByImage[imageID] {
		if f := s.faces[fid]; f != nil {
			if c := s.clusters[f.ClusterID]; c != nil {
				s.clusterRemove(c, f)
				in = append(in, f)
			}
		}
	}
	change()
	for _, f := range in {
		s.clusterAdd(s.clusters[f.ClusterID], f)
	}
}

// outOfTime reports whether a face doesn't look the age its person (by
// their confirmed faces) had at the photo's date.
func (s *Store) outOfTime(f *faceRec, pid int64) bool {
	by, ok := s.impliedBirth(f)
	if !ok || s.people[pid] == nil {
		return false
	}
	groups := map[int64]bool{}
	for _, c := range s.clusters {
		if c.PersonID == pid && !c.Auto {
			groups[c.id] = true
		}
	}
	m := s.buildAgeModel(s.people[pid], groups)
	want, ok := m.looksAt(by + f.Age)
	return ok && abs32(f.Age-want) > birthTolerance(max(f.Age, want))
}

func ptrEq(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// NotFacesInGroup says a whole group is not people (statues, patterns, a
// pet): its faces are marked as not faces, like NotAFace one by one. It
// returns their ids, for RestoreGroupFaces to undo it.
func (s *Store) NotFacesInGroup(ctx context.Context, gid int64) ([]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c := s.clusters[gid]
	if c == nil {
		return nil, ErrNotFound
	}
	var fs []*faceRec
	for _, f := range s.faces {
		if f.ClusterID == gid {
			fs = append(fs, f)
		}
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range fs {
			rec := *f
			rec.ClusterID, rec.NotFace = 0, true
			if err := put(tx, bucketFaces, rec.ID, &rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(fs))
	for _, f := range fs {
		s.clusterRemove(c, f)
		f.ClusterID, f.NotFace = 0, true
		ids = append(ids, f.ID)
	}
	return ids, nil
}

// RestoreGroupFaces undoes NotFacesInGroup: the faces are faces again, back
// in their group.
func (s *Store) RestoreGroupFaces(ctx context.Context, gid int64, ids []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var fs []*faceRec
	for _, id := range ids {
		if f := s.faces[id]; f != nil && f.NotFace && f.ClusterID == 0 {
			fs = append(fs, f)
		}
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range fs {
			rec := *f
			rec.ClusterID, rec.NotFace = gid, false
			if err := put(tx, bucketFaces, rec.ID, &rec); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	c := s.clusterOf(gid)
	for _, f := range fs {
		f.ClusterID, f.NotFace = gid, false
		s.clusterAdd(c, f)
	}
	return nil
}
