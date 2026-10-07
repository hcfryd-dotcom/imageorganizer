package media

import (
	"golang.org/x/image/draw"
	"image"
	"math/rand/v2"
	"testing"
)

// opaque hides the concrete type so boxShrink takes its generic path.
type opaque struct{ image.Image }

func testYCbCr(w, h int, ratio image.YCbCrSubsampleRatio) *image.YCbCr {
	img := image.NewYCbCr(image.Rect(0, 0, w, h), ratio)
	r := rand.New(rand.NewPCG(1, 2))
	// Values stay inside the RGB gamut: where per-pixel conversion would
	// clamp, averaging before or after conversion legitimately differs.
	for i := range img.Y {
		img.Y[i] = uint8(60 + i/7%100 + r.IntN(30)) // gradient plus noise
	}
	for i := range img.Cb {
		img.Cb[i] = uint8(112 + r.IntN(32))
		img.Cr[i] = uint8(112 + r.IntN(32))
	}
	return img
}

func TestBoxShrinkYCbCrMatchesGeneric(t *testing.T) {
	for _, ratio := range []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420,
		image.YCbCrSubsampleRatio440, image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	} {
		src := testYCbCr(403, 301, ratio) // odd sizes exercise edges
		for _, f := range []int{2, 3, 4, 8} {
			fast, slow := boxShrink(src, f), boxShrink(opaque{src}, f)
			if fast.Bounds() != slow.Bounds() {
				t.Fatalf("%v f=%d: bounds %v vs %v", ratio, f, fast.Bounds(), slow.Bounds())
			}
			worst := 0
			for i := range fast.Pix {
				d := int(fast.Pix[i]) - int(slow.Pix[i])
				worst = max(worst, d, -d)
			}
			// Averaging before vs after the YCbCr->RGB conversion differs
			// only by rounding and clamping.
			if worst > 3 {
				t.Errorf("%v f=%d: max channel difference %d", ratio, f, worst)
			}
		}
	}
}

func BenchmarkBoxShrink24MP(b *testing.B) {
	src := testYCbCr(6000, 4000, image.YCbCrSubsampleRatio420)
	b.Run("ycbcr", func(b *testing.B) {
		for b.Loop() {
			boxShrink(src, 5)
		}
	})
	b.Run("generic", func(b *testing.B) {
		for b.Loop() {
			boxShrink(opaque{src}, 5)
		}
	})
}

func BenchmarkResizeWork24MP(b *testing.B) {
	src := testYCbCr(6000, 4000, image.YCbCrSubsampleRatio420)
	for b.Loop() {
		Resize(src, WorkSize)
	}
}

func BenchmarkResizeParts(b *testing.B) {
	src := testYCbCr(6000, 4000, image.YCbCrSubsampleRatio420)
	half := boxShrink(src, 2)
	b.Run("box/2", func(b *testing.B) {
		for b.Loop() {
			boxShrink(src, 2)
		}
	})
	dst := image.NewRGBA(image.Rect(0, 0, 2048, 1365))
	b.Run("catmull-3000-to-2048", func(b *testing.B) {
		for b.Loop() {
			draw.CatmullRom.Scale(dst, dst.Bounds(), half, half.Bounds(), draw.Src, nil)
		}
	})
	b.Run("bilinear-3000-to-2048", func(b *testing.B) {
		for b.Loop() {
			draw.BiLinear.Scale(dst, dst.Bounds(), half, half.Bounds(), draw.Src, nil)
		}
	})
	b.Run("approxbilinear-3000-to-2048", func(b *testing.B) {
		for b.Loop() {
			draw.ApproxBiLinear.Scale(dst, dst.Bounds(), half, half.Bounds(), draw.Src, nil)
		}
	})
}
