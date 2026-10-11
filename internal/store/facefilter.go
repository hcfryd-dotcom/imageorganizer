package store

// Search's face filters: photos with someone not named yet ($unknown),
// someone filed by itself but not confirmed ($unconfirmed), someone the
// user confirmed or named ($confirmed), or either of the last two ($known);
// one(…) and many(…) ask for exactly one or at least two of them.

// FaceKinds are the kinds of face search can filter by.
var FaceKinds = []string{"known", "unknown", "confirmed", "unconfirmed"}

// FaceCounts are how many of a kind: "" at least one, "one" exactly one,
// "many" two or more.
var FaceCounts = []string{"", "one", "many"}

// faceKind is what a face counts as in a photo ("confirmed", "unconfirmed",
// "unknown"), and who: a named person counts once however often they're
// in it. "" for faces that count as nobody: non-faces, junk, pictures,
// hidden groups (strangers, pets), and background faces nobody named.
func (s *Store) faceKind(f *faceRec) (string, int64) {
	if f.NotFace || (f.ClusterID == 0 && s.isJunk(f)) || s.notSomeone(f) {
		return "", 0
	}
	c := s.clusters[f.ClusterID]
	switch {
	case c != nil && c.Hidden:
		return "", 0
	case c != nil && c.PersonID != 0 && f.Tagged == c.PersonID:
		return "confirmed", c.PersonID
	case c != nil && c.PersonID != 0:
		return "unconfirmed", c.PersonID
	case f.bg:
		return "", 0
	}
	return "unknown", -f.ID
}

// photoFaceCounts is, per photo, how many people of each kind are in it
// (known is confirmed and unconfirmed together).
func (s *Store) photoFaceCounts() map[int64]map[string]int {
	who := map[int64]map[int64]string{} // photo → person (or −face) → kind
	for _, f := range s.faces {
		k, id := s.faceKind(f)
		if k == "" {
			continue
		}
		m := who[f.ImageID]
		if m == nil {
			m = map[int64]string{}
			who[f.ImageID] = m
		}
		if m[id] != "confirmed" { // confirmed once is confirmed
			m[id] = k
		}
	}
	out := make(map[int64]map[string]int, len(who))
	for img, m := range who {
		n := map[string]int{}
		for _, k := range m {
			n[k]++
			if k != "unknown" {
				n["known"]++
			}
		}
		out[img] = n
	}
	return out
}

// countFits reports whether n fits count ("", "one", "many").
func countFits(n int, count string) bool {
	switch count {
	case "one":
		return n == 1
	case "many":
		return n >= 2
	}
	return n >= 1
}

// FaceImageIDs returns photos with faces of a kind (FaceKinds), as many as
// count says (FaceCounts).
func (s *Store) FaceImageIDs(kind, count string) []int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []int64
	for img, n := range s.photoFaceCounts() {
		if countFits(n[kind], count) {
			out = append(out, img)
		}
	}
	return out
}

// FacePhotoCounts is how many photos each face filter finds, by its name
// ("known", "one-known", "many-unknown", …).
func (s *Store) FacePhotoCounts() map[string]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string]int{}
	for _, n := range s.photoFaceCounts() {
		for _, k := range FaceKinds {
			for _, c := range FaceCounts {
				if countFits(n[k], c) {
					out[FaceFilterName(k, c)]++
				}
			}
		}
	}
	return out
}

// FaceFilterName is a face filter's name: the kind, or count-kind.
func FaceFilterName(kind, count string) string {
	if count == "" {
		return kind
	}
	return count + "-" + kind
}
