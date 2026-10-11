package store

// How strongly a face is known to be someone. What the user said of a
// face alone (it's them, or it isn't) is the strongest; confirming it with
// others (a batch, a whole group) less so; filed by itself the weakest:
// rechecked every pass, and outweighed by anything the user says later.
// Rules that read who's who from other faces (who is in a photo already,
// which group is someone else) take a weak say as a hint, not a fact, so
// that what's known better later changes what follows from it.
type Strength int

const (
	StrengthNone    Strength = iota
	StrengthFiled            // filed by itself (FileFaces)
	StrengthNamed            // confirmed along with others: a batch, a group
	StrengthChecked          // confirmed alone: in the photo, "Is this really …?"
)

func (st Strength) String() string {
	return [...]string{"", "filed", "named", "checked"}[st]
}

// faceStrength is who face f is and how strongly that's known.
func (s *Store) faceStrength(f *faceRec) (int64, Strength) {
	c := s.clusters[f.ClusterID]
	switch {
	case c == nil || c.PersonID == 0:
		return 0, StrengthNone
	case f.Tagged == c.PersonID && f.Checked:
		return c.PersonID, StrengthChecked
	case f.Tagged == c.PersonID:
		return c.PersonID, StrengthNamed
	}
	return c.PersonID, StrengthFiled
}

// sharedStrength is how strongly group h's faces in the photos it shares
// with group c are known to be h's person: the strongest of them.
func (s *Store) sharedStrength(c, h *cluster) Strength {
	best := StrengthNone
	for img := range c.imgs {
		if h.imgs[img] == 0 {
			continue
		}
		for _, id := range s.facesByImage[img] {
			if f := s.faces[id]; f != nil && f.ClusterID == h.id {
				if pid, st := s.faceStrength(f); pid == h.PersonID {
					best = max(best, st)
				}
			}
		}
	}
	return best
}
