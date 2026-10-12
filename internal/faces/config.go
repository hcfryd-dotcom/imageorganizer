package faces

import (
	"os"
	"path/filepath"
)

// Config locates the runtime and models.
type Config struct {
	Library   string // libonnxruntime.so
	Detector  string // SCRFD .onnx
	Recognize string // ArcFace .onnx (112×112 aligned input, 512-d output)
	Age       string // MiVOLO v2 .onnx (age, and gender when it has a "head" output); empty disables age estimation
	NSFW      string // an NSFW image classifier .onnx (ViT, 224×224, logits normal/nsfw); empty or missing disables it
	Threads   int    // intra-op threads per model run (CPU)
	// Device is "cpu", "cuda", or "auto" (CUDA if the runtime and a GPU
	// are available, else CPU). CUDA needs an ONNX Runtime GPU build.
	Device     string
	GPUMemory  int64 // CUDA memory cap in bytes per model (0 = 2 GiB)
	CUDADevice int
}

// DefaultConfig looks for models in dir.
func DefaultConfig(dir string) Config {
	return Config{
		Library:   "/usr/lib/libonnxruntime.so",
		Detector:  filepath.Join(dir, "scrfd_10g.onnx"),
		Recognize: filepath.Join(dir, "arcface_w600k_r50.onnx"),
		Age:       ageModel(dir),
		NSFW:      filepath.Join(dir, "nsfw_vit.onnx"),
		Threads:   1, // parallelism comes from concurrent workers
		Device:    "auto",
	}
}

// ageModel prefers the MiVOLO export that also gives gender (the same
// network with its head output exposed), else the age-only one.
func ageModel(dir string) string {
	if p := filepath.Join(dir, "mivolo_v2_age_gender.onnx"); fileExists(p) {
		return p
	}
	return filepath.Join(dir, "mivolo_v2_age.onnx")
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// Face is one detected face in image coordinates of the upright photo,
// normalised to 0..1 so it is independent of the analysed resolution.
type Face struct {
	Box       [4]float32    // x1, y1, x2, y2 (0..1)
	Landmarks [5][2]float32 // eyes, nose, mouth corners (0..1)
	Score     float32       // detection confidence
	Embedding []float32     // unit-length identity vector
	Crop      []byte        // JPEG of the face for the UI and later age estimation
	CropBox   [4]float32    // the face's box inside Crop, 0..1
	// Age is the apparent age when estimated during analysis (AgeDone),
	// -1 if the face is too small to judge; Female the estimated
	// probability that the face is a woman's or girl's (-1: unknown).
	Age     float32
	Female  float32
	AgeDone bool
}

// Estimate is what MiVOLO tells of a face: apparent age, and the
// probability that it is a woman's or girl's (-1 without a gender model).
// OK is false when the face is too small to judge.
type Estimate struct {
	Age, Female float32
	OK          bool
}
