//go:build cgo

package faces

import (
	"bytes"
	"errors"
	"image"
	"image/draw"
	"image/jpeg"
	"math"

	ort "github.com/yalue/onnxruntime_go"
)

// Eyes: InsightFace's 106-point landmark model (2d106det, from the same
// buffalo_l pack as the detector) places points round each eye; how tall
// an eye is against how wide tells open from shut.

// lmkSize is the landmark model's input: a 192×192 square round the face,
// one and a half faces wide.
const lmkSize = 192

// minEyesFace is the smallest face, in pixels across, whose eyes are judged.
const minEyesFace = 40

// eyePoints are the landmarks round each eye (and its pupil) in the
// 106-point layout.
var eyePoints = [2][]int{
	{33, 34, 35, 36, 37, 38, 39, 40, 41, 42},
	{87, 88, 89, 90, 91, 92, 93, 94, 95, 96},
}

// HasEyes reports whether the landmark model is loaded.
func (e *Engine) HasEyes() bool { return e.lmk != nil }

// landmarks returns the 106 points of the face at box (pixels of img).
func (e *Engine) landmarks(img *image.RGBA, box [4]float32) ([106][2]float32, error) {
	var pts [106][2]float32
	cx, cy := (box[0]+box[2])/2, (box[1]+box[3])/2
	s := float32(lmkSize) / (1.5 * max(box[2]-box[0], box[3]-box[1]))
	m := affine{a: s, tx: lmkSize/2 - s*cx, ty: lmkSize/2 - s*cy}
	crop := warp(img, m, lmkSize, lmkSize)
	in := make([]float32, 3*lmkSize*lmkSize)
	plane := lmkSize * lmkSize
	for y := range lmkSize {
		for x := range lmkSize {
			i := crop.PixOffset(x, y)
			o := y*lmkSize + x
			// The model normalises its input itself: plain 0..255 RGB.
			in[o] = float32(crop.Pix[i])
			in[plane+o] = float32(crop.Pix[i+1])
			in[2*plane+o] = float32(crop.Pix[i+2])
		}
	}
	t, err := ort.NewTensor(ort.NewShape(1, 3, lmkSize, lmkSize), in)
	if err != nil {
		return pts, err
	}
	defer t.Destroy()
	outs := []ort.Value{nil}
	if err := e.run(e.lmk, []ort.Value{t}, outs); err != nil {
		return pts, err
	}
	defer outs[0].Destroy()
	d := outs[0].(*ort.Tensor[float32]).GetData()
	if len(d) < 212 {
		return pts, errors.New("landmarks: short output")
	}
	for i := range 106 {
		u, v := (d[2*i]+1)*lmkSize/2, (d[2*i+1]+1)*lmkSize/2
		pts[i] = [2]float32{(u - m.tx) / s, (v - m.ty) / s}
	}
	return pts, nil
}

// eyeOpen is how tall an eye is against how wide: its points' spread
// across the line through its two ends, over that line's length. About
// 0.3 for an open eye, under 0.15 for a shut one.
func eyeOpen(pts [106][2]float32, idx []int) float32 {
	// The two points furthest apart are the eye's corners.
	var a, b [2]float32
	best := float32(-1)
	for _, i := range idx {
		for _, j := range idx {
			dx, dy := pts[i][0]-pts[j][0], pts[i][1]-pts[j][1]
			if d := dx*dx + dy*dy; d > best {
				best, a, b = d, pts[i], pts[j]
			}
		}
	}
	w := float32(math.Sqrt(float64(best)))
	if w == 0 {
		return 0
	}
	ux, uy := (b[0]-a[0])/w, (b[1]-a[1])/w
	lo, hi := float32(0), float32(0)
	for _, i := range idx {
		d := (pts[i][0]-a[0])*-uy + (pts[i][1]-a[1])*ux // across the line
		lo, hi = min(lo, d), max(hi, d)
	}
	return (hi - lo) / w
}

// EyesFromCrop judges the eyes of the face at box (0..1) inside a stored
// face crop: how open the more open eye is (eyeOpen). ok is false when
// the face is too small to judge.
func (e *Engine) EyesFromCrop(crop []byte, box [4]float32) (open float32, ok bool, err error) {
	if e.lmk == nil {
		return 0, false, errors.New("no landmark model")
	}
	src, err := jpeg.Decode(bytes.NewReader(crop))
	if err != nil {
		return 0, false, err
	}
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	W, H := float32(b.Dx()), float32(b.Dy())
	px := [4]float32{box[0] * W, box[1] * H, box[2] * W, box[3] * H}
	if px[2]-px[0] < minEyesFace {
		return 0, false, nil
	}
	pts, err := e.landmarks(img, px)
	if err != nil {
		return 0, false, err
	}
	// The more open eye: a wink, or an eye hidden by hair, is not shut eyes.
	return max(eyeOpen(pts, eyePoints[0]), eyeOpen(pts, eyePoints[1])), true, nil
}
