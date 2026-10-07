package faces

import "testing"

// face makes a face with eyes at the given pixel positions in a w x h picture.
func face(lx, ly, rx, ry float32, w, h float32) Face {
	return Face{Score: 0.9, Landmarks: [5][2]float32{{lx / w, ly / h}, {rx / w, ry / h}}}
}

// rotCW maps a point of a w x h picture turned 90° clockwise.
func rotCW(x, y, h float32) (float32, float32) { return h - 1 - y, x }

func TestSideways(t *testing.T) {
	const w, h = 400, 300
	upright := []Face{face(100, 100, 140, 100, w, h), face(250, 120, 290, 121, w, h)}
	if got := Sideways(upright, w, h); got != 0 {
		t.Errorf("upright faces: %d, want 0", got)
	}
	// The same photo with its content turned clockwise (now 300 x 400):
	// turning it back needs 270° clockwise.
	var turned []Face
	for _, f := range []([4]float32){{100, 100, 140, 100}, {250, 120, 290, 121}} {
		lx, ly := rotCW(f[0], f[1], h)
		rx, ry := rotCW(f[2], f[3], h)
		turned = append(turned, face(lx, ly, rx, ry, h, w))
	}
	if got := Sideways(turned, h, w); got != 270 {
		t.Errorf("content turned clockwise: %d, want 270", got)
	}
	// One person lying down is no reason to turn the photo.
	if got := Sideways(turned[:1], h, w); got != 0 {
		t.Errorf("single sideways face: %d, want 0", got)
	}
	// Faces disagreeing: leave it.
	if got := Sideways([]Face{turned[0], upright[0]}, h, w); got != 0 {
		t.Errorf("mixed faces: %d, want 0", got)
	}
}
