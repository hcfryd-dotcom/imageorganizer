package store

import "math"

// Junk detections: things the face detector took for a face (an animal, a
// toy, a pattern, the back of a head). The face model maps all
// of these to much the same place, so they share a direction: the mean of
// the detections the detector was least sure of. Measured on a real
// library (eval_test.go): similarity >= junkMin to it caught 99% of a
// group of 298 such detections, and none of a sample of real faces.
const (
	junkScore = 0.55 // detections below this confidence define the direction
	junkMin   = 0.40 // similarity to the direction that marks a detection as junk
	junkSeed  = 200  // fewer low-confidence detections than this: no junk detection
)

// computeJunkDir derives the junk direction from the library's own
// low-confidence detections. Call with the faces loaded.
func (s *Store) computeJunkDir() {
	var sum []float64
	n := 0
	for _, f := range s.faces {
		if f.Score >= junkScore || f.NotFace || len(f.emb) == 0 {
			continue
		}
		if sum == nil {
			sum = make([]float64, len(f.emb))
		}
		for i, x := range f.emb {
			sum[i] += float64(x)
		}
		n++
	}
	s.junkDir = nil
	if n < junkSeed {
		return
	}
	var nn float64
	for _, x := range sum {
		nn += x * x
	}
	nn = math.Sqrt(nn)
	s.junkDir = make([]float32, len(sum))
	for i, x := range sum {
		s.junkDir[i] = float32(x / nn)
	}
}

// isJunk reports whether a detection is most likely not a face. Junk is
// kept (so re-analysis doesn't bring it back as new) but never grouped,
// named or shown as a person.
// shown reports whether a face is shown in its photo: not said not to be
// a face, nor an ungrouped detection that looks like junk. Questions about
// a photo go by these faces only, or they'd speak of faces no one sees.
func (s *Store) shown(f *faceRec) bool {
	return !f.NotFace && !(f.ClusterID == 0 && s.isJunk(f))
}

func (s *Store) isJunk(f *faceRec) bool {
	if s.junkDir == nil || len(f.emb) != len(s.junkDir) {
		return false
	}
	var d, n float64
	for i, x := range f.emb {
		d += float64(x) * float64(s.junkDir[i])
		n += float64(x) * float64(x)
	}
	return n > 0 && d/math.Sqrt(n) >= junkMin
}
