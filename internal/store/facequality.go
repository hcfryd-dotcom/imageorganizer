package store

import (
	"bytes"
	"image"
	"image/jpeg"
	"math"
	"os"
)

// faceQuality is measured on a face's stored crop, for picking a person's
// picture: landmarks can't tell blur or closed eyes.
type faceQuality struct {
	sharp float32 // Laplacian variance over intensity variance: higher is sharper
	eyes  float32 // contrast around the eyes relative to the face: open eyes (iris on white) score higher
	ok    bool
}

// quality measures (and remembers) a face's crop. CropPath must be set.
func (s *Store) quality(f *faceRec) faceQuality {
	s.qmu.Lock()
	q, seen := s.qcache[f.ID]
	s.qmu.Unlock()
	if seen {
		return q
	}
	q = s.measure(f)
	s.qmu.Lock()
	if s.qcache == nil {
		s.qcache = map[int64]faceQuality{}
	}
	s.qcache[f.ID] = q
	s.qmu.Unlock()
	return q
}

func (s *Store) measure(f *faceRec) faceQuality {
	im := s.images[f.ImageID]
	if s.CropPath == nil || im == nil || f.CropBox[2] <= f.CropBox[0] || f.CropBox[3] <= f.CropBox[1] {
		return faceQuality{}
	}
	data, err := os.ReadFile(s.CropPath(im.Hash + "_" + itoa(f.Index)))
	if err != nil {
		return faceQuality{}
	}
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return faceQuality{}
	}
	yc, ok := img.(*image.YCbCr)
	if !ok {
		return faceQuality{}
	}
	b := yc.Bounds()
	cw, ch := float64(b.Dx()), float64(b.Dy())
	lum := func(x, y int) float64 { return float64(yc.Y[yc.YOffset(x, y)]) }
	// The face box inside the crop, in crop pixels.
	fx0, fy0 := int(float64(f.CropBox[0])*cw), int(float64(f.CropBox[1])*ch)
	fx1, fy1 := int(float64(f.CropBox[2])*cw), int(float64(f.CropBox[3])*ch)
	if fx1-fx0 < 16 || fy1-fy0 < 16 {
		return faceQuality{}
	}
	// Sharpness: variance of the Laplacian over the face, against the
	// face's own intensity variance (so lighting and contrast cancel out).
	var lsum, lsq, isum, isq float64
	n := 0
	for y := max(fy0, 1); y < min(fy1, b.Dy()-1); y++ {
		for x := max(fx0, 1); x < min(fx1, b.Dx()-1); x++ {
			v := lum(x, y)
			l := 4*v - lum(x-1, y) - lum(x+1, y) - lum(x, y-1) - lum(x, y+1)
			lsum += l
			lsq += l * l
			isum += v
			isq += v * v
			n++
		}
	}
	if n == 0 {
		return faceQuality{}
	}
	lvar := lsq/float64(n) - (lsum/float64(n))*(lsum/float64(n))
	ivar := isq/float64(n) - (isum/float64(n))*(isum/float64(n))
	q := faceQuality{ok: true}
	if ivar > 1 {
		q.sharp = float32(lvar / ivar)
	}
	// Eyes: map the eye landmarks (relative to the photo) into the crop.
	W, H := float64(im.Width), float64(im.Height)
	cropW := float64(f.Box[2]-f.Box[0]) * W / float64(f.CropBox[2]-f.CropBox[0])
	cropH := float64(f.Box[3]-f.Box[1]) * H / float64(f.CropBox[3]-f.CropBox[1])
	x0 := float64(f.Box[0])*W - float64(f.CropBox[0])*cropW
	y0 := float64(f.Box[1])*H - float64(f.CropBox[1])*cropH
	at := func(p [2]float32) (float64, float64) {
		return (float64(p[0])*W - x0) / cropW * cw, (float64(p[1])*H - y0) / cropH * ch
	}
	lx, ly := at(f.Landmarks[0])
	rx, ry := at(f.Landmarks[1])
	r := int(0.18 * math.Hypot(rx-lx, ry-ly))
	if r < 2 {
		return q
	}
	patch := func(cx, cy float64) float64 {
		var sum, sq float64
		m := 0
		for y := int(cy) - r; y <= int(cy)+r; y++ {
			for x := int(cx) - r; x <= int(cx)+r; x++ {
				if x < 0 || y < 0 || x >= b.Dx() || y >= b.Dy() {
					continue
				}
				v := lum(x, y)
				sum += v
				sq += v * v
				m++
			}
		}
		if m == 0 {
			return 0
		}
		return math.Sqrt(max(0, sq/float64(m)-(sum/float64(m))*(sum/float64(m))))
	}
	if sd := math.Sqrt(max(ivar, 1)); sd > 0 {
		q.eyes = float32((patch(lx, ly) + patch(rx, ry)) / 2 / sd)
	}
	return q
}

func itoa(i int) string {
	return string(appendInt(nil, i))
}

func appendInt(b []byte, i int) []byte {
	if i >= 10 {
		b = appendInt(b, i/10)
	}
	return append(b, byte('0'+i%10))
}
