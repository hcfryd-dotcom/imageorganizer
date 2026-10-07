package store

import (
	"cmp"
	"runtime"
	"slices"
	"sync"
	"time"
)

// groupingRules are the knobs of face grouping, so variants can be
// measured against the people the user has already named (see
// eval_test.go) before one replaces the rules in use.
type groupingRules struct {
	Name string
	// Join is the cosine similarity to a group's centroid needed to join.
	Join float32
	// Implied birth years may differ by BirthBase + BirthSlope*age.
	BirthBase, BirthSlope float32
	// A photo may be TimeBase + TimeSlope*age years from a group's
	// typical capture time.
	TimeBase, TimeSlope float32
	// OnePerPhoto keeps two faces of one photo out of the same group: a
	// person appears once in a photo.
	OnePerPhoto bool
	// EventGap > 0 links photos from the same camera taken within EventGap
	// of each other into an event; a face may join a group that already
	// has a face from its event at the lower EventJoin similarity.
	EventGap  time.Duration
	EventJoin float32
}

// currentRules are the rules in use; bestCluster implements the same for
// one face at a time.
var currentRules = groupingRules{
	Name: "current", Join: clusterJoin,
	BirthBase: 2, BirthSlope: 0.15, TimeBase: 1, TimeSlope: 0.15,
	OnePerPhoto: true,
}

// grouper groups faces under a set of rules into its own groups, leaving
// the store's groups alone.
type grouper struct {
	s      *Store
	r      groupingRules
	byID   map[int64]*cluster
	list   []*cluster
	evs    map[*cluster]map[int64]bool // events with a face in the group
	events map[int64]int64             // image id -> event id
	of     map[int64]*cluster          // face id -> group
}

func newGrouper(s *Store, r groupingRules) *grouper {
	g := &grouper{s: s, r: r, byID: map[int64]*cluster{}, evs: map[*cluster]map[int64]bool{}, of: map[int64]*cluster{}}
	if r.EventGap > 0 {
		g.events = s.photoEvents(r.EventGap)
	}
	return g
}

// photoEvents links photos taken with the same camera within gap of the
// previous one into events, keyed by the first photo's id. Photos without
// a camera or date are their own event.
func (s *Store) photoEvents(gap time.Duration) map[int64]int64 {
	var dated []*imageRec
	ev := map[int64]int64{}
	for id, im := range s.images {
		ev[id] = id
		if im.TakenAt != nil && im.CameraModel != "" {
			dated = append(dated, im)
		}
	}
	slices.SortFunc(dated, func(a, b *imageRec) int {
		return cmp.Or(cmp.Compare(a.CameraMake+"\x00"+a.CameraModel, b.CameraMake+"\x00"+b.CameraModel),
			cmp.Compare(*a.TakenAt, *b.TakenAt), cmp.Compare(a.ID, b.ID))
	})
	for i := 1; i < len(dated); i++ {
		a, b := dated[i-1], dated[i]
		if a.CameraMake == b.CameraMake && a.CameraModel == b.CameraModel && time.Duration(*b.TakenAt-*a.TakenAt)*time.Second <= gap {
			ev[b.ID] = ev[a.ID]
		}
	}
	return ev
}

// run groups the faces in capture order, as regroup does.
func (g *grouper) run(faces []*faceRec) {
	s := g.s
	slices.SortFunc(faces, func(a, b *faceRec) int {
		ta, oka := s.captureYear(a)
		tb, okb := s.captureYear(b)
		if oka != okb {
			return cmp.Compare(boolInt(!oka), boolInt(!okb))
		}
		return cmp.Or(cmp.Compare(ta, tb), cmp.Compare(a.ID, b.ID))
	})
	for _, f := range faces {
		c := g.best(f)
		if c == nil {
			c = &cluster{id: int64(len(g.list) + 1)}
			g.byID[c.id] = c
			g.list = append(g.list, c)
			g.evs[c] = map[int64]bool{}
		}
		s.clusterAdd(c, f)
		if g.events != nil {
			g.evs[c][g.events[f.ImageID]] = true
		}
		g.of[f.ID] = c
	}
}

// best finds the group f belongs in under the rules, or nil for a new one.
// Groups are scanned in parallel; ties go to the oldest group.
func (g *grouper) best(f *faceRec) *cluster {
	s, r := g.s, g.r
	by, aged := s.impliedBirth(f)
	ft, timed := s.captureYear(f)
	ev, hasEv := g.events[f.ImageID]
	fn := float32(norm(f.emb))

	workers := runtime.GOMAXPROCS(0)
	chunk := (len(g.list) + workers - 1) / workers
	type hit struct {
		c   *cluster
		sim float32
	}
	hits := make([]hit, workers)
	var wg sync.WaitGroup
	for w := range workers {
		lo, hi := w*chunk, min((w+1)*chunk, len(g.list))
		if lo >= hi {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			best := hit{}
			for _, c := range g.list[lo:hi] {
				if c.n == 0 {
					continue
				}
				if r.OnePerPhoto && c.imgs[f.ImageID] > 0 {
					continue
				}
				cb, ca, cok := c.birth()
				if aged && cok && abs32(by-cb) > r.BirthBase+r.BirthSlope*max(f.Age, ca) {
					continue
				}
				if ct, ok := c.when(); ok && timed {
					age := f.Age
					if !aged {
						age = ca
						if !cok {
							age = 0
						}
					}
					if abs32(ft-ct) > r.TimeBase+r.TimeSlope*age {
						continue
					}
				}
				need := r.Join
				if hasEv && g.evs[c][ev] {
					need = r.EventJoin
				}
				sim := c.simNorm(f.emb, fn)
				if sim >= need && (best.c == nil || sim > best.sim) {
					best = hit{c, sim}
				}
			}
			hits[w] = best
		}()
	}
	wg.Wait()
	var out hit
	for _, h := range hits {
		if h.c != nil && (out.c == nil || h.sim > out.sim || (h.sim == out.sim && h.c.id < out.c.id)) {
			out = h
		}
	}
	return out.c
}

// simNorm is sim with the vector's norm already known.
func (c *cluster) simNorm(v []float32, vn float32) float32 {
	n := norm(c.sum) * float64(vn)
	if n == 0 {
		return 0
	}
	var d float64
	for i, x := range c.sum {
		d += float64(x) * float64(v[i])
	}
	return float32(d / n)
}
