package faces

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAnalyze runs the real models on a group photo. Set FACES_TEST_IMAGE
// to a photo and FACES_MODELS to the model directory to run it.
func TestAnalyze(t *testing.T) {
	path, dir := os.Getenv("FACES_TEST_IMAGE"), os.Getenv("FACES_MODELS")
	if path == "" || dir == "" {
		t.Skip("FACES_TEST_IMAGE / FACES_MODELS not set")
	}
	cfg := DefaultConfig(dir)
	if lib := os.Getenv("FACES_ORT"); lib != "" {
		cfg.Library = lib
	}
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t.Logf("device: %s", e.Device())
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	src, _, err := image.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	rgba := image.NewRGBA(src.Bounds())
	draw.Draw(rgba, rgba.Bounds(), src, src.Bounds().Min, draw.Src)

	start := time.Now()
	faces, err := e.Analyze(rgba)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%s: %d faces in %v", filepath.Base(path), len(faces), time.Since(start))
	for i, fc := range faces {
		age, ok, err := e.AgeFromCrop(fc.Crop, fc.CropBox)
		t.Logf("  face %d: score %.2f age %.1f (ok=%v err=%v) box %.3f", i, fc.Score, age, ok, err, fc.Box)
	}
	if len(faces) == 0 {
		t.Fatal("no faces found")
	}

	// The same people in a half-size copy must match themselves strongly
	// and not each other.
	half := resizeRGBA(rgba, rgba.Bounds().Dx()/2, rgba.Bounds().Dy()/2)
	again, err := e.Analyze(half)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range faces {
		best, bestJ, second := float32(-1), -1, float32(-1)
		for j, b := range again {
			s := dot(a.Embedding, b.Embedding)
			if s > best {
				best, second, bestJ = s, best, j
			} else if s > second {
				second = s
			}
		}
		t.Logf("  face %d -> half-size face %d: same %.2f, next best %.2f", i, bestJ, best, second)
		if best < 0.6 || second > 0.4 {
			t.Errorf("face %d: weak identity separation (same %.2f, other %.2f)", i, best, second)
		}
	}
}

func dot(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}
