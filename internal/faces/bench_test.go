package faces

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	"os"
	"testing"
)

// BenchmarkStages times each model separately on FACES_TEST_IMAGE.
func BenchmarkStages(b *testing.B) {
	path, dir := os.Getenv("FACES_TEST_IMAGE"), os.Getenv("FACES_MODELS")
	if path == "" || dir == "" {
		b.Skip("FACES_TEST_IMAGE / FACES_MODELS not set")
	}
	e, err := Open(DefaultConfig(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer e.Close()
	f, _ := os.Open(path)
	src, _, _ := image.Decode(f)
	f.Close()
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	dets, err := e.detect(img)
	if err != nil || len(dets) == 0 {
		b.Fatal("no faces", err)
	}
	b.Run("detect", func(b *testing.B) {
		for b.Loop() {
			e.detect(img)
		}
	})
	b.Run("embed-per-face", func(b *testing.B) {
		for b.Loop() {
			e.embed(img, dets[0].kps)
		}
	})
	b.Run("age-per-face", func(b *testing.B) {
		for b.Loop() {
			e.estimateAge(img, dets[0].box)
		}
	})
}
