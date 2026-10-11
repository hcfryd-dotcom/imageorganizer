package store

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"
)

// TestEvalChain looks at the "Who is this?" chain most like a person
// (IO_EVAL_PERSON): how alike its faces are to each other (each to its
// group without itself), each group to the person, and each face.
func TestEvalChain(t *testing.T) {
	path := os.Getenv("IO_EVAL_DB")
	pid, _ := strconv.ParseInt(os.Getenv("IO_EVAL_PERSON"), 10, 64)
	if path == "" || pid == 0 {
		t.Skip("set IO_EVAL_DB and IO_EVAL_PERSON")
	}
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	chs := s.DiscoverPeople(context.Background(), 30)
	s.mu.RLock()
	defer s.mu.RUnlock()
	m := s.newMatcher()
	q := func(xs []float32) string {
		if len(xs) == 0 {
			return "-"
		}
		slices.Sort(xs)
		return strconv.FormatFloat(float64(xs[len(xs)/10]), 'f', 2, 32) + " / " +
			strconv.FormatFloat(float64(xs[len(xs)/2]), 'f', 2, 32) + " / " +
			strconv.FormatFloat(float64(xs[len(xs)*9/10]), 'f', 2, 32)
	}
	// The chain most like the person.
	slices.SortStableFunc(chs, func(a, b Chain) int {
		return boolInt(b.Maybe == pid) - boolInt(a.Maybe == pid)
	})
	for _, ch := range chs[:1] {
		t.Logf("chain: %d photos, %d groups (p10 / median / p90 below)", ch.Photos, len(ch.Groups))
		var allSelf, allLike []float32
		for _, gid := range ch.Groups {
			c := s.clusters[gid]
			var self, like []float32
			for _, f := range s.groupFaces(gid) {
				if len(f.emb) == 0 {
					continue
				}
				if c.n > 1 {
					self = append(self, looLike(f, c))
				}
				like = append(like, m.likes(f)[pid])
			}
			gl, _ := bestRef(clusterUnit(c), m.mine[pid])
			t.Logf("  group %d: %d faces, group like the person %.2f; faces like the rest of the group %s; faces like the person %s",
				gid, c.n, gl, q(slices.Clone(self)), q(slices.Clone(like)))
			allSelf, allLike = append(allSelf, self...), append(allLike, like...)
		}
		t.Logf("  all: faces like their group %s; faces like the person %s", q(allSelf), q(allLike))
	}
	// For comparison: the person's own confirmed faces, like the rest.
	var own []float32
	for _, f := range s.faces {
		if f.Tagged == pid && len(f.emb) > 0 {
			own = append(own, m.likes(f)[pid])
		}
	}
	t.Logf("the person's confirmed faces, like the rest of theirs: %s (%d)", q(own), len(own))
}
