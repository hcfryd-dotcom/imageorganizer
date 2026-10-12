//go:build cgo

package faces

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	"math"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// TestAgeMethods compares face-only and full (face+body+mirror) age
// estimates against known ages. FACES_TEST_AGES lists the true ages of the
// faces from left to right, comma-separated.
func TestAgeMethods(t *testing.T) {
	path, dir, lib, ages := os.Getenv("FACES_TEST_IMAGE"), os.Getenv("FACES_MODELS"), os.Getenv("FACES_ORT"), os.Getenv("FACES_TEST_AGES")
	if path == "" || dir == "" || lib == "" || ages == "" {
		t.Skip("FACES_TEST_IMAGE / FACES_MODELS / FACES_ORT / FACES_TEST_AGES not set")
	}
	cfg := DefaultConfig(dir)
	cfg.Library = lib
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	f, _ := os.Open(path)
	src, _, _ := image.Decode(f)
	f.Close()
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)
	dets, err := e.detect(img)
	if err != nil {
		t.Fatal(err)
	}
	slices.SortFunc(dets, func(a, b detection) int { return int(a.box[0] - b.box[0]) })
	var truth []float64
	for _, s := range strings.Split(ages, ",") {
		v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
		truth = append(truth, v)
	}
	if len(truth) != len(dets) {
		t.Fatalf("%d ages given for %d faces", len(truth), len(dets))
	}
	var errFace, errFull float64
	for i, d := range dets {
		a1, _, _ := e.runAge(letterbox(img, d.box, ageSize), nil, false)
		a2, _, _ := e.AgeFull(img, d.box)
		errFace += math.Abs(float64(a1) - truth[i])
		errFull += math.Abs(float64(a2) - truth[i])
		t.Logf("face %d: true %.0f  face-only %.1f  full %.1f", i, truth[i], a1, a2)
	}
	n := float64(len(dets))
	t.Logf("mean absolute error: face-only %.2f years, full %.2f years", errFace/n, errFull/n)
}
