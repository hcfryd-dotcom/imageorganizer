package store

import (
	"context"
	"time"
)

// Approval is how a confirmed face matched its person when the user
// confirmed it: its score, and the likeliest someone else's.
type Approval struct {
	Person int64   `json:"p"`
	Score  float32 `json:"s"`           // Match.Score for Person
	Next   int64   `json:"n,omitempty"` // the likeliest someone else
	Other  float32 `json:"o,omitempty"` // their score
	At     int64   `json:"t"`           // when recorded
}

// approvalChunk is how many approvals are stored at a time.
const approvalChunk = 2000

// RecordApprovals records how each face confirmed since the last time
// matched then (Approval), with the library readable meanwhile. Faces
// confirmed before approvals were kept get today's match, as a baseline.
func (s *Store) RecordApprovals(ctx context.Context) (int, error) {
	read, run := s.readSteps(false)
	var m *matcher
	var fs []*faceRec
	read(func() {
		for _, f := range s.faces {
			if f.Tagged != 0 && !f.NotFace && len(f.emb) > 0 && (f.Approved == nil || f.Approved.Person != f.Tagged) {
				fs = append(fs, f)
			}
		}
		if len(fs) > 0 {
			m = s.newMatcher()
		}
	})
	if len(fs) == 0 {
		return 0, nil
	}
	now := time.Now().Unix()
	out := make([]*Approval, len(fs))
	run(len(fs), func(i int) {
		f := fs[i]
		pid := f.Tagged
		if pid == 0 || s.people[pid] == nil {
			return // answered meanwhile
		}
		likes := m.likes(f)
		a := &Approval{Person: pid, Score: m.match(f, pid, likes).Score, At: now}
		a.Next, a.Other = m.nextBest(f, pid, likes)
		out[i] = a
	})
	n := 0
	for start := 0; start < len(fs); start += approvalChunk {
		if err := ctx.Err(); err != nil {
			return n, err
		}
		s.mu.Lock()
		ch := s.newChanges()
		for i := start; i < min(len(fs), start+approvalChunk); i++ {
			if a, f := out[i], fs[i]; a != nil && f.Tagged == a.Person {
				f.Approved = a
				ch.face(f)
				n++
			}
		}
		err := ch.save()
		s.mu.Unlock()
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

// nextBest is the likeliest person for face f other than pid, and their
// score (as Match scores it), 0 if none.
func (m *matcher) nextBest(f *faceRec, pid int64, likes map[int64]float32) (int64, float32) {
	here := m.s.whoIn(f.ImageID, f.ID)
	var who int64
	top := float32(0)
	for q, like := range likes {
		if q == pid || m.ruledOut(f, q, here) {
			continue
		}
		if sc := min(1, m.score(f, q, like, here)); sc > top || sc == top && q < who {
			who, top = q, sc
		}
	}
	return who, top
}
