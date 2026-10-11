package faces

import (
	"os"
	"strings"
	"testing"
)

// TestGender runs the real age-and-gender model on face crops of known
// gender: FACES_MODELS (with mivolo_v2_age_gender.onnx), and FACES_MEN and
// FACES_WOMEN, comma-separated crop files (the face roughly in the middle).
func TestGender(t *testing.T) {
	dir := os.Getenv("FACES_MODELS")
	if dir == "" || os.Getenv("FACES_MEN") == "" {
		t.Skip("FACES_MODELS / FACES_MEN / FACES_WOMEN not set")
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
	if !e.HasGender() {
		t.Fatal("no gender from", cfg.Age)
	}
	box := [4]float32{0.2, 0.15, 0.8, 0.85}
	for _, c := range []struct {
		env    string
		female bool
	}{{"FACES_MEN", false}, {"FACES_WOMEN", true}} {
		right := 0
		files := strings.Split(os.Getenv(c.env), ",")
		for _, p := range files {
			crop, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			est, err := e.EstimateFromCrop(crop, box)
			if err != nil {
				t.Fatal(err)
			}
			if est.Female > 0.5 == c.female {
				right++
			}
			t.Logf("%s: age %.0f female %.2f", c.env, est.Age, est.Female)
		}
		if right*10 < len(files)*8 {
			t.Errorf("%s: %d of %d right", c.env, right, len(files))
		}
	}
}
