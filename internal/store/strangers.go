package store

import (
	"context"
	"slices"

	bolt "go.etcd.io/bbolt"
)

// eventGap is the longest pause within one event: photos taken closer
// together than this, one after another, belong to the same occasion.
const eventGap = 2 * 3600

// Event is the occasion a photo was taken at, and the people at it nobody
// has named.
type Event struct {
	Photos int     `json:"photos"`
	From   int64   `json:"from"`
	To     int64   `json:"to"`
	Faces  int     `json:"faces"`  // unnamed faces in its photos
	Groups []int64 `json:"groups"` // groups hidden as strangers (when done)
	// With details: the photos, each one's unnamed faces, and the unnamed
	// faces that look like someone named, to ask about first.
	Images  []Image       `json:"images,omitempty"`
	Unnamed map[int64]int `json:"unnamed,omitempty"`
	Likely  []FaceMatch   `json:"likely,omitempty"`
	ids     []int64
}

// eventLikely is how alike an unnamed face at an event must be to someone
// named to be asked about before hiding (stricter than the matcher: an
// event has thousands of faces).
const eventLikely = 0.40

// event finds the photos taken at the same occasion as image id: the run
// of dated photos, from any folder, around it with no pause longer than
// eventGap. An undated photo is an event of its own. Call with s.mu held.
func (s *Store) event(id int64) (Event, error) {
	im := s.images[id]
	if im == nil {
		return Event{}, ErrNotFound
	}
	if im.TakenAt == nil {
		return Event{Photos: 1, ids: []int64{id}}, nil
	}
	var list []*imageRec
	for _, o := range s.images {
		if o.TakenAt != nil && s.present(o.ID) {
			list = append(list, o)
		}
	}
	sortImages(list, "taken_asc")
	i := slices.Index(list, im)
	if i < 0 {
		return Event{Photos: 1, ids: []int64{id}, From: *im.TakenAt, To: *im.TakenAt}, nil
	}
	lo, hi := i, i
	for lo > 0 && *list[lo].TakenAt-*list[lo-1].TakenAt <= eventGap {
		lo--
	}
	for hi < len(list)-1 && *list[hi+1].TakenAt-*list[hi].TakenAt <= eventGap {
		hi++
	}
	ev := Event{Photos: hi - lo + 1, From: *list[lo].TakenAt, To: *list[hi].TakenAt}
	for _, o := range list[lo : hi+1] {
		ev.ids = append(ev.ids, o.ID)
	}
	return ev, nil
}

// strangerFace reports whether a face is someone nobody has named: not a
// person's (not even filed automatically), not tagged, not hidden already.
func (s *Store) strangerFace(f *faceRec) bool {
	c := s.clusters[f.ClusterID]
	return c != nil && c.PersonID == 0 && !c.Hidden && f.Tagged == 0 && !f.NotFace
}

// EventStrangers previews hiding the people nobody has named at the event
// image id was taken at; with details, the photos and the faces that look
// like someone named too.
func (s *Store) EventStrangers(ctx context.Context, id int64, details bool) (Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev, err := s.event(id)
	if err != nil {
		return ev, err
	}
	if details {
		ev.Unnamed = map[int64]int{}
	}
	for _, img := range ev.ids {
		for _, fid := range s.facesByImage[img] {
			if f := s.faces[fid]; f != nil && s.strangerFace(f) {
				ev.Faces++
				if details {
					ev.Unnamed[img]++
				}
			}
		}
	}
	if !details {
		return ev, nil
	}
	only := map[int64]bool{}
	for _, img := range ev.ids {
		ev.Images = append(ev.Images, s.view(s.images[img]))
		only[img] = true
	}
	for _, m := range s.faceMatches(200, only, MatchOptions{}) {
		if m.Score >= eventLikely {
			ev.Likely = append(ev.Likely, m)
		}
	}
	return ev, nil
}

// HideEventStrangers hides everyone nobody has named at the event image id
// was taken at (performers, an audience), as strangers, except in the
// photos in skipImages and the faces in keepFaces. A group seen only there
// is hidden as a whole; one seen elsewhere too gives up just its faces
// from the event, to a hidden group of their own. It returns the hidden
// groups, for an undo (unhiding them).
func (s *Store) HideEventStrangers(ctx context.Context, id int64, skipImages, keepFaces []int64) (Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ev, err := s.event(id)
	if err != nil {
		return ev, err
	}
	byGroup := map[int64][]*faceRec{}
	for _, img := range ev.ids {
		if slices.Contains(skipImages, img) {
			continue
		}
		for _, fid := range s.facesByImage[img] {
			if f := s.faces[fid]; f != nil && s.strangerFace(f) && !slices.Contains(keepFaces, fid) {
				byGroup[f.ClusterID] = append(byGroup[f.ClusterID], f)
			}
		}
	}
	type move struct {
		from, to *cluster
		faces    []*faceRec
	}
	var hide []*cluster
	var moves []move
	for gid, fs := range byGroup {
		c := s.clusters[gid]
		ev.Faces += len(fs)
		if len(fs) >= c.n {
			hide = append(hide, c)
			continue
		}
		s.nextCluster++
		moves = append(moves, move{c, &cluster{id: s.nextCluster, Hidden: true, Rejected: slices.Clone(c.Rejected)}, fs})
	}
	err = s.db.Update(func(tx *bolt.Tx) error {
		for _, c := range hide {
			upd := *c
			upd.Hidden = true
			if err := putGroup(tx, &upd); err != nil {
				return err
			}
		}
		for _, m := range moves {
			if err := putGroup(tx, m.to); err != nil {
				return err
			}
			for _, f := range m.faces {
				rec := *f
				rec.ClusterID = m.to.id
				if err := put(tx, bucketFaces, rec.ID, &rec); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return ev, err
	}
	for _, c := range hide {
		c.Hidden = true
		ev.Groups = append(ev.Groups, c.id)
	}
	for _, m := range moves {
		s.clusters[m.to.id] = m.to
		for _, f := range m.faces {
			s.clusterRemove(m.from, f)
			f.ClusterID = m.to.id
			s.clusterAdd(m.to, f)
		}
		ev.Groups = append(ev.Groups, m.to.id)
	}
	return ev, nil
}

// UnhideGroups shows hidden groups again (undoing HideEventStrangers).
func (s *Store) UnhideGroups(ctx context.Context, ids []int64) error {
	for _, id := range ids {
		if err := s.HideGroup(ctx, id, false); err != nil && err != ErrNotFound {
			return err
		}
	}
	return nil
}
