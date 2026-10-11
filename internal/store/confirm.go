package store

import (
	"cmp"
	"context"
	"slices"

	bolt "go.etcd.io/bbolt"
)

// A person is the faces the user saw and said are them (tagged), and the
// faces filed under them automatically because they are near certain
// (FileFaces). Both live in the person's groups, one per stretch of their
// life; unnamed groups hold everyone else.

// ConfirmFaces says the faces yes are person personID (or a person by
// name, new or existing), and the faces no are not: what the user saw in
// a batch, the rest ticked off. It returns the person's id.
func (s *Store) ConfirmFaces(ctx context.Context, personID int64, name string, yes, no []int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, newPerson, err := s.resolvePerson(personID, name)
	if err != nil {
		return 0, err
	}
	if newPerson {
		if err := s.db.Update(func(tx *bolt.Tx) error {
			id, err := nextID(tx, bucketPeople)
			if err != nil {
				return err
			}
			p.ID = id
			return put(tx, bucketPeople, p.ID, p)
		}); err != nil {
			return 0, err
		}
		s.people[p.ID] = p
	}
	ch := s.newChanges()
	for _, id := range yes {
		if f := s.faces[id]; f != nil && !f.NotFace && f.Tagged != p.ID {
			f.Not = slices.DeleteFunc(slices.Clone(f.Not), func(q int64) bool { return q == p.ID })
			ch.group(s.placeConfirmed(f, p.ID))
			ch.face(f)
		}
	}
	for _, id := range no {
		f := s.faces[id]
		if f == nil {
			continue
		}
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID == p.ID {
			ch.group(s.unfile(f))
		}
		if !slices.Contains(f.Not, p.ID) {
			f.Not = append(slices.Clone(f.Not), p.ID)
		}
		ch.face(f)
	}
	return p.ID, ch.save()
}

// PersonFaces lists a person's faces, confirmed and filed, the newest
// photos first.
func (s *Store) PersonFaces(ctx context.Context, pid int64) []FaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []FaceView
	for _, f := range s.faces {
		if c := s.clusters[f.ClusterID]; c != nil && c.PersonID == pid && !f.NotFace {
			out = append(out, s.faceView(f))
		}
	}
	taken := func(v FaceView) int64 {
		if v.Taken != nil {
			return *v.Taken
		}
		return 0
	}
	slices.SortFunc(out, func(a, b FaceView) int { return cmp.Or(cmp.Compare(taken(b), taken(a)), cmp.Compare(a.ID, b.ID)) })
	return out
}

// MarkSeen confirms faces the user saw as person pid.
func (s *Store) MarkSeen(ctx context.Context, pid int64, faceIDs []int64) error {
	_, err := s.ConfirmFaces(ctx, pid, "", faceIDs, nil)
	return err
}

// unfile takes face f out of its person's group into the unnamed group it
// fits, untagged. It changes memory only and returns the new group. Call
// with s.mu held for writing.
func (s *Store) unfile(f *faceRec) *cluster {
	if old := s.clusters[f.ClusterID]; old != nil {
		s.clusterRemove(old, f)
	}
	f.Tagged, f.ClusterID = 0, 0
	c := s.clusterOf(s.bestCluster(f))
	f.ClusterID = c.id
	s.clusterAdd(c, f)
	return c
}

// placeFiled files face f under person pid automatically: into their
// group it fits, not tagged. Call with s.mu held for writing.
func (s *Store) placeFiled(f *faceRec, pid int64) *cluster {
	c := s.placeConfirmed(f, pid)
	f.Tagged = 0
	return c
}

// changes collects the faces and groups a decision touched, to store them
// in one transaction.
type changes struct {
	s      *Store
	faces  map[int64]*faceRec
	groups map[int64]*cluster
}

func (s *Store) newChanges() *changes {
	return &changes{s: s, faces: map[int64]*faceRec{}, groups: map[int64]*cluster{}}
}

func (ch *changes) face(f *faceRec)  { ch.faces[f.ID] = f }
func (ch *changes) group(c *cluster) { ch.groups[c.id] = c }

func (ch *changes) save() error {
	if len(ch.faces) == 0 && len(ch.groups) == 0 {
		return nil
	}
	return ch.s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range ch.faces {
			if err := put(tx, bucketFaces, f.ID, f); err != nil {
				return err
			}
		}
		for _, c := range ch.groups {
			if err := putGroup(tx, c); err != nil {
				return err
			}
		}
		return nil
	})
}
