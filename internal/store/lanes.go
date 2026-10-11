package store

import (
	"cmp"
	"context"
	"slices"

	bolt "go.etcd.io/bbolt"
)

// Lane Match: a row per person with the faces most likely theirs. Faces
// that are now near certain are filed under them by themselves (not
// confirmed, so a mistake can't teach more mistakes). On a library copy
// (TestEvalAutoAccept), these cut-offs picked the wrong person for 3 of
// 3,632 confirmed faces (0.08%, the same rate as at 0.70, for twice the
// faces).
const (
	laneAutoSim  = 0.60 // likeness to the person's confirmed groups
	laneAutoLead = 0.15 // over anyone else's
	laneAutoPx   = 40   // face width in pixels, at least
	laneDayBonus = 0.05 // a face from a day the person was photographed (same occasion: same clothes, glasses)
)

// LanePeople picks whose lanes to show first: the people with the most
// likely matches waiting.
func (s *Store) LanePeople(ctx context.Context, n int) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := map[int64]float32{}
	for _, m := range s.faceMatches(300, nil, MatchOptions{}) {
		count[m.Person] += m.Score
	}
	ids := make([]int64, 0, len(count))
	for pid := range count {
		ids = append(ids, pid)
	}
	slices.SortFunc(ids, func(a, b int64) int { return cmp.Or(cmp.Compare(count[b], count[a]), cmp.Compare(a, b)) })
	return ids[:min(n, len(ids))]
}

// Accepted is a face filed under a person by itself, and the group it
// came from (for an undo).
type Accepted struct {
	Face int64 `json:"face"`
	From int64 `json:"from"`
}

// AutoAccept files under person pid the unnamed faces that are now near
// certain theirs: very alike to their confirmed groups and clearly more
// than to anyone else's, their age fitting strictly, their gender not
// against, big enough, and the person not already in the photo. A
// relative in the photo counts for them, as in the suggestions
// (TestEvalKin: a quarter more faces filed, as few wrong).
func (s *Store) AutoAccept(ctx context.Context, pid int64) ([]Accepted, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.personIndex()
	pi := idx[pid]
	if pi == nil || pi.p.Picture {
		return nil, nil
	}
	fam := s.family()
	type ref struct {
		pid int64
		us  [][]float32
	}
	var mine [][]float32
	var others []ref
	for q, qi := range idx {
		r := ref{pid: q}
		for _, g := range qi.groups {
			if !g.Auto {
				r.us = append(r.us, clusterUnit(g))
			}
		}
		if q == pid {
			mine = r.us
		} else if len(r.us) > 0 {
			others = append(others, r)
		}
	}
	if len(mine) == 0 {
		return nil, nil
	}
	var picked []*faceRec
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID != 0 || c.Hidden || f.NotFace || f.Picture || f.Tagged != 0 || len(f.emb) == 0 ||
			slices.Contains(f.Not, pid) || slices.Contains(c.Rejected, pid) || pi.imgs[f.ImageID] || s.facePx(f) < laneAutoPx {
			continue
		}
		u := unitVec(f.emb)
		sim, _ := bestRef(u, mine)
		if sim+kinNext < laneAutoSim {
			continue // even beside a relative
		}
		here := s.whoIn(f.ImageID, f.ID)
		sim += kinWith(f, here, fam[pid])
		if sim < laneAutoSim {
			continue
		}
		by, ok := s.impliedBirth(f)
		if !ok || !pi.ages.fits(by, f.Age, true) || genderMismatch(pi.gender, f) {
			continue
		}
		lead := true
		for _, o := range others {
			if b, _ := bestRef(u, o.us); b+kinWith(f, here, fam[o.pid]) > sim-laneAutoLead {
				lead = false
				break
			}
		}
		if lead {
			picked = append(picked, f)
		}
	}
	var out []Accepted
	for _, f := range picked {
		from, err := s.fileAuto(f, pid)
		if err != nil {
			return out, err
		}
		out = append(out, Accepted{Face: f.ID, From: from})
	}
	return out, nil
}

// fileAuto files one face under a person, not confirmed: in a group of
// its own, filed automatically (or its group, if it is alone in it). It
// returns the group it was in. Call with s.mu held for writing.
func (s *Store) fileAuto(f *faceRec, pid int64) (int64, error) {
	old := s.clusters[f.ClusterID]
	from := f.ClusterID
	if old != nil && old.n == 1 {
		upd := *old
		upd.PersonID, upd.Auto = pid, true
		if err := s.db.Update(func(tx *bolt.Tx) error { return putGroup(tx, &upd) }); err != nil {
			return 0, err
		}
		old.PersonID, old.Auto = pid, true
		return from, nil
	}
	s.nextCluster++
	nc := &cluster{id: s.nextCluster, PersonID: pid, Auto: true}
	rec := *f
	rec.ClusterID = nc.id
	if err := s.db.Update(func(tx *bolt.Tx) error {
		if err := put(tx, bucketFaces, rec.ID, &rec); err != nil {
			return err
		}
		return putGroup(tx, nc)
	}); err != nil {
		return 0, err
	}
	if old != nil {
		s.clusterRemove(old, f)
	}
	f.ClusterID = nc.id
	s.clusters[nc.id] = nc
	s.clusterAdd(nc, f)
	return from, nil
}

// UndoAccept puts a face filed by AutoAccept back as it was: unnamed, in
// the group it came from (if that is still unnamed).
func (s *Store) UndoAccept(ctx context.Context, faceID, from int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.faces[faceID]
	if f == nil {
		return ErrNotFound
	}
	c := s.clusters[f.ClusterID]
	if c == nil || !c.Auto || f.Tagged != 0 {
		return nil // decided otherwise since
	}
	if c.n == 1 && (f.ClusterID == from || s.clusters[from] == nil || s.clusters[from].PersonID != 0) {
		// Its own group: unnamed again.
		upd := *c
		upd.PersonID, upd.Auto = 0, false
		if err := s.db.Update(func(tx *bolt.Tx) error { return putGroup(tx, &upd) }); err != nil {
			return err
		}
		c.PersonID, c.Auto = 0, false
		return nil
	}
	back := s.clusters[from]
	if back == nil || back.PersonID != 0 {
		return nil
	}
	rec := *f
	rec.ClusterID = from
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, rec.ID, &rec) }); err != nil {
		return err
	}
	s.clusterRemove(c, f)
	f.ClusterID = from
	s.clusterAdd(back, f)
	return nil
}
