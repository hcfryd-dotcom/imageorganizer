//go:build cgo

package faces

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestEyes runs the landmark model on stored face crops (FACES_CROPS, a
// face crop folder) and writes each crop with its eye points and how open
// the eyes are to FACES_OUT, to look at. FACES_MODELS and FACES_ORT as
// for TestAnalyze.
func TestEyes(t *testing.T) {
	dir, crops, out := os.Getenv("FACES_MODELS"), os.Getenv("FACES_CROPS"), os.Getenv("FACES_OUT")
	if dir == "" || crops == "" || out == "" {
		t.Skip("FACES_MODELS / FACES_CROPS / FACES_OUT not set")
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
	if !e.HasEyes() {
		t.Fatal("no landmark model")
	}
	var files []string
	filepath.WalkDir(crops, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && len(files) < 300 {
			files = append(files, p)
		}
		return nil
	})
	type res struct {
		f    string
		open float32
		img  *image.RGBA
	}
	var all []res
	for _, f := range files {
		data, err := os.ReadFile(f)
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
		// A crop is 1.6 faces wide round the face (cropJPEG).
		W := float32(b.Dx())
		px := [4]float32{W * .1875, W * .1875, W * .8125, W * .8125}
		if px[2]-px[0] < minEyesFace {
			continue
		}
		pts, err := e.landmarks(img, px)
		if err != nil {
			t.Fatal(err)
		}
		open := max(eyeOpen(pts, eyePoints[0]), eyeOpen(pts, eyePoints[1]))
		for k, idx := range eyePoints {
			c := color.RGBA{255, 0, 0, 255}
			if k == 1 {
				c = color.RGBA{0, 255, 0, 255}
			}
			for _, i := range idx {
				x, y := int(pts[i][0]), int(pts[i][1])
				for dx := -1; dx <= 1; dx++ {
					for dy := -1; dy <= 1; dy++ {
						img.Set(x+dx, y+dy, c)
					}
				}
			}
		}
		all = append(all, res{f, open, img})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].open < all[j].open })
	for i, r := range all {
		if i < 12 || i >= len(all)-4 {
			w, _ := os.Create(filepath.Join(out, fmt.Sprintf("%03d_%.3f.png", i, r.open)))
			png.Encode(w, r.img)
			w.Close()
		}
	}
	var hist [10]int
	for _, r := range all {
		hist[min(9, int(r.open*20))]++
	}
	t.Logf("%d faces; openness in steps of 0.05: %v", len(all), hist)
}
