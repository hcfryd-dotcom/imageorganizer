package store

import (
	"cmp"
	"context"
	"slices"
)

// FaceWhy is everything that decides who a face is, for asking why it is
// (or isn't) filed under someone: GET /api/faces/{id}/why.
type FaceWhy struct {
	Face     FaceView   `json:"face"`
	Px       float32    `json:"px"`             // width in pixels
	Born     float32    `json:"born,omitempty"` // birth year its age and date imply
	Group    WhyGroup   `json:"group"`          // the match group it's in
	Strength string     `json:"strength"`       // how strongly it's known to be Face.PersonID (Strength)
	Not      []string   `json:"not,omitempty"`  // who the user said it isn't (strong)
	Rejected []string   `json:"group_rejected,omitempty"`
	Card     int64      `json:"card_first"` // the photo card's first pick (0: none)
	Files    int64      `json:"would_file"` // the next filing pass files it under them (0: no one)
	Bars     WhyBars    `json:"bars"`
	InPhoto  []WhyFace  `json:"in_photo"`
	People   []WhyMatch `json:"people"` // the likeliest, and anyone asked about
}

// WhyBars are the filing thresholds in use.
type WhyBars struct {
	File, Keep, Lead, KeepLead, AgeUnknown float32
	Px                                     int
}

// WhyGroup is a match group, as it bears on a face.
type WhyGroup struct {
	ID       int64   `json:"id"`
	Size     int     `json:"size"`
	Person   int64   `json:"person,omitempty"`
	Name     string  `json:"name,omitempty"`
	Like     float32 `json:"like,omitempty"`     // as a whole, to the person in question
	Shared   int     `json:"shared,omitempty"`   // photos shared with the face's group
	Strength string  `json:"strength,omitempty"` // how strongly its faces there are its person's
	Counts   bool    `json:"counts,omitempty"`   // whether someoneElse counts it
}

// WhyFace is another face in the photo, and who it counts as.
type WhyFace struct {
	ID       int64  `json:"id"`
	Person   int64  `json:"person,omitempty"`
	Name     string `json:"name,omitempty"`
	Strength string `json:"strength,omitempty"`
	Here     bool   `json:"here"` // counts as them being in the photo (whoIn)
}

// WhyMatch is Match for one person, with what went into it.
type WhyMatch struct {
	Match
	Name     string     `json:"name"`
	CostWhy  string     `json:"cost_why,omitempty"`
	Rival    float32    `json:"rival"` // likeness to someone not named yet seen with them
	RuledOut bool       `json:"ruled_out,omitempty"`
	Else     bool       `json:"someone_else,omitempty"`
	Groups   []WhyGroup `json:"cooc,omitempty"` // groups in photos with the face's group (someoneElse)
}

// ExplainFace is FaceWhy for a face; also for the people in ask.
func (s *Store) ExplainFace(ctx context.Context, faceID int64, ask []int64) (FaceWhy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	f := s.faces[faceID]
	if f == nil {
		return FaceWhy{}, ErrNotFound
	}
	return s.explainFace(s.newMatcher(), f, ask), nil
}

// ExplainImage is FaceWhy for each face in a photo, left to right.
func (s *Store) ExplainImage(ctx context.Context, imageID int64) ([]FaceWhy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.images[imageID] == nil {
		return nil, ErrNotFound
	}
	m := s.newMatcher()
	out := []FaceWhy{}
	for _, id := range s.facesByImage[imageID] {
		if f := s.faces[id]; f != nil && !f.NotFace {
			out = append(out, s.explainFace(m, f, nil))
		}
	}
	slices.SortFunc(out, func(a, b FaceWhy) int { return cmp.Compare(a.Face.Box[0], b.Face.Box[0]) })
	return out, nil
}

// explainFace is FaceWhy; call with s.mu held.
func (s *Store) explainFace(m *matcher, f *faceRec, ask []int64) FaceWhy {
	w := FaceWhy{Face: s.faceView(f), Px: s.facePx(f), InPhoto: []WhyFace{}, People: []WhyMatch{},
		Bars: WhyBars{File: fileSim, Keep: fileKeep, Lead: fileLead, KeepLead: fileKeepLead, AgeUnknown: ageUnknownSim, Px: filePx}}
	if by, ok := s.impliedBirth(f); ok {
		w.Born = by
	}
	_, st := s.faceStrength(f)
	w.Strength = st.String()
	name := func(pid int64) string {
		if p := s.people[pid]; p != nil {
			return p.Name
		}
		return ""
	}
	for _, q := range f.Not {
		w.Not = append(w.Not, name(q))
	}
	c := s.clusters[f.ClusterID]
	if c != nil {
		w.Group = WhyGroup{ID: c.id, Size: c.n, Person: c.PersonID, Name: name(c.PersonID)}
		for _, q := range c.Rejected {
			w.Rejected = append(w.Rejected, name(q))
		}
	}
	here := s.whoIn(f.ImageID, f.ID)
	for _, id := range s.facesByImage[f.ImageID] {
		o := s.faces[id]
		if o == nil || o.ID == f.ID || o.NotFace {
			continue
		}
		pid, st := s.faceStrength(o)
		if o.Tagged != 0 {
			pid = o.Tagged
		}
		w.InPhoto = append(w.InPhoto, WhyFace{ID: o.ID, Person: pid, Name: name(pid), Strength: st.String(), Here: pid != 0 && here[pid] == o})
	}
	if len(f.emb) == 0 {
		return w
	}
	likes := m.likes(f)
	w.Card = s.cardTop(m, f)
	if who := m.best(f, likes); who != 0 && (c == nil || c.PersonID == 0) && f.Tagged == 0 && m.match(f, who, likes).Sure {
		w.Files = who
	}
	// The likeliest five, and anyone asked about.
	var pids []int64
	for pid := range likes {
		pids = append(pids, pid)
	}
	score := func(pid int64) float32 { return m.score(f, pid, likes[pid], here) }
	slices.SortFunc(pids, func(a, b int64) int { return cmp.Or(cmp.Compare(score(b), score(a)), cmp.Compare(a, b)) })
	pids = pids[:min(5, len(pids))]
	for _, q := range ask {
		if _, ok := likes[q]; ok && !slices.Contains(pids, q) {
			pids = append(pids, q)
		}
	}
	for _, pid := range pids {
		wm := WhyMatch{Match: m.match(f, pid, likes), Name: name(pid), Rival: m.rival(f, pid), RuledOut: m.ruledOut(f, pid, here), Else: m.someoneElse(f, pid)}
		_, wm.CostWhy = m.cost(f, pid)
		if c != nil && c.PersonID == 0 {
			mine := m.groupLike(c, pid)
			for _, h := range m.cooc[c.id] {
				g := WhyGroup{ID: h.id, Size: h.n, Person: h.PersonID, Name: name(h.PersonID), Like: m.groupLike(h, pid)}
				for img := range c.imgs {
					if h.imgs[img] != 0 {
						g.Shared++
					}
				}
				counts := h.PersonID == 0 || h.PersonID == pid
				if !counts {
					st := s.sharedStrength(c, h)
					g.Strength, counts = st.String(), st < elseStrength
				}
				g.Counts = counts && g.Like >= mine
				wm.Groups = append(wm.Groups, g)
			}
			slices.SortFunc(wm.Groups, func(a, b WhyGroup) int { return cmp.Compare(b.Like, a.Like) })
		}
		w.People = append(w.People, wm)
	}
	return w
}
