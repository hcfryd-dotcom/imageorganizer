package faces

import "math"

// Sideways looks at the eye landmarks of the faces found in an upright
// (w x h) picture and returns the clockwise rotation (90, 180, 270) that
// would put them upright when the photo itself is turned, or 0. Cameras
// sometimes record the wrong orientation; then every face lies on its side
// the same way. It needs two confident faces that agree, as one person
// lying down is no reason to turn a photo.
func Sideways(found []Face, w, h int) int {
	votes := map[int]int{}
	n := 0
	for _, f := range found {
		if f.Score < 0.6 {
			continue
		}
		n++
		lx, ly := float64(f.Landmarks[0][0])*float64(w), float64(f.Landmarks[0][1])*float64(h)
		rx, ry := float64(f.Landmarks[1][0])*float64(w), float64(f.Landmarks[1][1])*float64(h)
		// Angle of the line from the left eye to the right eye, clockwise
		// (y grows downwards): 0 for an upright face.
		a := math.Atan2(ry-ly, rx-lx) * 180 / math.Pi
		switch {
		case a > 60 && a < 120:
			votes[270]++ // turned clockwise: turn back counter-clockwise
		case a < -60 && a > -120:
			votes[90]++
		case a > 150 || a < -150:
			votes[180]++
		}
	}
	if n < 2 {
		return 0
	}
	for deg, v := range votes {
		if v == n { // every confident face agrees
			return deg
		}
	}
	return 0
}
