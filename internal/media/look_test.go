package media

import (
	"image"
	"testing"
)

func TestLight(t *testing.T) {
	dark := image.NewGray(image.Rect(0, 0, 50, 50))
	for i := range dark.Pix {
		dark.Pix[i] = 20
	}
	if l := Looks(dark).Light; l > .1 {
		t.Errorf("dark: %v", l)
	}
	light := image.NewGray(image.Rect(0, 0, 50, 50))
	for i := range light.Pix {
		light.Pix[i] = 230
	}
	if l := Looks(light).Light; l < .85 {
		t.Errorf("light: %v", l)
	}
}
