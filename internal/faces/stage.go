package faces

import (
	"context"
	"log/slog"
	"os"
	"strconv"

	"github.com/lkarlslund/imageorganizer/internal/pipeline"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// AgeStage estimates apparent ages from stored face crops. It is separate
// from import because the age model is far heavier than detection and
// recognition, and needs no access to the original photos.
func AgeStage(e *Engine, st *store.Store, cropPath func(key string) string, workers int, log *slog.Logger) pipeline.Stage[store.AgeJob] {
	return pipeline.Stage[store.AgeJob]{
		Name:    "age",
		Workers: workers,
		Next:    st.NeedAge,
		Key:     func(j store.AgeJob) int64 { return j.FaceID },
		Do: func(ctx context.Context, j store.AgeJob) error {
			crop, err := os.ReadFile(cropPath(CropKey(j.Hash, j.Index)))
			if err != nil {
				return st.SetAge(ctx, j.FaceID, -1) // no crop: can't judge
			}
			age, ok, err := e.AgeFromCrop(crop, j.CropBox)
			if err != nil {
				return err
			}
			if !ok {
				age = -1
			}
			return st.SetAge(ctx, j.FaceID, age)
		},
		Log: log,
	}
}

// CropKey names the crop of face i of the image with this content hash.
func CropKey(hash string, i int) string { return hash + "_" + strconv.Itoa(i) }
