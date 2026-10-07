package media

import (
	"image"
	"image/color"

	"golang.org/x/image/draw"
)

// Resize scales img so its long edge is at most size pixels. Any whole
// factor of 2 or more is removed with a box filter (cheap and alias-free);
// the remaining shrink is under 2x, where bilinear sampling is visually as
// good as Catmull-Rom at a fraction of the cost. (Catmull-Rom over a 24 MP
// photo cost more than decoding it.)
func Resize(img image.Image, size int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	long := max(w, h)
	if long <= size {
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	if f := long / size; f >= 2 {
		img = boxShrink(img, f)
		b = img.Bounds()
	}
	tw, th := max(w*size/long, 1), max(h*size/long, 1)
	if rgba, ok := img.(*image.RGBA); ok && b.Dx() == tw && b.Dy() == th {
		return rgba // the box filter landed exactly on the target size
	}
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst
}

// boxShrink averages f×f blocks. It has a fast path for YCbCr, which is what
// nearly every JPEG decodes to.
func boxShrink(img image.Image, f int) *image.RGBA {
	if yc, ok := img.(*image.YCbCr); ok {
		return boxShrinkYCbCr(yc, f)
	}
	b := img.Bounds()
	w, h := b.Dx()/f, b.Dy()/f
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	n := uint32(f * f)

	pixel := func(x, y int) (r, g, bl uint32) {
		c := img.At(x, y)
		cr, cg, cb, _ := c.RGBA()
		return cr >> 8, cg >> 8, cb >> 8
	}
	if rgba, ok := img.(*image.RGBA); ok {
		pixel = func(x, y int) (uint32, uint32, uint32) {
			i := rgba.PixOffset(x, y)
			return uint32(rgba.Pix[i]), uint32(rgba.Pix[i+1]), uint32(rgba.Pix[i+2])
		}
	}

	for dy := range h {
		for dx := range w {
			var sr, sg, sb uint32
			x0, y0 := b.Min.X+dx*f, b.Min.Y+dy*f
			for y := y0; y < y0+f; y++ {
				for x := x0; x < x0+f; x++ {
					r, g, bl := pixel(x, y)
					sr, sg, sb = sr+r, sg+g, sb+bl
				}
			}
			i := dst.PixOffset(dx, dy)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = uint8(sr/n), uint8(sg/n), uint8(sb/n), 255
		}
	}
	return dst
}

// boxShrinkYCbCr averages luma and chroma planes directly and converts to
// RGB once per output pixel. The colour conversion is affine, so averaging
// before converting gives the same result as converting every source pixel
// first, at a fraction of the cost.
func boxShrinkYCbCr(yc *image.YCbCr, f int) *image.RGBA {
	b := yc.Rect
	w, h := b.Dx()/f, b.Dy()/f
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	// Chroma subsampling factors.
	hs, vs := 1, 1
	switch yc.SubsampleRatio {
	case image.YCbCrSubsampleRatio422:
		hs = 2
	case image.YCbCrSubsampleRatio420:
		hs, vs = 2, 2
	case image.YCbCrSubsampleRatio440:
		vs = 2
	case image.YCbCrSubsampleRatio411:
		hs = 4
	case image.YCbCrSubsampleRatio410:
		hs, vs = 4, 2
	}
	// Chroma planes start at the subsampled origin of the bounds.
	cx0, cy0 := b.Min.X/hs, b.Min.Y/vs

	for dy := range h {
		y0 := dy * f // relative to b.Min
		cyA, cyB := (b.Min.Y+y0)/vs-cy0, (b.Min.Y+y0+f-1)/vs-cy0
		for dx := range w {
			x0 := dx * f
			var sy uint32
			for y := y0; y < y0+f; y++ {
				row := yc.Y[y*yc.YStride+x0 : y*yc.YStride+x0+f]
				for _, v := range row {
					sy += uint32(v)
				}
			}
			cxA, cxB := (b.Min.X+x0)/hs-cx0, (b.Min.X+x0+f-1)/hs-cx0
			// Weight each chroma sample by how many of the block's pixels
			// it covers; blocks need not align with the subsampling grid.
			var scb, scr, n uint32
			for cy := cyA; cy <= cyB; cy++ {
				off := cy * yc.CStride
				wy := overlap(y0, f, (cy+cy0)*vs-b.Min.Y, vs)
				for cx := cxA; cx <= cxB; cx++ {
					wgt := wy * overlap(x0, f, (cx+cx0)*hs-b.Min.X, hs)
					scb += wgt * uint32(yc.Cb[off+cx])
					scr += wgt * uint32(yc.Cr[off+cx])
					n += wgt
				}
			}
			r, g, bl := color.YCbCrToRGB(uint8(sy/uint32(f*f)), uint8(scb/n), uint8(scr/n))
			i := dst.PixOffset(dx, dy)
			dst.Pix[i], dst.Pix[i+1], dst.Pix[i+2], dst.Pix[i+3] = r, g, bl, 255
		}
	}
	return dst
}

// overlap returns how many of [a, a+an) fall inside [b, b+bn).
func overlap(a, an, b, bn int) uint32 {
	return uint32(max(0, min(a+an, b+bn)-max(a, b)))
}

// RotateCW turns an image clockwise by 0, 90, 180 or 270 degrees.
func RotateCW(src *image.RGBA, deg int) *image.RGBA {
	switch (deg%360 + 360) % 360 {
	case 90:
		return Orient(src, 6)
	case 180:
		return Orient(src, 3)
	case 270:
		return Orient(src, 8)
	}
	return src
}

// Orient applies an EXIF orientation (1-8) so the image displays upright.
func Orient(src *image.RGBA, o int) *image.RGBA {
	if o <= 1 || o > 8 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			si, di := src.PixOffset(x, y), dst.PixOffset(nx, ny)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}

// DHash computes a 64-bit difference hash: robust to re-encoding, resizing
// and small edits, so it can recognise the same photo after it was re-saved.
func DHash(img image.Image) uint64 {
	small := image.NewGray(image.Rect(0, 0, 9, 8))
	draw.ApproxBiLinear.Scale(small, small.Bounds(), img, img.Bounds(), draw.Src, nil)
	var h uint64
	for y := range 8 {
		for x := range 8 {
			h <<= 1
			if small.GrayAt(x, y).Y < small.GrayAt(x+1, y).Y {
				h |= 1
			}
		}
	}
	return h
}
