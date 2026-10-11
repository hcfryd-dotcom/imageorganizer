package store

import (
	"cmp"
	"context"
	"slices"
)

// Faces filed under someone that don't fit them: the wrong age for the
// photo's date, the other gender, or unlike the rest of their group. A
// group is confirmed as a whole, and holds a stray face now and then;
// asked about one by one, a yes tags the face (certain), a no takes it out.
const (
	fitAgeOff  = 8    // years a face's birth year may be off the person's before it's doubted,
	fitAgeSure = 3    // and that many times the uncertainty of its age
	fitAlike   = 0.30 // likeness to the rest of its group below which a face is doubted
	fitOther   = 0.10 // more alike to someone else by this much, to offer them
	fitMinPx   = 40   // smaller faces aren't doubted: their age and looks read too loosely
	fitLook    = 0.55 // likeness below which a face is compared with everyone else
	fitAgeGap  = 2    // years further off someone's birth year than its own person's, to rule them out
	fitDrop    = 0.15 // score lost since the user confirmed it (Approval), to doubt it
	fitAhead   = 0.10 // someone else's score over its person's, to doubt it (and at least fileKeep)
)

// FaceDoubt is a face filed under a person that doesn't seem to fit them.
type FaceDoubt struct {
	Face     FaceView `json:"face"`
	Person   int64    `json:"person"`
	Name     string   `json:"name"`
	Cover    string   `json:"cover"` // the person's face, to compare
	Fit      float32  `json:"fit"`   // likeness to the rest of their faces, 0–1
	Score    float32  `json:"score"` // how surely it's them, as shown everywhere (Match)
	Auto     bool     `json:"auto"`  // filed by itself, not confirmed
	Born     float32  `json:"born,omitempty"`
	Looks    float32  `json:"looks"`
	Year     float32  `json:"year,omitempty"` // when the photo was taken
	AgeOff   bool     `json:"age_off,omitempty"`
	Gender   string   `json:"gender,omitempty"` // the person's, when the face reads as the other
	Other    int64    `json:"other,omitempty"`  // someone it's more like
	OtherFit float32  `json:"other_fit,omitempty"`
	Name2    string   `json:"other_name,omitempty"`
	Strength string   `json:"strength,omitempty"` // how it's theirs: "named" (confirmed with others), "filed" (by itself)
	CostWhy  string   `json:"cost_why,omitempty"` // what age and gender say against (Match)
	Weak     bool     `json:"weak,omitempty"`     // scores below what keeps a face filed by itself
	// Then is its score when the user confirmed it (Approval), when it has
	// dropped since; Ahead someone else who scores higher now.
	Then       float32 `json:"then,omitempty"`
	Ahead      int64   `json:"ahead,omitempty"`
	AheadName  string  `json:"ahead_name,omitempty"`
	AheadScore float32 `json:"ahead_score,omitempty"`
	AheadThen  float32 `json:"ahead_then,omitempty"` // their score when confirmed
	score      float32
}

// DoubtfulFaces lists faces filed under people that don't fit them, the
// clearest cases first. It holds the library a little at a time: it runs in
// the background, and answers saved meanwhile mustn't wait for it.
// Faces the user confirmed are doubted too, but only when they look more
// like someone else named or their age doesn't fit: a confirmation can be
// a slip (look-alike brothers), a face merely unlike the rest is no reason.
func (s *Store) DoubtfulFaces(ctx context.Context, limit int) []FaceDoubt {
	return s.doubtfulFaces(limit, doubtOpts{which: doubtBoth}, false)
}

// PersonDoubts is "Is this really …?" on a person's page: their faces
// that stand out, confirmed with others or filed (not those confirmed
// alone), each with why: scoring below what keeps a face filed by itself,
// the wrong age, more like someone else, or less them than when confirmed.
// Those confirmed with others come first: filed ones are rechecked anyway.
func (s *Store) PersonDoubts(ctx context.Context, pid int64, limit int) []FaceDoubt {
	return s.doubtfulFaces(limit, doubtOpts{which: doubtBoth, person: pid, outliers: true}, false)
}

// doubtWhich is which of a person's faces are judged.
type doubtWhich int

const (
	doubtFiled  doubtWhich = iota // filed by themselves
	doubtTagged                   // confirmed by the user (all right: to measure how often a right face is doubted)
	doubtBoth
)

type doubtOpts struct {
	which    doubtWhich
	person   int64 // just theirs, if not 0
	all      bool  // every face, the least likely first, not just those in doubt
	outliers bool  // also confirmed faces merely unlike the rest (a person's page)
}

// doubtfulFaces is DoubtfulFaces, as opts say. held: the caller holds the
// library already.
func (s *Store) doubtfulFaces(limit int, o doubtOpts, held bool) []FaceDoubt {
	read, run := s.readSteps(held)
	var born map[int64]float32
	var m *matcher
	var est map[int64]string
	var covers map[int64]string
	read(func() { born = s.bornFromFaces() })
	read(func() { m = s.newMatcher() })
	read(func() { est = s.genderEstimates(); covers = s.personCovers(s.facesByGroup()) })
	// Photos whose date is in doubt: there the date is wrong, not the faces.
	misdated := map[int64]bool{}
	read(func() {
		for _, d := range s.doubtfulDates(0) {
			misdated[d.Image] = true
		}
	})
	var fs []*faceRec
	read(func() {
		for _, f := range s.faces {
			c := s.clusters[f.ClusterID]
			if c == nil || c.PersonID == 0 || o.person != 0 && c.PersonID != o.person || f.NotFace || f.Picture || len(f.emb) == 0 || s.facePx(f) < fitMinPx {
				continue
			}
			switch confirmed := f.Tagged == c.PersonID; {
			case f.Tagged != 0 && !confirmed, o.which == doubtFiled && confirmed, o.which == doubtTagged && !confirmed,
				confirmed && f.Checked && o.which != doubtTagged:
				continue
			}
			if p := s.people[c.PersonID]; p == nil || p.Picture {
				continue
			}
			fs = append(fs, f)
		}
	})
	// Each face on its own, across all cores.
	res := make([]*FaceDoubt, len(fs))
	run(len(fs), func(i int) { res[i] = s.doubtFace(fs[i], m, born, est, covers, misdated, o) })
	var out []FaceDoubt
	for _, d := range res {
		if d != nil {
			out = append(out, *d)
		}
	}
	named := func(d FaceDoubt) int { return boolInt(o.outliers && d.Strength == "named") }
	slices.SortFunc(out, func(a, b FaceDoubt) int {
		return cmp.Or(cmp.Compare(named(b), named(a)), cmp.Compare(b.score, a.score), cmp.Compare(a.Score, b.Score), cmp.Compare(a.Face.ID, b.Face.ID))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// doubtFace is the doubt about one filed face, or nil when it fits.
func (s *Store) doubtFace(f *faceRec, m *matcher, born map[int64]float32, est map[int64]string, covers map[int64]string, misdated map[int64]bool, o doubtOpts) *FaceDoubt {
	c := s.clusters[f.ClusterID]
	if c == nil || c.PersonID == 0 || f.NotFace || f.Picture {
		return nil // answered meanwhile
	}
	p := s.people[c.PersonID]
	if p == nil {
		return nil
	}
	d := FaceDoubt{Person: p.ID, Name: p.Name, Auto: f.Tagged != p.ID, Looks: f.Age, Cover: covers[p.ID]}
	// Likeness to their faces, without itself (as everywhere: Match).
	likes := m.likes(f)
	fit, ok := likes[p.ID]
	if !ok || fit < -0.5 {
		return nil // nothing else of theirs to compare with
	}
	d.Fit = fit
	d.Score = m.match(f, p.ID, likes).Score
	_, d.CostWhy = m.cost(f, p.ID)
	_, st := s.faceStrength(f)
	d.Strength = st.String()
	why := float32(0)
	if d.Fit < fitAlike {
		why += (fitAlike - d.Fit) * 10
	}
	// Below what keeps a face filed by itself: if the user hadn't said so,
	// it wouldn't be theirs (a person's page).
	if o.outliers && d.Score < fileKeep {
		d.Weak = true
		why += (fileKeep - d.Score) * 2
	}
	// What's known since it was confirmed: less them, or someone else's.
	a := f.Approved
	if a != nil && (a.Person != p.ID || f.Tagged != p.ID) {
		a = nil
	}
	if a != nil && a.Score-d.Score >= fitDrop {
		d.Then = a.Score
		why += (a.Score - d.Score) * 5
	}
	if q, sc := m.nextBest(f, p.ID, likes); q != 0 && sc >= d.Score+fitAhead && sc >= fileKeep {
		d.Ahead, d.AheadName, d.AheadScore = q, s.people[q].Name, sc
		if a != nil && a.Next == q {
			d.AheadThen = a.Other
		}
		why += (sc - d.Score) * 5
	}
	if by, ok := s.impliedBirth(f); ok && !misdated[f.ImageID] && f.Age >= 0 {
		if b, ok := born[p.ID]; ok {
			off := abs32(by - b)
			sd := float32(dateKidAge + dateAgeErr*float64(f.Age))
			if off >= fitAgeOff && off >= fitAgeSure*sd {
				d.AgeOff, d.Born, d.Year = true, b, by+f.Age
				why += off / sd / fitAgeSure
			}
		}
	}
	// Someone it looks more like, not in the photo already, and whose
	// age fits it no worse (babies look alike; their birth years
	// tell brothers apart).
	by, byOK := s.impliedBirth(f)
	ageSays := func(q int64) float32 { // how far off q's birth year the face's is
		if b, ok := born[q]; ok && byOK {
			return abs32(by - b)
		}
		return 0
	}
	if d.Fit < fitLook || why > 0 {
		here := s.whoIn(f.ImageID, f.ID)
		for q, b := range likes {
			if q == p.ID || m.ruledOut(f, q, here) || ageSays(q) > ageSays(p.ID)+fitAgeGap {
				continue
			}
			if b > d.Fit+fitOther && (b > d.OtherFit || b == d.OtherFit && q < d.Other) {
				d.Other, d.OtherFit = q, b
			}
		}
	}
	if d.Other != 0 {
		why += d.OtherFit - d.Fit
	}
	switch {
	case o.all:
	case why == 0:
		return nil // the gender alone misreads too often (TestEvalDoubtfulFaces)
	case !d.Auto && d.Other == 0 && !d.AgeOff && d.Ahead == 0 && d.Then == 0 && !o.outliers:
		return nil // confirmed, and merely unlike the rest: no reason to ask
	}
	if g, _ := genderOf(p, est[p.ID]); genderMismatch(g, f) {
		d.Gender = g
		why++
	}
	if d.Other != 0 {
		d.Name2 = s.people[d.Other].Name
	}
	d.score = why
	d.Face = s.faceView(f)
	return &d
}
