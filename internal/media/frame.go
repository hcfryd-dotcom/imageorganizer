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
	lum := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			lum[y*w+x] = (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 65535
		}
	}
	// line tells of the n points (x0+i*dx, y0+i*dy) what share is dark
	// and how light they are on average.
	line := func(x0, y0, dx, dy, n int) (dark, mean float64) {
		for i := 0; i < n; i++ {
			v := lum[(y0+i*dy)*w+x0+i*dx]
			if v < frameDark {
				dark++
			}
			mean += v
		}
		return dark / float64(n), mean / float64(n)
	}
	// side counts the frame lines from an edge in; then the soft edge
	// between frame and photo, lines still clearly darker than the photo a
	// little further in; then one more. 0 if too few frame lines, or if
	// they go on past most (a dark photo, not a frame).
	side := func(most int, at func(i int) (float64, float64)) int {
		i := 0
		for i <= most {
			if d, _ := at(i); d < frameShare {
				break
			}
			i++
		}
		if i < frameLeast || i > most {
			return 0
		}
		for soft := 0; soft < frameSoft; soft++ {
			_, m := at(i)
			_, in := at(i + frameSoft)
			if m >= 0.9*in {
				break
			}
			i++
		}
		return i + 1
	}
	mw, mh := int(float64(w)*frameMost), int(float64(h)*frameMost)
	l := side(mw, func(i int) (float64, float64) { return line(i, 0, 0, 1, h) })
	r := side(mw, func(i int) (float64, float64) { return line(w-1-i, 0, 0, 1, h) })
	t := side(mh, func(i int) (float64, float64) { return line(0, i, 1, 0, w) })
	bt := side(mh, func(i int) (float64, float64) { return line(0, h-1-i, 1, 0, w) })
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
	frameSoft  = 4    // lines of soft edge after the frame, at most
)
