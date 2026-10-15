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
			if err != nil { // no crop: can't judge
				if err := st.SetGender(ctx, j.FaceID, -1); err != nil {
					return err
				}
				if j.GenderOnly {
					return nil
				}
				return st.SetAge(ctx, j.FaceID, -1)
			}
			est, err := e.EstimateFromCrop(crop, j.CropBox)
			if err != nil {
				return err
			}
			if !est.OK {
				est.Age, est.Female = -1, -1
			}
			if err := st.SetGender(ctx, j.FaceID, est.Female); err != nil {
				return err
			}
			if j.GenderOnly { // keep the age it has (from the full photo, perhaps)
				return nil
			}
			return st.SetAge(ctx, j.FaceID, est.Age)
		},
		Log: log,
	}
}

// CropKey names the crop of face i of the image with this content hash.
func CropKey(hash string, i int) string { return hash + "_" + strconv.Itoa(i) }

// EyesStage judges how open each face's eyes are, from its stored crop
// (EyesFromCrop), so photo books can leave out shut eyes.
func EyesStage(e *Engine, st *store.Store, cropPath func(key string) string, workers int, log *slog.Logger) pipeline.Stage[store.EyesJob] {
	return pipeline.Stage[store.EyesJob]{
		Name:    "eyes",
		Workers: workers,
		Next:    st.NeedEyes,
		Key:     func(j store.EyesJob) int64 { return j.FaceID },
		Do: func(ctx context.Context, j store.EyesJob) error {
			crop, err := os.ReadFile(cropPath(CropKey(j.Hash, j.Index)))
			if err != nil { // no crop: can't judge
				return st.SetEyes(ctx, j.FaceID, -1)
			}
			open, ok, err := e.EyesFromCrop(crop, j.CropBox)
			if err != nil {
				return err
			}
			if !ok {
				open = -1
			}
			return st.SetEyes(ctx, j.FaceID, open)
		},
		Log: log,
	}
}
