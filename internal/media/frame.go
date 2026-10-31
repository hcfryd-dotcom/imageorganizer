package media

import "image"

// Frame finds a dark frame around a photo, as on a scanned print or a
// studio photo with a black edge: on each side, the rows or columns from
// the edge in that are nearly all dark. It returns how much of the photo
// each side takes (left, top, right, bottom, as shares of its width and
// height), or all 0 when there is no frame. A frame goes all the way
// round and is narrow (at most frameMost of a side): a dark photo, or one
// with something dark along one edge, has none.
func Frame(img image.Image) [4]float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 20 || h < 20 {
		return [4]float64{}
	}
	dark := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dark[y*w+x] = (0.299*float64(r)+0.587*float64(g)+0.114*float64(bl))/65535 < frameDark
		}
	}
	// line reports whether the n points (x0+i*dx, y0+i*dy) are dark enough
	// to be frame.
	line := func(x0, y0, dx, dy, n int) bool {
		d := 0
		for i := 0; i < n; i++ {
			if dark[(y0+i*dy)*w+x0+i*dx] {
				d++
			}
		}
		return float64(d) >= frameShare*float64(n)
	}
	// side counts the frame lines from an edge in, plus one for the soft
	// edge between frame and photo; 0 if too few, or if they go on past
	// most (a dark photo, not a frame).
	side := func(most int, isFrame func(i int) bool) int {
		i := 0
		for i <= most && isFrame(i) {
			i++
		}
		if i < frameLeast || i > most {
			return 0
		}
		return i + 1
	}
	mw, mh := int(float64(w)*frameMost), int(float64(h)*frameMost)
	l := side(mw, func(i int) bool { return line(i, 0, 0, 1, h) })
	r := side(mw, func(i int) bool { return line(w-1-i, 0, 0, 1, h) })
	t := side(mh, func(i int) bool { return line(0, i, 1, 0, w) })
	bt := side(mh, func(i int) bool { return line(0, h-1-i, 1, 0, w) })
	if l == 0 || r == 0 || t == 0 || bt == 0 {
		return [4]float64{}
	}
	return [4]float64{float64(l) / float64(w), float64(t) / float64(h), float64(r) / float64(w), float64(bt) / float64(h)}
}

const (
	frameDark  = 0.35 // a pixel darker than this (0 black, 1 white) may be frame
	frameShare = 0.9  // a line is frame when this share of it is dark
	frameLeast = 2    // fewer frame lines than this (of a thumbnail) is no frame
	frameMost  = 0.06 // a side takes at most this share of the photo
)
