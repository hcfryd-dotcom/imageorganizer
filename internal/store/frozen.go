package store

import (
	"cmp"
	"context"
	"slices"

	bolt "go.etcd.io/bbolt"
)

// A picture on the wall (a framed photo, a poster) shows someone at the
// same age in photos taken years apart, while they themselves age. For a
// person, the faces that look like them but clearly younger than they
// were that day can't be them: they're the evidence (seeds). Faces alike
// to a seed (photographed from another angle, in other light, a picture
// is only so alike) at the same apparent age join it: those are the same
// picture, earlier on too, when its age was still plausible.
const (
	frozenLike    = 0.60 // a face this alike to the person is looked at
	frozenAlike   = 0.70 // alike to a seed, to join its set
	frozenAge     = 1.5  // at the same apparent age, within
	frozenYounger = 3    // years younger than they were then, at least (and the age tolerance plus 2), to be a seed
	frozenSeeds   = 2
	frozenPhotos  = 3
	frozenSpan    = 365 * 24 * 3600
	frozenCloseUp = 0.12 // face width, as a share of the photo's, of a close-up (an original, not a print on the wall)
	frozenSlope   = 0.35 // a living person's apparent age rises about a year a year; a picture's, less than this
	frozenInTime  = 0.25 // share of an adult set's faces whose age fits the person's then, from which it's them, not a picture
)

// FrozenSet is a face of a person that keeps the same age across the
// years: a picture on the wall, most likely.
type FrozenSet struct {
	Faces  []FaceView `json:"faces"` // all of them, oldest photo first
	Photos int        `json:"photos"`
	From   int64      `json:"from"`
	To     int64      `json:"to"`
	Age    float32    `json:"age"` // the apparent age it keeps
	// InTime: faces whose age fits the person's then: likely the real
	// photos the picture was made from (offered unticked).
	InTime []int64 `json:"in_time,omitempty"`
	// With: others on the same wall (in the picture, or a frame beside it):
	// faces at its height and about its size in many of its photos, alike
	// each time, their age not moving either.
	With []FrozenWith `json:"with,omitempty"`
}

// FrozenWith is someone else in the same picture.
type FrozenWith struct {
	Person int64      `json:"person,omitempty"` // whom they look most like (0: no one named)
	Name   string     `json:"name,omitempty"`
	Faces  []FaceView `json:"faces"`
}

const (
	frozenWithShare   = 0.10 // in this share of the picture's photos, at least (a frame beside it is not in every shot)
	frozenWithAlike   = 0.60 // alike to each other
	frozenWithName    = 0.45 // alike to someone named, to be named after them
	frozenWithReach   = 25   // face widths to the side, at most
	frozenWithLevel   = 3    // face heights up or down, at most
	frozenWithSibling = 0.05 // the person themselves, if this close to the likeliest
)

// FrozenFor finds pictures of person pid (on a wall, say): faces that look
// like them at an age they had long passed, and the faces of the same
// picture, biggest set first. Faces marked as pictures already, or said to
// be them for real (Live), are left out.
func (s *Store) FrozenFor(ctx context.Context, pid int64) []FrozenSet {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pi := s.personIndex()[pid]
	if pi == nil || pi.p.Picture || !pi.ages.ok {
		return nil
	}
	var refs [][]float32
	for _, g := range pi.groups {
		if !g.Auto {
			refs = append(refs, clusterUnit(g))
		}
	}
	if len(refs) == 0 {
		return nil
	}
	type face struct {
		f      *faceRec
		u      []float32
		t      int64
		seed   bool
		inTime bool
	}
	var cands []*faceRec
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if f.NotFace || f.Picture || f.Live || len(f.emb) == 0 || f.AgeVersion == 0 || f.Age < 0 ||
			c == nil || c.Hidden || c.PersonID != 0 && c.PersonID != pid {
			continue
		}
		if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil && !scanned(im) {
			cands = append(cands, f)
		}
	}
	sims := maxSims(len(cands), func(i int) []float32 { return unitVec(cands[i].emb) }, refs)
	var pool []face
	for i, f := range cands {
		if sims[i] < frozenLike {
			continue
		}
		t := *s.images[f.ImageID].TakenAt
		x := face{f: f, u: unitVec(f.emb), t: t}
		// Clearly younger than they were then: not them.
		year := float32(t)/(365.25*86400) + 1970
		real, ok := pi.ages.looksAt(year)
		if !ok && pi.ages.birth > 0 {
			real, ok = year-pi.ages.birth, true
		}
		if ok && real < 0 {
			continue // before they were born: the photo's date is wrong, not a picture
		}
		if ok {
			x.seed = real-f.Age >= max(frozenYounger, birthTolerance(real)+2)
			// An original: the age fits then, and the face is a close-up,
			// not a small print on a wall across the room.
			x.inTime = abs32(real-f.Age) <= birthTolerance(real) && f.Box[2]-f.Box[0] >= frozenCloseUp
		}
		pool = append(pool, x)
	}
	// Sets: each seed with the faces alike to it at its age.
	root := make([]int, len(pool))
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(a int) int {
		for root[a] != a {
			root[a] = root[root[a]]
			a = root[a]
		}
		return a
	}
	type pair struct{ a, b int }
	found := make([][]pair, len(pool))
	parallel(len(pool), func(i int) {
		if !pool[i].seed {
			return
		}
		for j := range pool {
			if j != i && pool[j].f.ImageID != pool[i].f.ImageID && abs32(pool[j].f.Age-pool[i].f.Age) <= frozenAge &&
				dot(pool[i].u, pool[j].u) >= frozenAlike {
				found[i] = append(found[i], pair{i, j})
			}
		}
	})
	for _, ps := range found {
		for _, p := range ps {
			root[find(p.a)] = find(p.b)
		}
	}
	sets := map[int][]face{}
	for i, x := range pool {
		sets[find(i)] = append(sets[find(i)], x) // lone faces are dropped below (no seeds, or too few photos)
	}
	var out []FrozenSet
	for _, set := range sets {
		seeds := 0
		photos := map[int64]bool{}
		var from, to int64
		var ages float32
		for _, x := range set {
			if x.seed {
				seeds++
			}
			photos[x.f.ImageID] = true
			if from == 0 || x.t < from {
				from = x.t
			}
			to = max(to, x.t)
			ages += x.f.Age
		}
		if seeds < frozenSeeds || len(photos) < frozenPhotos || to-from < frozenSpan {
			continue
		}
		// An adult whose faces are many of them their age then: it's
		// them, photographed as they were (an adult's apparent age barely
		// moves in twenty years), not a picture. A child's picture is told
		// by their age (which moves fast), its first years in time or not.
		inTime := 0
		for _, x := range set {
			if x.inTime {
				inTime++
			}
		}
		if ages/float32(len(set)) >= ageChild && float32(inTime) >= frozenInTime*float32(len(set)) {
			continue
		}
		// Ageing with its photos, a year a year: a living child growing
		// up (someone else, or them with their ages read wrong), not a
		// picture.
		all := make([]*faceRec, len(set))
		idx := make([]int, len(set))
		for i, x := range set {
			all[i], idx[i] = x.f, i
		}
		if s.agesMove(all, idx) {
			continue
		}
		slices.SortFunc(set, func(a, b face) int { return cmp.Compare(a.t, b.t) })
		fz := FrozenSet{Photos: len(photos), From: from, To: to, Age: ages / float32(len(set))}
		var fs []*faceRec
		for _, x := range set {
			fz.Faces = append(fz.Faces, s.faceView(x.f))
			fs = append(fs, x.f)
			if x.inTime {
				fz.InTime = append(fz.InTime, x.f.ID)
			}
		}
		fz.With = s.frozenWith(fs, pid, len(photos))
		out = append(out, fz)
	}
	slices.SortFunc(out, func(a, b FrozenSet) int { return cmp.Compare(b.Photos, a.Photos) })
	return out
}

// frozenWith finds the others on the same wall as a picture (in it, or in
// a frame beside it): in the photos of its faces (set), the faces at its
// height and about its size, alike to each other across many of those
// photos, whose age doesn't move either. Call with s.mu held.
func (s *Store) frozenWith(set []*faceRec, pid int64, photos int) []FrozenWith {
	var cs []*faceRec
	for _, x := range set {
		w, h := x.Box[2]-x.Box[0], x.Box[3]-x.Box[1]
		for _, fid := range s.facesByImage[x.ImageID] {
			o := s.faces[fid]
			if o == nil || o.ID == x.ID || o.NotFace || o.Live || len(o.emb) == 0 {
				continue
			}
			ow := o.Box[2] - o.Box[0]
			dx := abs32((o.Box[0]+o.Box[2])/2 - (x.Box[0]+x.Box[2])/2)
			dy := abs32((o.Box[1]+o.Box[3])/2 - (x.Box[1]+x.Box[3])/2)
			if dx <= frozenWithReach*w && dy <= frozenWithLevel*h && ow >= w/2 && ow <= 1.5*w {
				cs = append(cs, o)
			}
		}
	}
	us := make([][]float32, len(cs))
	for i, o := range cs {
		us[i] = unitVec(o.emb)
	}
	root := make([]int, len(cs))
	for i := range root {
		root[i] = i
	}
	var find func(int) int
	find = func(a int) int {
		for root[a] != a {
			root[a] = root[root[a]]
			a = root[a]
		}
		return a
	}
	for i := range cs {
		for j := i + 1; j < len(cs); j++ {
			if cs[i].ImageID != cs[j].ImageID && dot(us[i], us[j]) >= frozenWithAlike {
				root[find(i)] = find(j)
			}
		}
	}
	groups := map[int][]int{}
	for i := range cs {
		groups[find(i)] = append(groups[find(i)], i)
	}
	idx := s.personIndex()
	var out []FrozenWith
	for _, g := range groups {
		imgs := map[int64]bool{}
		for _, i := range g {
			imgs[cs[i].ImageID] = true
		}
		if float32(len(imgs)) < frozenWithShare*float32(photos) || len(imgs) < frozenPhotos || s.agesMove(cs, g) {
			continue
		}
		// Whom they look like, whatever the age: it is a picture. Without
		// age, siblings can't be told apart: on the same wall as their own
		// picture, the person is preferred when about as alike.
		w := FrozenWith{}
		best, own := float32(frozenWithName), float32(-1)
		for q, qi := range idx {
			if qi.p.Picture {
				continue
			}
			var refs [][]float32
			for _, c := range qi.groups {
				if !c.Auto {
					refs = append(refs, clusterUnit(c))
				}
			}
			var sum float32
			for _, i := range g {
				b, _ := bestRef(us[i], refs)
				sum += b
			}
			m := sum / float32(len(g))
			if q == pid {
				own = m
			}
			if m > best {
				best, w.Person, w.Name = m, q, qi.p.Name
			}
		}
		if w.Person != pid && own >= frozenWithName && own >= best-frozenWithSibling {
			w.Person, w.Name = pid, idx[pid].p.Name
		}
		for _, i := range g {
			w.Faces = append(w.Faces, s.faceView(cs[i]))
		}
		out = append(out, w)
	}
	slices.SortFunc(out, func(a, b FrozenWith) int { return cmp.Compare(len(b.Faces), len(a.Faces)) })
	return out
}

// agesMove reports whether the faces cs[g] age with their photos (a year
// a year, roughly): then they're a living person, not a picture.
func (s *Store) agesMove(cs []*faceRec, g []int) bool {
	var n, sx, sy, sxx, sxy float64
	for _, i := range g {
		f := cs[i]
		im := s.images[f.ImageID]
		if f.AgeVersion == 0 || f.Age < 0 || im == nil || im.TakenAt == nil {
			continue
		}
		x, y := float64(*im.TakenAt)/(365.25*86400), float64(f.Age)
		n, sx, sy, sxx, sxy = n+1, sx+x, sy+y, sxx+x*x, sxy+x*y
	}
	v := n*sxx - sx*sx
	return n >= 3 && v > 0 && (n*sxy-sx*sy)/v >= frozenSlope
}

// SetFrozen answers "a picture of them on the wall?" for faces: yes files
// them under person pid, as a picture of them; no marks them as not a
// picture, not to be asked about again.
func (s *Store) SetFrozen(ctx context.Context, pid int64, ids []int64, picture bool) error {
	if picture {
		for _, id := range ids {
			if pid == 0 { // no one named: a picture of someone
				if err := s.SetFacePicture(ctx, id, true); err != nil && err != ErrNotFound {
					return err
				}
				continue
			}
			s.mu.RLock()
			f := s.faces[id]
			mine := f != nil && (f.Tagged == pid || s.clusters[f.ClusterID] != nil && s.clusters[f.ClusterID].PersonID == pid)
			s.mu.RUnlock()
			if f == nil {
				continue
			}
			if !mine {
				if _, err := s.TagFace(ctx, id, pid, ""); err != nil {
					return err
				}
			}
			if err := s.SetFacePicture(ctx, id, true); err != nil {
				return err
			}
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var recs []faceRec
	for _, id := range ids {
		if f := s.faces[id]; f != nil && !f.Live {
			r := *f
			r.Live = true
			recs = append(recs, r)
		}
	}
	if err := s.db.Update(func(tx *bolt.Tx) error {
		for i := range recs {
			if err := put(tx, bucketFaces, recs[i].ID, &recs[i]); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, r := range recs {
		s.faces[r.ID].Live = true
	}
	return nil
}
