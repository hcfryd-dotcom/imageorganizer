package server

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"os"

	"github.com/lkarlslund/imageorganizer/internal/media"
)

// levelFace evens out the light of a face crop, so a face in shadow can be
// compared with one in daylight: the face's (the middle of the crop's)
// darkest and brightest percent are stretched to black and white, and a
// face that is still dark is lifted with a gamma curve. Colours keep their
// balance: all channels get the same curve.
func levelFace(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	img := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.Set(x, y, src.At(x, y))
		}
	}
	// Luminance spread over the middle of the crop, where the face is.
	var hist [256]int
	n := 0
	for y := b.Min.Y + b.Dy()/5; y < b.Max.Y-b.Dy()/5; y++ {
		for x := b.Min.X + b.Dx()/5; x < b.Max.X-b.Dx()/5; x++ {
			i := img.PixOffset(x, y)
			p := img.Pix[i : i+3]
			hist[(299*int(p[0])+587*int(p[1])+114*int(p[2]))/1000]++
			n++
		}
	}
	if n == 0 {
		return data, nil
	}
	at := func(frac float64) float64 {
		want, sum := int(frac*float64(n)), 0
		for v, c := range hist {
			if sum += c; sum > want {
				return float64(v)
			}
		}
		return 255
	}
	lo, hi := at(0.01), at(0.99)
	if hi-lo < 8 {
		return data, nil // flat: nothing to stretch
	}
	gain := min(255/(hi-lo), 5.0) // a very dark face gets noisy beyond this
	// The face's middle brightness after stretching; lift it towards 0.45.
	mid := min(max((at(0.5)-lo)*gain/255, 0.01), 1)
	gamma := 1.0
	if mid < 0.4 {
		gamma = math.Log(0.45) / math.Log(mid)
	}
	var curve [256]uint8
	for v := range curve {
		x := min(max((float64(v)-lo)*gain/255, 0), 1)
		curve[v] = uint8(math.Round(255 * math.Pow(x, gamma)))
	}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = curve[img.Pix[i]], curve[img.Pix[i+1]], curve[img.Pix[i+2]]
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: 88}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// turnJPEG reads a JPEG and turns it clockwise by deg (90, 180, 270).
func turnJPEG(path string, deg int) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	rgba := image.NewRGBA(src.Bounds())
	draw.Draw(rgba, rgba.Bounds(), src, src.Bounds().Min, draw.Src)
	var out bytes.Buffer
	if err := jpeg.Encode(&out, media.RotateCW(rgba, deg), &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
