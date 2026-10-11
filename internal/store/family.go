package store

import (
	"cmp"
	"slices"
)

const (
	familyMin   = 0.25 // share of a person's days a companion must be on, to count as their company
	familyLift  = 3    // and how many times more often than on their other days: your own household is everywhere
	familyRare  = 0.15 // and on at most this share of those other days
	familyAge   = 12   // the family path is for faces this young or younger: older ones match on likeness
	familyDay   = 0.30 // a day's company score for its faces to be looked at
	familySim   = 0.15 // likeness enough for a face on such a day (a small child's face matches little)
	familyBonus = 0.25 // ranking bonus at full company
)

// takenDay is the day a photo was taken (days since 1970), -1 undated.
func takenDay(im *imageRec) int64 {
	if im == nil || im.TakenAt == nil {
		return -1
	}
	return *im.TakenAt / 86400
}

// dayCompany finds, for person pid, the days their own company was
// photographed (people on at least familyMin of their days, and familyLift
// times more often than on the other days of those years: for a child,
// say, their parents, but not the household whose photos these are): how strongly
// (the largest such share among those there) and who. A small child
// matches their older self poorly, but is photographed on the days their
// family is. Call with s.mu held.
func (s *Store) dayCompany(pid int64) (score map[int64]float32, who map[int64][]string) {
	// Who is photographed on which days, by confirmed or tagged faces.
	days := map[int64]map[int64]bool{} // person -> days
	for _, f := range s.faces {
		if f.NotFace || f.Picture {
			continue
		}
		p := f.Tagged
		if c := s.clusters[f.ClusterID]; p == 0 && c != nil && c.PersonID != 0 && !c.Auto {
			p = c.PersonID
		}
		im := s.images[f.ImageID]
		if p == 0 || im == nil || im.TakenAt == nil {
			continue
		}
		if days[p] == nil {
			days[p] = map[int64]bool{}
		}
		days[p][*im.TakenAt/86400] = true
	}
	mine := days[pid]
	if len(mine) == 0 {
		return nil, nil
	}
	// "In general" means over the years of their own photos: before
	// someone was born they're on no days at all.
	lo, hi := int64(1<<62), int64(-1<<62)
	for d := range mine {
		lo, hi = min(lo, d), max(hi, d)
	}
	other := map[int64]bool{} // the other days photographed, in that span
	for _, im := range s.images {
		if d := takenDay(im); d >= lo && d <= hi && !mine[d] {
			other[d] = true
		}
	}
	// Each companion's share of the person's days.
	share := map[int64]float32{}
	for q, ds := range days {
		if q == pid {
			continue
		}
		n := 0
		for d := range mine {
			if ds[d] {
				n++
			}
		}
		elsewhere := 0
		for d := range ds {
			if other[d] {
				elsewhere++
			}
		}
		sh := float32(n) / float32(len(mine))
		base := float32(0)
		if len(other) > 0 {
			base = float32(elsewhere) / float32(len(other))
		}
		if sh >= familyMin && n >= 2 && sh >= familyLift*base && base <= familyRare {
			share[q] = sh
		}
	}
	// Family the user entered counts, whatever the numbers say.
	for q := range s.relatives(pid) {
		if len(days[q]) > 0 {
			share[q] = max(share[q], 0.5)
		}
	}
	score, who = map[int64]float32{}, map[int64][]string{}
	for q, sh := range share {
		name := ""
		if p := s.people[q]; p != nil {
			name = p.Name
		}
		for d := range days[q] {
			score[d] = max(score[d], sh)
			who[d] = append(who[d], name)
		}
	}
	for d := range who {
		slices.SortFunc(who[d], func(a, b string) int { return cmp.Compare(a, b) })
	}
	return score, who
}
