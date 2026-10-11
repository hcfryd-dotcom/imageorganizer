package media

import (
	"image"
	"slices"
)

// Sharpness measures how sharp a photo is, from its thumbnail: the
// variance of the Laplacian (fine detail) in its sharpest parts. The image
// is cut into a 4×4 grid and the second sharpest tile counts, so a sharp
// subject before a soft background reads sharp, and a single noisy or
// high-contrast patch doesn't make a blurry photo look sharp. Values are
// on the thumbnail's 0–255 scale; below about 50 a photo looks blurry.
func Sharpness(img image.Image) float32 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 16 || h < 16 {
		return 0
	}
	gray := make([]float32, w*h)
	for y := range h {
		for x := range w {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			gray[y*w+x] = (0.299*float32(r) + 0.587*float32(g) + 0.114*float32(bl)) / 257
		}
	}
	const n = 4
	var tiles []float32
	for ty := range n {
		for tx := range n {
			x0, x1 := max(1, tx*w/n), min(w-1, (tx+1)*w/n)
			y0, y1 := max(1, ty*h/n), min(h-1, (ty+1)*h/n)
			var sum, sq float64
			cnt := 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					i := y*w + x
					l := float64(4*gray[i] - gray[i-1] - gray[i+1] - gray[i-w] - gray[i+w])
					sum += l
					sq += l * l
					cnt++
				}
			}
			if cnt > 0 {
				m := sum / float64(cnt)
				tiles = append(tiles, float32(sq/float64(cnt)-m*m))
			}
		}
	}
	slices.Sort(tiles)
	return tiles[max(0, len(tiles)-2)]
}
