package media

import "image"

// Look is what a photo book wants to know of how a photo looks, read
// from its thumbnail. (How much a photo leans was tried too, from the
// direction of its edges, but clothes, hair and leaves fooled it on a
// third of real photos; turning is left to the user.)
type Look struct {
	Light float64 // mean brightness, 0 (black) to 1 (white)
}

// Looks measures img (best a thumbnail: a few hundred pixels).
func Looks(img image.Image) Look {
	b := img.Bounds()
	if b.Empty() {
		return Look{}
	}
	s := 0.0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			s += (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 65535
		}
	}
	return Look{Light: s / float64(b.Dx()*b.Dy())}
}
