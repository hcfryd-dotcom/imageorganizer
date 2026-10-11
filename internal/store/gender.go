package store

const (
	genderAdult  = 16   // faces looking this old or older tell gender well; children's don't
	genderSure   = 0.9  // a face reads as one gender this surely
	genderMinN   = 5    // confirmed adult faces needed to estimate a person's gender
	genderPerson = 0.75 // their mean reading, to call it
	genderMiss   = 0.10 // taken off a suggestion whose face clearly reads as the other gender
)

// Genders as stored and shown.
const (
	Female = "female"
	Male   = "male"
)

// genderAcc sums a person's adult faces' readings.
type genderAcc struct {
	sum float32
	n   int
}

func (a *genderAcc) add(f *faceRec) {
	if f.FemV > 0 && f.Fem >= 0 && f.Age >= genderAdult && !f.NotFace && !f.Picture {
		a.sum += f.Fem
		a.n++
	}
}

// guess is the gender the readings point to, "" if too few or mixed.
func (a genderAcc) guess() string {
	if a.n < genderMinN {
		return ""
	}
	switch m := a.sum / float32(a.n); {
	case m >= genderPerson:
		return Female
	case m <= 1-genderPerson:
		return Male
	}
	return ""
}

// genderOf is a person's gender: as entered, else as estimated ("" when
// unknown), and whether it was estimated.
func genderOf(p *personRec, est string) (string, bool) {
	if p.Gender != "" {
		return p.Gender, false
	}
	return est, est != ""
}

// genderEstimates estimates every person's gender from the faces in their
// confirmed groups, in one pass; call with s.mu held.
func (s *Store) genderEstimates() map[int64]string {
	acc := map[int64]*genderAcc{}
	for _, f := range s.faces {
		c := s.clusters[f.ClusterID]
		if c == nil || c.PersonID == 0 || c.Auto {
			continue
		}
		a := acc[c.PersonID]
		if a == nil {
			a = &genderAcc{}
			acc[c.PersonID] = a
		}
		a.add(f)
	}
	out := map[int64]string{}
	for pid, a := range acc {
		if g := a.guess(); g != "" {
			out[pid] = g
		}
	}
	return out
}

// genderMismatch reports whether a face, or the faces of a group, clearly
// read as the other gender than the person's (gender: "female", "male" or
// "" unknown). Only adult faces count.
func genderMismatch(gender string, faces ...*faceRec) bool {
	if gender == "" {
		return false
	}
	var a genderAcc
	for _, f := range faces {
		a.add(f)
	}
	if a.n == 0 {
		return false
	}
	m := a.sum / float32(a.n)
	return gender == Male && m >= genderSure || gender == Female && m <= 1-genderSure
}

// kinWord names a relation with the relative's gender, when known:
// "mother" rather than "parent".
func kinWord(kind, gender string) string {
	words := map[string][2]string{
		KinParent:       {"mother", "father"},
		KinChild:        {"daughter", "son"},
		KinSibling:      {"sister", "brother"},
		KinGrandparent:  {"grandmother", "grandfather"},
		KinGrandchild:   {"granddaughter", "grandson"},
		KinHalfSibling:  {"half-sister", "half-brother"},
		KinStepParent:   {"stepmother", "stepfather"},
		KinStepChild:    {"stepdaughter", "stepson"},
		KinStepSibling:  {"stepsister", "stepbrother"},
		KinAuntUncle:    {"aunt", "uncle"},
		KinNieceNephew:  {"niece", "nephew"},
		KinParentInLaw:  {"mother-in-law", "father-in-law"},
		KinChildInLaw:   {"daughter-in-law", "son-in-law"},
		KinSiblingInLaw: {"sister-in-law", "brother-in-law"},
	}
	neutral := map[string]string{KinAuntUncle: "aunt or uncle", KinNieceNephew: "niece or nephew"}
	w, ok := words[kind]
	switch {
	case !ok:
		return kind
	case gender == Female:
		return w[0]
	case gender == Male:
		return w[1]
	case neutral[kind] != "":
		return neutral[kind]
	}
	return kind
}
