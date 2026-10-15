//go:build cgo

package faces

import (
	"bufio"
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"
	"os"
	"strings"
	"testing"
)

// TestGenderAge reads FACES_GENDER_LIST, lines of a face crop's path, a tab
// and F or M, and reports how many the gender model gets right (above
// 0.5 for F, below for M). FACES_MODELS and FACES_ORT as for TestAnalyze.
func TestGenderAge(t *testing.T) {
	dir, list := os.Getenv("FACES_MODELS"), os.Getenv("FACES_GENDER_LIST")
	if dir == "" || list == "" {
		t.Skip("FACES_MODELS / FACES_GENDER_LIST not set")
	}
	cfg := DefaultConfig(dir)
	cfg.Device = "cpu"
	if lib := os.Getenv("FACES_ORT"); lib != "" {
		cfg.Library = lib
	}
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if e.ga == nil {
		t.Fatal("no gender model")
	}
	f, err := os.Open(list)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	right, n := map[string]int{}, map[string]int{}
	sum := map[string]float32{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.Split(strings.TrimPrefix(sc.Text(), "\ufeff"), "\t")
		if len(parts) < 2 {
			continue
		}
		data, err := os.ReadFile(parts[0])
		if err != nil {
			continue
		}
		src, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			continue
		}
		b := src.Bounds()
		img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
		W := float32(b.Dx()) // a crop is 1.6 faces wide round the face
		fem, err := e.femaleFrom(img, [4]float32{W * .1875, W * .1875, W * .8125, W * .8125})
		if err != nil {
			t.Fatal(err)
		}
		g := parts[1]
		sum[g] += fem
		n[g]++
		if (fem > .5) == (g == "F") {
			right[g]++
		}
	}
	t.Logf("women %d of %d right (mean %.2f), men %d of %d right (mean %.2f)", right["F"], n["F"], sum["F"]/float32(max(1, n["F"])), right["M"], n["M"], sum["M"]/float32(max(1, n["M"])))
}
