package media

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/draw"
)

// A detailed image reads sharp; the same image blurred (shrunk and
// enlarged) reads blurry.
func TestSharpness(t *testing.T) {
	const n = 256
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			v := uint8(0)
			if (x/4+y/4)%2 == 0 {
				v = 255
			}
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	small := image.NewRGBA(image.Rect(0, 0, n/16, n/16))
	draw.BiLinear.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)
	soft := image.NewRGBA(img.Bounds())
	draw.BiLinear.Scale(soft, soft.Bounds(), small, small.Bounds(), draw.Src, nil)
	sharp, blurred := Sharpness(img), Sharpness(soft)
	if sharp < 1000 || blurred > 50 {
		t.Errorf("sharp %.0f, blurred %.0f", sharp, blurred)
	}
}
