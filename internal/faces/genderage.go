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

// InsightFace's genderage model (from the buffalo_l pack, like the
// detector) tells gender when the age model does not (MiVOLO's age-only
// export). Its own age guess is coarse and not used.

// gaSize is its input: a 96×96 square round the face, one and a half
// faces wide.
const gaSize = 96

// gaBias moves its readings toward female: on 120 adult faces of eight
// people of the owner's (2026-10-15) its plain readings averaged 0.61
// for women and 0.11 for men, short of the 0.75 the store needs to call
// a person a woman (gender.go); moved by 2 they average 0.81 and 0.24,
// and 98 of 120 faces read right.
const gaBias = 2.0

// femaleFrom estimates the probability that the face at box (pixels of
// img) is a woman's or girl's.
func (e *Engine) femaleFrom(img *image.RGBA, box [4]float32) (float32, error) {
	if e.ga == nil {
		return -1, errors.New("no gender model")
	}
	cx, cy := (box[0]+box[2])/2, (box[1]+box[3])/2
	s := float32(gaSize) / (1.5 * max(box[2]-box[0], box[3]-box[1]))
	m := affine{a: s, tx: gaSize/2 - s*cx, ty: gaSize/2 - s*cy}
	crop := warp(img, m, gaSize, gaSize)
	in := make([]float32, 3*gaSize*gaSize)
	plane := gaSize * gaSize
	for y := range gaSize {
		for x := range gaSize {
			i := crop.PixOffset(x, y)
			o := y*gaSize + x
			// Like the landmark model, it takes plain 0..255 RGB.
			in[o] = float32(crop.Pix[i])
			in[plane+o] = float32(crop.Pix[i+1])
			in[2*plane+o] = float32(crop.Pix[i+2])
		}
	}
	t, err := ort.NewTensor(ort.NewShape(1, 3, gaSize, gaSize), in)
	if err != nil {
		return -1, err
	}
	defer t.Destroy()
	outs := []ort.Value{nil}
	if err := e.run(e.ga, []ort.Value{t}, outs); err != nil {
		return -1, err
	}
	defer outs[0].Destroy()
	d := outs[0].(*ort.Tensor[float32]).GetData()
	if len(d) < 2 {
		return -1, errors.New("gender: short output")
	}
	// Scores for female and male; their softmax, moved by gaBias.
	return float32(1 / (1 + math.Exp(float64(d[1]-d[0]-gaBias)))), nil
}

// GenderFromCrop estimates gender alone, for faces whose age is known:
// with the genderage model it skips the far heavier age model. ok is
// false when the face is too small to judge or no model tells gender.
func (e *Engine) GenderFromCrop(crop []byte, box [4]float32) (female float32, ok bool, err error) {
	if e.ga == nil {
		est, err := e.EstimateFromCrop(crop, box)
		return est.Female, est.OK && est.Female >= 0, err
	}
	src, err := jpeg.Decode(bytes.NewReader(crop))
	if err != nil {
		return -1, false, err
	}
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	W, H := float32(b.Dx()), float32(b.Dy())
	px := [4]float32{box[0] * W, box[1] * H, box[2] * W, box[3] * H}
	if px[2]-px[0] < minAgeFace {
		return -1, false, nil
	}
	f, err := e.femaleFrom(img, px)
	return f, err == nil, err
}
