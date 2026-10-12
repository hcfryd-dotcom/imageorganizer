package faces

import (
	"bytes"
	"context"
	"image/jpeg"
	"log/slog"
	"os"

	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/pipeline"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

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
