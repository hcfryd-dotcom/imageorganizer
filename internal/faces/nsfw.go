package faces

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"log/slog"
	"math"
	"os"

	ort "github.com/yalue/onnxruntime_go"
	xdraw "golang.org/x/image/draw"

	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/pipeline"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// nsfwSize is the classifier's input size; the whole image is squeezed to
// it, as the model was trained.
const nsfwSize = 224

// HasNSFW reports whether an NSFW classifier is loaded.
func (e *Engine) HasNSFW() bool { return e != nil && e.nsfw != nil }

// NSFW is the probability that an image is not safe for work (nudity).
func (e *Engine) NSFW(img image.Image) (float32, error) {
	const S = nsfwSize
	// Smoothed when shrinking, like the model's own preprocessing; a
	// quick nearest-pixel scale changes scores near the cut-off.
	small := image.NewRGBA(image.Rect(0, 0, S, S))
	xdraw.BiLinear.Scale(small, small.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	in := make([]float32, 3*S*S)
	for y := range S {
		for x := range S {
			i := small.PixOffset(x, y)
			for c := range 3 {
				in[c*S*S+y*S+x] = (float32(small.Pix[i+c])/255 - 0.5) / 0.5
			}
		}
	}
	t, err := ort.NewTensor(ort.NewShape(1, 3, S, S), in)
	if err != nil {
		return 0, err
	}
	defer t.Destroy()
	outs := []ort.Value{nil}
	if err := e.run(e.nsfw, []ort.Value{t}, outs); err != nil {
		return 0, err
	}
	defer outs[0].Destroy()
	l := outs[0].(*ort.Tensor[float32]).GetData() // normal, nsfw
	return float32(1 / (1 + math.Exp(float64(l[0]-l[1])))), nil
}

// LookStage looks at each photo's stored thumbnail (never the original):
// how sharp it is, and, with the classifier (e may be nil), whether it is
// not safe for work.
func LookStage(e *Engine, st *store.Store, thumbPath func(hash string) string, workers int, log *slog.Logger) pipeline.Stage[store.VectorJob] {
	return pipeline.Stage[store.VectorJob]{
		Name:    "look",
		Workers: workers,
		Next:    st.NeedLook,
		Key:     func(j store.VectorJob) int64 { return j.ID },
		Do: func(ctx context.Context, j store.VectorJob) error {
			var res store.Look
			data, err := os.ReadFile(thumbPath(j.Hash))
			if err != nil {
				return st.SetLook(ctx, j.ID, j.Hash, store.Look{Nsfw: -1, Sharp: -1}) // no thumbnail: can't judge
			}
			img, err := jpeg.Decode(bytes.NewReader(data))
			if err != nil {
				return st.SetLook(ctx, j.ID, j.Hash, store.Look{Nsfw: -1, Sharp: -1})
			}
			res.Sharp = media.Sharpness(img)
			res.Nsfw = -1
			if e.HasNSFW() {
				if res.Nsfw, err = e.NSFW(img); err != nil {
					return err
				}
			}
			return st.SetLook(ctx, j.ID, j.Hash, res)
		},
		Log: log,
	}
}
