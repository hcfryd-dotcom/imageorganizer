package store

import (
	"cmp"
	"context"
	"slices"
)

// ConfidentPick is a group in the review inbox whose best suggestion is
// sure enough to file without looking at it.
type ConfidentPick struct {
	Group  int64   `json:"group"`
	Person int64   `json:"person"`
	Name   string  `json:"name"`
	Score  float32 `json:"score"`
	Faces  int     `json:"faces"`
}

// ConfidentReview lists the inbox groups whose best suggestion is at least
// minScore alike, best first. A group is left out when its second
// suggestion is that alike too (two people it could be, siblings say), or
// the face reads as the other gender than the suggested person's. ambiguous
// counts the groups left out for two people that alike.
func (s *Store) ConfidentReview(ctx context.Context, minSize int, minScore float32) (picks []ConfidentPick, ambiguous int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	byGroup := s.facesByGroup()
	covers := s.personCovers(byGroup)
	idx := s.personIndex()
	fam := s.family()
	for _, c := range s.clusters {
		if !c.inReview(minSize) {
			continue
		}
		sg := s.suggest(c, byGroup[c.id], covers, idx, fam)
		if len(sg) == 0 {
			continue
		}
		best := slices.MaxFunc(sg, func(a, b Suggestion) int { return cmp.Compare(a.Score, b.Score) })
		if best.Score < minScore || best.Off != "" {
			continue
		}
		if slices.ContainsFunc(sg, func(o Suggestion) bool { return o.ID != best.ID && o.Score >= minScore }) {
			ambiguous++
			continue
		}
		picks = append(picks, ConfidentPick{Group: c.id, Person: best.ID, Name: best.Name, Score: best.Score, Faces: c.n})
	}
	slices.SortFunc(picks, func(a, b ConfidentPick) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Group, b.Group))
	})
	return picks, ambiguous
}
