package store

import (
	"cmp"
	"context"
	"slices"
)

// Faces filed under someone by themselves: near certain, by the faces the
// user confirmed and those filed before (the more faces of someone, the
// better they're known, so a person's set grows on itself). Strict, since
// a mistake would teach more mistakes; and every pass rechecks what was
// filed, each face against the others without itself, so one that no
// longer stands out as them goes back. A face the user said isn't them
// never is (Not), and "Is this really them?" asks about those that fit
// worst.
const (
	fileLead     = 0.15 // over anyone else's, and over anyone seen with them not named yet
	fileKeepLead = 0.10 // and this far ahead of anyone else
	filePx       = 40   // face width in pixels, at least
)

// The score (Match: likeness, relatives, the occasion, less age and gender
// against) a face is filed at, and kept at (rechecked without itself).
// Variables, for TestEvalStartOver to try others: from no names, a few
// people named a round, 0.55 filed a tenth more than 0.60 with none wrong;
// 0.50 began filing strangers (faces never named, ten times as many).
var (
	fileSim  float32 = 0.55
	fileKeep float32 = 0.50
)

// fileChunk is how many faces are filed at a time: the library stays
// readable in between, and the counts grow as they're filed.
const fileChunk = 300

// FileFaces files the faces now near certain someone's, and unfiles the
// filed faces that no longer are. One pass: call again while it files any
// (each pass learns from the last).
func (s *Store) FileFaces(ctx context.Context) (filed, unfiled int, err error) {
	return s.FileFacesProgress(ctx, nil)
}

// FileFacesProgress is FileFaces, saying how far it got after each chunk
// it files. Faces are scored with the library readable, then filed a
// chunk at a time, the surest first; a face decided otherwise meanwhile is
// left as it is.
func (s *Store) FileFacesProgress(ctx context.Context, progress func(filed, unfiled int)) (filed, unfiled int, err error) {
	type verdict struct {
		f     *faceRec
		pid   int64 // to file under
		from  int64 // filed under, to leave
		sim   float32
		leave bool // a filed face no longer near certain
	}
	// The library is held readable a little at a time: answers saved
	// meanwhile don't wait for the scoring, and are checked for below.
	read, run := s.readSteps(false)
	var m *matcher
	var fs []*faceRec
	read(func() {
		m = s.newMatcher()
		for _, f := range s.faces {
			c := s.clusters[f.ClusterID]
			if c == nil || f.Tagged != 0 || f.NotFace || f.Picture || len(f.emb) == 0 {
				continue
			}
			if c.PersonID == 0 && c.Hidden {
				continue // strangers
			}
			fs = append(fs, f)
		}
	})
	if len(m.refs) == 0 {
		return 0, 0, nil
	}
	out := make([]verdict, len(fs))
	run(len(fs), func(i int) {
		f := fs[i]
		c := s.clusters[f.ClusterID]
		if c == nil || f.Tagged != 0 || f.NotFace {
			return // answered meanwhile
		}
		likes := m.likes(f)
		if c.PersonID != 0 {
			// Filed already: does it still stand out as them?
			if !m.match(f, c.PersonID, likes).Sure {
				out[i] = verdict{f: f, from: c.PersonID, leave: true, sim: 2}
			}
			return
		}
		if who := m.best(f, likes); who != 0 {
			if mt := m.match(f, who, likes); mt.Sure {
				out[i] = verdict{f: f, pid: who, sim: mt.Like + mt.Kin + mt.Event + mt.Shot - mt.Cost}
			}
		}
	})
	out = slices.DeleteFunc(out, func(v verdict) bool { return v.f == nil })
	// Faces leaving first, then the surest: two faces of one photo can't
	// both be someone, and the likelier wins.
	slices.SortFunc(out, func(a, b verdict) int {
		return cmp.Or(cmp.Compare(b.sim, a.sim), cmp.Compare(a.f.ID, b.f.ID))
	})
	taken := map[[2]int64]bool{}
	for start := 0; start < len(out); start += fileChunk {
		if err := ctx.Err(); err != nil {
			return filed, unfiled, err
		}
		s.mu.Lock()
		ch := s.newChanges()
		for _, v := range out[start:min(len(out), start+fileChunk)] {
			f := v.f
			c := s.clusters[f.ClusterID]
			if c == nil || f.Tagged != 0 || f.NotFace {
				continue // decided otherwise meanwhile
			}
			if v.leave {
				if c.PersonID == v.from {
					ch.group(s.unfile(f))
					ch.face(f)
					unfiled++
				}
				continue
			}
			key := [2]int64{v.pid, f.ImageID}
			if c.PersonID != 0 || taken[key] || slices.Contains(f.Not, v.pid) || s.people[v.pid] == nil || s.whoIn(f.ImageID, f.ID)[v.pid] != nil {
				continue
			}
			taken[key] = true
			ch.group(s.placeFiled(f, v.pid))
			ch.face(f)
			filed++
		}
		err := ch.save()
		s.mu.Unlock()
		if err != nil {
			return filed, unfiled, err
		}
		if progress != nil {
			progress(filed, unfiled)
		}
	}
	return filed, unfiled, nil
}
