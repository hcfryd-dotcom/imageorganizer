package store

import (
	"cmp"
	"math"
	"slices"
	"time"
)

// Company is who an unnamed group is photographed with: people you've
// named, on how many days, over which years, in how many places. Someone
// seen with your family on many days, over years and in many places is
// most likely someone you know; a stranger at one event is seen once.
type Company struct {
	Days   int      `json:"days"`   // distinct days in a photo with someone named
	From   int      `json:"from"`   // first and last year of those
	To     int      `json:"to"`     //
	Places int      `json:"places"` // distinct places (of photos with a location)
	With   []string `json:"with"`   // the named people most often with them
	Score  float32  `json:"score"`
}

type companyAcc struct {
	days   map[int64]bool
	years  map[int]bool
	places map[[2]int32]bool
	with   map[int64]map[int64]bool // person -> days
}

// company gathers, in one pass over the photos, each unnamed (or only
// automatically filed) group's company. Passers-by don't count, nor do
// hidden strangers. Call with s.mu held.
func (s *Store) company() map[int64]*Company {
	acc := map[int64]*companyAcc{}
	for img, fids := range s.facesByImage {
		im := s.images[img]
		if im == nil || im.TakenAt == nil {
			continue
		}
		var named []int64
		var unnamed []int64
		for _, fid := range fids {
			f := s.faces[fid]
			if f == nil || f.NotFace || f.Picture {
				continue
			}
			c := s.clusters[f.ClusterID]
			switch {
			case f.Tagged != 0:
				named = append(named, f.Tagged)
			case c == nil || c.Hidden:
			case c.PersonID != 0 && !c.Auto:
				named = append(named, c.PersonID)
			case !f.bg:
				unnamed = append(unnamed, c.id)
			}
		}
		if len(named) == 0 || len(unnamed) == 0 {
			continue
		}
		day := *im.TakenAt / 86400
		year := time.Unix(*im.TakenAt, 0).UTC().Year()
		for _, gid := range unnamed {
			a := acc[gid]
			if a == nil {
				a = &companyAcc{days: map[int64]bool{}, years: map[int]bool{}, places: map[[2]int32]bool{}, with: map[int64]map[int64]bool{}}
				acc[gid] = a
			}
			a.days[day] = true
			a.years[year] = true
			if im.Lat != nil && im.Lon != nil {
				// About 10 km: the same town is one place.
				a.places[[2]int32{int32(math.Round(*im.Lat * 10)), int32(math.Round(*im.Lon * 10))}] = true
			}
			for _, pid := range named {
				if c := s.clusters[gid]; c != nil && c.PersonID == pid {
					continue // filed as them: not company
				}
				if a.with[pid] == nil {
					a.with[pid] = map[int64]bool{}
				}
				a.with[pid][day] = true
			}
		}
	}
	out := make(map[int64]*Company, len(acc))
	for gid, a := range acc {
		c := &Company{Days: len(a.days), Places: len(a.places)}
		for y := range a.years {
			if c.From == 0 || y < c.From {
				c.From = y
			}
			c.To = max(c.To, y)
		}
		c.Score = float32(c.Days + 2*(len(a.years)-1) + max(c.Places-1, 0))
		type pw struct {
			pid  int64
			days int
		}
		var ws []pw
		for pid, d := range a.with {
			ws = append(ws, pw{pid, len(d)})
		}
		slices.SortFunc(ws, func(x, y pw) int { return cmp.Or(cmp.Compare(y.days, x.days), cmp.Compare(x.pid, y.pid)) })
		for _, w := range ws[:min(len(ws), 3)] {
			if p := s.people[w.pid]; p != nil {
				c.With = append(c.With, p.Name)
			}
		}
		out[gid] = c
	}
	return out
}
