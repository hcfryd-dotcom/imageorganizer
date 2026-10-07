package faces

import (
	"image"

	"golang.org/x/image/draw"
)

func resizeRGBA(src *image.RGBA, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, max(w, 1), max(h, 1)))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}

// affine maps source to destination: u = a*x - b*y + tx, v = b*x + a*y + ty
// (a similarity: rotation, uniform scale, translation).
type affine struct{ a, b, tx, ty float32 }

// similarity estimates the least-squares similarity transform taking src
// points onto dst (Umeyama's method, closed form in 2D).
func similarity(src, dst [5][2]float32) affine {
	var mx, my, mu, mv float32
	for i := range 5 {
		mx += src[i][0] / 5
		my += src[i][1] / 5
		mu += dst[i][0] / 5
		mv += dst[i][1] / 5
	}
	var sa, sb, v float32
	for i := range 5 {
		x, y := src[i][0]-mx, src[i][1]-my
		u, w := dst[i][0]-mu, dst[i][1]-mv
		sa += x*u + y*w
		sb += x*w - y*u
		v += x*x + y*y
	}
	if v == 0 {
		return affine{a: 1}
	}
	a, b := sa/v, sb/v
	return affine{a: a, b: b, tx: mu - (a*mx - b*my), ty: mv - (b*mx + a*my)}
}

// warp renders a w×h image where each output pixel (u, v) samples src at
// the inverse transform of m, bilinearly; outside pixels are black.
func warp(src *image.RGBA, m affine, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	det := m.a*m.a + m.b*m.b
	if det == 0 {
		return dst
	}
	ia, ib := m.a/det, m.b/det // inverse rotation-scale
	b := src.Bounds()
	for v := range h {
		for u := range w {
			du, dv := float32(u)-m.tx, float32(v)-m.ty
			x := ia*du + ib*dv
			y := -ib*du + ia*dv
			sampleBilinear(src, b, x, y, dst.Pix[dst.PixOffset(u, v):])
		}
	}
	return dst
}

func sampleBilinear(src *image.RGBA, b image.Rectangle, x, y float32, out []uint8) {
	x0, y0 := int(floor(x)), int(floor(y))
	fx, fy := x-float32(x0), y-float32(y0)
	var acc [3]float32
	for _, p := range [4]struct {
		dx, dy int
		w      float32
	}{{0, 0, (1 - fx) * (1 - fy)}, {1, 0, fx * (1 - fy)}, {0, 1, (1 - fx) * fy}, {1, 1, fx * fy}} {
		px, py := x0+p.dx, y0+p.dy
		if px < b.Min.X || py < b.Min.Y || px >= b.Max.X || py >= b.Max.Y {
			continue
		}
		i := src.PixOffset(px, py)
		acc[0] += p.w * float32(src.Pix[i])
		acc[1] += p.w * float32(src.Pix[i+1])
		acc[2] += p.w * float32(src.Pix[i+2])
	}
	out[0], out[1], out[2], out[3] = uint8(acc[0]+0.5), uint8(acc[1]+0.5), uint8(acc[2]+0.5), 255
}

func floor(f float32) float32 {
	i := float32(int(f))
	if f < 0 && i != f {
		return i - 1
	}
	return i
}

// letterbox fits the box region into an S×S square, keeping the aspect
// ratio, centred on black.
func letterbox(src *image.RGBA, box [4]float32, S int) *image.RGBA {
	r := image.Rect(int(box[0]), int(box[1]), int(box[2]+0.5), int(box[3]+0.5)).Intersect(src.Bounds())
	dst := image.NewRGBA(image.Rect(0, 0, S, S))
	for i := 3; i < len(dst.Pix); i += 4 {
		dst.Pix[i] = 255
	}
	if r.Empty() {
		return dst
	}
	scale := float32(S) / float32(max(r.Dx(), r.Dy()))
	w, h := int(float32(r.Dx())*scale), int(float32(r.Dy())*scale)
	ox, oy := (S-w)/2, (S-h)/2
	draw.BiLinear.Scale(dst, image.Rect(ox, oy, ox+w, oy+h), src, r, draw.Src, nil)
	return dst
}
