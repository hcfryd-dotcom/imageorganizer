package store

import (
	"os"
	"testing"
)

// TestEvalAgeFits compares today's age check (one birth year, the entered
// one if any) with the per-person age model: each confirmed group is
// checked against its own person (learnt without the group) and against
// everyone else.
func TestEvalAgeFits(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	if path == "" {
		t.Skip("set IO_EVAL_DB to a database copy")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	idx := s.confirmedIndex()
	groupsOf := func(pi *personInfo, skip int64) map[int64]bool {
		m := map[int64]bool{}
		for _, g := range pi.groups {
			if g.id != skip {
				m[g.id] = true
			}
		}
		return m
	}
	type tally struct{ ownG, ownBadOld, ownBadNew, ownF, ownBadOldF, ownBadNewF, other, otherOutOld, otherOutNew int }
	var kid, adult tally
	for pid, pi := range idx {
		for _, c := range pi.groups {
			cb, ca, cok := c.birth()
			if !cok || c.birthN < 2 {
				continue
			}
			tl := &adult
			if ca < 16 {
				tl = &kid
			}
			// Old rule: personBirth as it was, without this group.
			var bs, as float64
			var bn int
			for _, g := range pi.groups {
				if g.id != c.id {
					bs += g.birthSum
					as += g.ageSum
					bn += g.birthN
				}
			}
			ob, oa, ook := float32(0), float32(0), false
			if pi.p.BirthYear != 0 {
				ob, ook = float32(pi.p.BirthYear)+0.5, true
			} else if bn > 0 {
				ob, oa, ook = float32(bs/float64(bn)), float32(as/float64(bn)), true
			}
			m := s.buildAgeModel(pi.p, groupsOf(pi, c.id))
			tl.ownG++
			tl.ownF += c.birthN
			if !birthCompatible(cb, ca, true, ob, oa, ook) {
				tl.ownBadOld++
				tl.ownBadOldF += c.birthN
			}
			if !m.fits(cb, ca, true) {
				tl.ownBadNew++
				tl.ownBadNewF += c.birthN
			}
			for qid, qi := range idx {
				if qid == pid || len(qi.groups) == 0 {
					continue
				}
				tl.other++
				if qb, qa, qok := qi.oldBirth(); !birthCompatible(cb, ca, true, qb, qa, qok) {
					tl.otherOutOld++
				}
				if !qi.ages.fits(cb, ca, true) {
					tl.otherOutNew++
				}
			}
		}
	}
	for _, x := range []struct {
		n string
		t tally
	}{{"children", kid}, {"adults", adult}} {
		t.Logf("%-8s own groups %d (%d faces): turned away old %d (%d faces) new %d (%d faces); other people ruled out old %.1f%% new %.1f%%",
			x.n, x.t.ownG, x.t.ownF, x.t.ownBadOld, x.t.ownBadOldF, x.t.ownBadNew, x.t.ownBadNewF,
			100*float64(x.t.otherOutOld)/float64(x.t.other), 100*float64(x.t.otherOutNew)/float64(x.t.other))
	}
}

func (pi *personInfo) oldBirth() (float32, float32, bool) {
	if pi.p.BirthYear != 0 {
		return float32(pi.p.BirthYear) + 0.5, 0, true
	}
	var bs, as float64
	var bn int
	for _, g := range pi.groups {
		bs += g.birthSum
		as += g.ageSum
		bn += g.birthN
	}
	if bn == 0 {
		return 0, 0, false
	}
	return float32(bs / float64(bn)), float32(as / float64(bn)), true
}
