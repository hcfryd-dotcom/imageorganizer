package store

import (
	"cmp"
	"context"
	"slices"
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	faceGroupMin = 0.20 // a group's likeness to a person for its faces to be looked at one by one
	faceCandMin  = 0.30 // a single face's likeness to a person's groups to be offered
	faceGroups   = 400  // most groups looked at
	familyFloor  = 0.15 // for a small child on a day their family was photographed, age fitting: they look little like their older selves
	batchFloor   = 0.30 // a score (Match) below this isn't offered: under it, fewer than half are them (TestEvalCandScore)
)

// FaceCandidates returns single faces that may be the person, for "Is this
// also <name>?" one face at a time: faces filed as them automatically and
// not confirmed yet, and faces in unnamed groups that look like them. A
// group can hold several people, so each face is scored on its own against
// the person's groups, and its own age is checked; best first.
func (s *Store) FaceCandidates(ctx context.Context, personID int64, limit int) []FaceView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.faceCandidates(s.newMatcher(), personID, limit)
}

// faceCandidates is FaceCandidates, with a matcher made under the same
// lock (or one made a moment before, for many people at once).
func (s *Store) faceCandidates(m *matcher, personID int64, limit int) []FaceView {
	idx := m.idx
	pi := idx[personID]
	if pi == nil {
		return nil
	}
	// The groups the user confirmed are what they look like; automatic
	// ones only when there is nothing else.
	var refs [][]float32
	for _, g := range pi.groups {
		if !g.Auto {
			refs = append(refs, clusterUnit(g))
		}
	}
	if len(refs) == 0 {
		for _, g := range pi.groups {
			refs = append(refs, clusterUnit(g))
		}
	}
	if len(refs) == 0 {
		return nil
	}
	type scored struct {
		c     *cluster
		score float32
	}
	var auto []*cluster
	var cs []*cluster
	for _, c := range s.clusters {
		switch {
		case c.n == 0 || c.Hidden:
		case c.PersonID == personID && c.Auto:
			auto = append(auto, c)
		case c.PersonID == 0 && !slices.Contains(c.Rejected, personID):
			cs = append(cs, c)
		}
	}
	sims := maxSims(len(cs), func(i int) []float32 { return clusterUnit(cs[i]) }, refs)
	var groups []scored
	for i, c := range cs {
		if sims[i] >= faceGroupMin {
			groups = append(groups, scored{c, sims[i]})
		}
	}
	slices.SortFunc(groups, func(a, b scored) int { return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.c.id, b.c.id)) })
	groups = groups[:min(len(groups), faceGroups)]
	for _, c := range auto {
		groups = append(groups, scored{c: c})
	}
	byGroup := facesOfGroups(s, groups, func(g scored) int64 { return g.c.id })

	var fs []*faceRec
	for _, g := range groups {
		for _, f := range byGroup[g.c.id] {
			if f.NotFace || f.Picture || f.Tagged != 0 || len(f.emb) == 0 || slices.Contains(f.Not, personID) {
				continue
			}
			if g.c.PersonID == 0 && (f.bg || pi.imgs[f.ImageID]) {
				continue // a passer-by, or the person is already in the photo
			}
			fs = append(fs, f) // the age is checked once its likeness is known
		}
	}
	// Faces on the days their family (usual company) was photographed:
	// looked at with a lower likeness, if their age fits.
	dayScore, dayWho := s.dayCompany(personID)
	seen := make(map[int64]bool, len(fs))
	for _, f := range fs {
		seen[f.ID] = true
	}
	for _, f := range s.faces {
		im := s.images[f.ImageID]
		if seen[f.ID] || im == nil || im.TakenAt == nil || dayScore[*im.TakenAt/86400] < familyDay {
			continue
		}
		c := s.clusters[f.ClusterID]
		if c == nil || c.Hidden || c.PersonID != 0 || f.NotFace || f.Picture || f.Tagged != 0 || f.bg ||
			len(f.emb) == 0 || slices.Contains(f.Not, personID) || slices.Contains(c.Rejected, personID) || pi.imgs[f.ImageID] {
			continue
		}
		// A young face (older ones match well enough on likeness), whose
		// age is known and fits: here it does most of the work.
		if f.Age > familyAge {
			continue
		}
		if by, ok := s.impliedBirth(f); !ok || !pi.ages.ok || !pi.ages.fits(by, f.Age, true) {
			continue
		}
		seen[f.ID] = true
		fs = append(fs, f)
	}
	// Faces in photos with a relative of theirs: relatives cling together.
	rel := s.relatives(personID)
	if len(rel) > 0 {
		kinImgs := map[int64]bool{}
		for _, f := range s.faces {
			if c := s.clusters[f.ClusterID]; c != nil && !c.Auto && rel[c.PersonID] != "" || rel[f.Tagged] != "" {
				kinImgs[f.ImageID] = true
			}
		}
		for img := range kinImgs {
			if pi.imgs[img] {
				continue
			}
			for _, fid := range s.facesByImage[img] {
				f := s.faces[fid]
				c := s.clusters[f.ClusterID]
				if f == nil || seen[f.ID] || c == nil || c.Hidden || c.PersonID != 0 || f.NotFace || f.Picture || f.Tagged != 0 ||
					f.bg || len(f.emb) == 0 || slices.Contains(f.Not, personID) || slices.Contains(c.Rejected, personID) {
					continue
				}
				seen[f.ID] = true
				fs = append(fs, f)
			}
		}
	}
	scores := maxSims(len(fs), func(i int) []float32 { return unitVec(fs[i].emb) }, refs)
	// The days they were photographed (confirmed): a face from the same
	// occasion, in another photo, likely wears the same clothes and
	// glasses, and is more likely them.
	ownDays := map[int64]bool{}
	for _, g := range pi.groups {
		if !g.Auto {
			for d := range g.days {
				ownDays[d] = true
			}
		}
	}
	// Scored as everywhere (Match): likeness and a relative beside it.
	// Family photographed that day, or the person themselves, only let a
	// face be looked at, and say why.
	var out []FaceView
	for i, f := range fs {
		day := int64(-1)
		if im := s.images[f.ImageID]; im != nil && im.TakenAt != nil {
			day = *im.TakenAt / 86400
		}
		fam := dayScore[day]
		kin, who, next := s.kinNear(f, rel)
		famOK := fam >= familyDay && scores[i] >= familySim && f.Age <= familyAge
		// Let in on a relative's account only with a known age that fits:
		// a tiny face beside them could be anyone, a child among them.
		_, ageOK := s.impliedBirth(f)
		kinOK := kin > 0 && scores[i] >= kinSim && ageOK
		if s.clusters[f.ClusterID].PersonID == 0 && scores[i] < faceCandMin && !famOK && !kinOK {
			continue
		}
		sc := m.score(f, personID, scores[i], s.whoIn(f.ImageID, f.ID))
		if sc < batchFloor && (!famOK || sc < familyFloor) {
			continue // too unlike, or the age or gender says otherwise
		}
		v := s.faceView(f)
		v.Score = min(1, sc)
		if fam >= familyDay {
			v.Company = dayWho[day]
		}
		if kin > 0 {
			v.Why = s.kinWhy(who, next, rel)
		}
		if ownDays[day] && !pi.imgs[f.ImageID] && v.Why == "" {
			v.Why = "same day as a photo of " + pi.p.Name
		}
		out = append(out, v)
	}
	// Only faces whose photo card, looking at the face alone, offers them
	// first (not someone else named, nor no one), and not of a group in
	// photos with them: someone else, likely a relative.
	drop := make([]bool, len(out))
	parallel(len(out), func(i int) {
		f := s.faces[out[i].ID]
		drop[i] = s.cardTop(m, f) != personID || m.someoneElse(f, personID)
	})
	kept := out[:0]
	for i, v := range out {
		if !drop[i] {
			kept = append(kept, v)
		}
	}
	out = s.rankByQuality(kept, limit)
	// Why each isn't filed by itself.
	parallel(len(out), func(i int) {
		f := s.faces[out[i].ID]
		out[i].Hold = m.match(f, personID, m.likes(f)).Hold
	})
	return out
}

// rankByQuality orders candidate faces as the matcher does: by score, but
// faces turned away, small or blurry lower, and dark or faint smudges left
// out (measured on their crops, for the likeliest few).
func (s *Store) rankByQuality(out []FaceView, limit int) []FaceView {
	// Faces too small to judge (the age model gave up on them) say
	// nothing anyone can confirm.
	out = slices.DeleteFunc(out, func(v FaceView) bool { f := s.faces[v.ID]; return f.AgeVersion > 0 && f.Age < 0 })
	rank := make([]float32, len(out))
	for i, v := range out {
		f := s.faces[v.ID]
		pose := matchPose + (1-matchPose)*s.frontal(f)
		size := matchSize + (1-matchSize)*min(1, s.facePx(f)/matchPx)
		det := min(1, max(0, (f.Score-matchDet0)/(matchDet1-matchDet0)))
		rank[i] = v.Score * pose * size * det
	}
	order := make([]int, len(out))
	for i := range order {
		order[i] = i
	}
	byRank := func(a, b int) int { return cmp.Or(cmp.Compare(rank[b], rank[a]), cmp.Compare(out[a].ID, out[b].ID)) }
	slices.SortFunc(order, byRank)
	order = order[:min(len(order), matchLook*limit)]
	vis := make([]float32, len(out))
	parallel(len(order), func(k int) { vis[order[k]] = s.visible(s.faces[out[order[k]].ID]) })
	order = slices.DeleteFunc(order, func(i int) bool { return vis[i] == 0 })
	for _, i := range order {
		rank[i] *= vis[i]
	}
	slices.SortStableFunc(order, byRank)
	res := make([]FaceView, 0, min(len(order), limit))
	for _, i := range order[:min(len(order), limit)] {
		res = append(res, out[i])
	}
	return res
}

// maxSims scores n vectors (from vec) by their best cosine similarity to
// any of refs (all unit length), in parallel.
func maxSims(n int, vec func(int) []float32, refs [][]float32) []float32 {
	out := make([]float32, n)
	parallel(n, func(i int) { out[i], _ = bestRef(vec(i), refs) })
	return out
}

func unitVec(v []float32) []float32 {
	n := float32(norm(v))
	u := make([]float32, len(v))
	if n > 0 {
		for i, x := range v {
			u[i] = x / n
		}
	}
	return u
}

// SkipFace puts a face off in the matcher: it goes to the back of the
// queue, and stays there across reloads.
func (s *Store) SkipFace(ctx context.Context, faceID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f := s.faces[faceID]
	if f == nil {
		return ErrNotFound
	}
	rec := *f
	rec.Skipped = time.Now().UnixNano()
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, rec.ID, &rec) }); err != nil {
		return err
	}
	f.Skipped = rec.Skipped
	return nil
}

// RejectFace says this one face is not the person. A face filed as them
// leaves its group (RemoveFace); any other face just remembers it, as its
// group may still be someone else entirely, or even them.
func (s *Store) RejectFace(ctx context.Context, faceID, personID int64) error {
	s.mu.RLock()
	f, ok := s.faces[faceID]
	var filed bool
	if ok {
		c := s.clusters[f.ClusterID]
		filed = c != nil && c.PersonID == personID || f.Tagged == personID
	}
	s.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	if filed {
		return s.RemoveFace(ctx, faceID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if f = s.faces[faceID]; f == nil {
		return ErrNotFound
	}
	if slices.Contains(f.Not, personID) {
		return nil
	}
	rec := *f
	rec.Not = append(slices.Clone(f.Not), personID)
	if err := s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFaces, rec.ID, &rec) }); err != nil {
		return err
	}
	f.Not = rec.Not
	return nil
}
