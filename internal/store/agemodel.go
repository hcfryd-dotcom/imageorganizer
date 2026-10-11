package store

import (
	"math"
	"slices"
	"strings"
)

// scanned reports whether a photo came off a scanner: its date is when it
// was scanned, not when it was taken, so it says nothing about age.
func scanned(im *imageRec) bool {
	m := strings.ToLower(im.CameraMake + " " + im.CameraModel)
	return strings.Contains(m, "scan") || strings.Contains(m, "perfection")
}

// Age model step and window: how a person looks is read every quarter
// year from their confirmed faces within two years of it.
const (
	ageStep   = 0.25
	ageWindow = 2
	ageMinN   = 10
)

// ageModel is how old a person looks over time, learnt from their faces
// ("at 50 he looks 36"): some people look younger than they are, and
// apparent ages are read differently for children, adults and the old, so
// a group is compared with how the person looked around its own dates
// rather than with one birth year.
type ageModel struct {
	t0    float32   // capture year of looks[0]
	looks []float32 // apparent age every ageStep years; NaN where too few faces
	birth float32   // fallback: mean implied birth year
	age   float32   // and mean apparent age, for its tolerance
	ok    bool
}

type agePoint struct{ t, a float32 } // capture year, apparent age

// buildAgeModel learns a person's model from the faces in their groups.
func (s *Store) buildAgeModel(p *personRec, groups map[int64]bool) ageModel {
	var ps []agePoint
	for _, f := range s.faces {
		if groups[f.ClusterID] {
			ps = s.addAgePoint(ps, f)
		}
	}
	return ageModelFrom(p, ps)
}

// ageModels sets every indexed person's model in one pass over the faces.
func (s *Store) ageModels(idx personIndex) {
	owner := map[int64]*personInfo{}
	for _, pi := range idx {
		for _, g := range pi.groups {
			owner[g.id] = pi
		}
	}
	// From the faces the user confirmed, once there are enough of them:
	// faces filed by themselves must not move how old someone looks when,
	// or a look-alike sibling's faces, once a few slip in, shift it their
	// way and let in more (TestFiledDontMoveAge).
	pts, conf := map[*personInfo][]agePoint{}, map[*personInfo][]agePoint{}
	gs, gc := map[*personInfo]*genderAcc{}, map[*personInfo]*genderAcc{} // their gender, in the same pass
	for _, f := range s.faces {
		pi := owner[f.ClusterID]
		if pi == nil {
			continue
		}
		pts[pi] = s.addAgePoint(pts[pi], f)
		if gs[pi] == nil {
			gs[pi], gc[pi] = &genderAcc{}, &genderAcc{}
		}
		gs[pi].add(f)
		if f.Tagged == pi.p.ID {
			conf[pi] = s.addAgePoint(conf[pi], f)
			gc[pi].add(f)
		}
	}
	for _, pi := range idx {
		ps, g := pts[pi], gs[pi]
		if len(conf[pi]) >= ageConfirmed {
			ps, g = conf[pi], gc[pi]
		}
		pi.ages = ageModelFrom(pi.p, ps)
		est := ""
		if g != nil {
			est = g.guess()
		}
		pi.gender, _ = genderOf(pi.p, est)
	}
}

// ageConfirmed is how many dated faces the user confirmed a person needs
// for their age and gender to be read from those alone.
const ageConfirmed = 8

func (s *Store) addAgePoint(ps []agePoint, f *faceRec) []agePoint {
	if f.NotFace {
		return ps
	}
	if by, ok := s.impliedBirth(f); ok {
		ps = append(ps, agePoint{by + f.Age, f.Age})
	}
	return ps
}

// ageModelFrom fits the model to a person's points (sorted in place).
// With none, an entered birth year stands in.
func ageModelFrom(p *personRec, ps []agePoint) ageModel {
	if len(ps) == 0 {
		if p.BirthYear != 0 {
			return ageModel{birth: float32(p.BirthYear) + 0.5, ok: true}
		}
		return ageModel{}
	}
	var bs, as float64
	for _, x := range ps {
		bs += float64(x.t - x.a)
		as += float64(x.a)
	}
	m := ageModel{birth: float32(bs / float64(len(ps))), age: float32(as / float64(len(ps))), ok: true}
	if p.BirthYear != 0 {
		m.birth = float32(p.BirthYear) + 0.5 // known: the faces only say how old they look
	}
	slices.SortFunc(ps, func(a, b agePoint) int { return cmpF32(a.t, b.t) })
	m.t0 = float32(math.Floor(float64(ps[0].t)))
	steps := int((ps[len(ps)-1].t-m.t0)/ageStep) + 1
	m.looks = make([]float32, steps)
	var v []float32
	lo := 0
	for i := range m.looks {
		t := m.t0 + float32(i)*ageStep
		for lo < len(ps) && ps[lo].t < t-ageWindow {
			lo++
		}
		v = v[:0]
		for _, x := range ps[lo:] {
			if x.t > t+ageWindow {
				break
			}
			v = append(v, x.a+(t-x.t)) // aged to t
		}
		if len(v) < ageMinN {
			m.looks[i] = float32(math.NaN())
			continue
		}
		slices.Sort(v)
		m.looks[i] = v[len(v)/2]
	}
	return m
}

// looksAt is the apparent age the person would have at capture year t:
// read off the nearest quarter with enough faces, aged by the gap.
func (m *ageModel) looksAt(t float32) (float32, bool) {
	i := int(math.Round(float64((t - m.t0) / ageStep)))
	for d := 0; d < len(m.looks)+abs(i); d++ {
		for _, j := range []int{i - d, i + d} {
			if j >= 0 && j < len(m.looks) && !math.IsNaN(float64(m.looks[j])) {
				return m.looks[j] + t - (m.t0 + float32(j)*ageStep), true
			}
		}
	}
	return 0, false
}

const (
	ageAlike = 0.55 // likeness at which an adult's age is judged loosely
	ageAdult = 30   // from this apparent age (both the face's and the person's)
)

// fitsLike is fits for a group or face sim alike to the person: an adult
// who is very alike gets twice the age tolerance. Age estimates of adults
// are rough (sun, glasses, a small face read 17 years young), while a
// likeness this strong rarely happens between different people; children
// keep the strict check, which keeps look-alike siblings apart.
func (m *ageModel) fitsLike(cb, ca float32, cok bool, sim float32) bool {
	if m.fits(cb, ca, cok) {
		return true
	}
	if sim < ageAlike || ca < ageAdult {
		return false
	}
	if want, ok := m.looksAt(cb + ca); ok {
		return want >= ageAdult && abs32(ca-want) <= 2*birthTolerance(max(ca, want))
	}
	return m.age >= ageAdult && abs32(cb-m.birth) <= 2*birthTolerance(max(ca, m.age))
}

// fits reports whether a group with mean implied birth year cb and mean
// apparent age ca could be the person; unknown ages always fit.
func (m *ageModel) fits(cb, ca float32, cok bool) bool {
	if !cok || !m.ok {
		return true
	}
	if want, ok := m.looksAt(cb + ca); ok {
		return abs32(ca-want) <= birthTolerance(max(ca, want))
	}
	return birthCompatible(cb, ca, true, m.birth, m.age, true)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func cmpF32(a, b float32) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// looksNow is the person's real and apparent age at the latest time with
// enough confirmed faces to tell; zero if there is none.
func (s *Store) looksNow(p *personRec) (real, looks float32) {
	groups := map[int64]bool{}
	for _, c := range s.clusters {
		if c.PersonID == p.ID && !c.Auto {
			groups[c.id] = true
		}
	}
	m := s.buildAgeModel(p, groups)
	for i := len(m.looks) - 1; i >= 0; i-- {
		if !math.IsNaN(float64(m.looks[i])) {
			return m.t0 + float32(i)*ageStep - float32(p.BirthYear) - 0.5, m.looks[i]
		}
	}
	return 0, 0
}
