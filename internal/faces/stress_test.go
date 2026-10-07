package faces

import (
	"image"
	"image/draw"
	_ "image/jpeg"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestConcurrentGPU hammers the engine from many goroutines, as import and
// age workers do, and reports throughput and this process's VRAM. Set
// FACES_ORT to a GPU build of ONNX Runtime to run it on CUDA.
func TestConcurrentGPU(t *testing.T) {
	path, dir, lib := os.Getenv("FACES_TEST_IMAGE"), os.Getenv("FACES_MODELS"), os.Getenv("FACES_ORT")
	if path == "" || dir == "" || lib == "" {
		t.Skip("FACES_TEST_IMAGE / FACES_MODELS / FACES_ORT not set")
	}
	cfg := DefaultConfig(dir)
	cfg.Library, cfg.Device = lib, "cuda"
	if m, err := strconv.ParseInt(os.Getenv("FACES_GPU_MEM"), 10, 64); err == nil {
		cfg.GPUMemory = m
	}
	e, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	t.Logf("after open: VRAM %s", procVRAM())
	f, _ := os.Open(path)
	src, _, _ := image.Decode(f)
	f.Close()
	img := image.NewRGBA(src.Bounds())
	draw.Draw(img, img.Bounds(), src, src.Bounds().Min, draw.Src)

	if faces, err := e.Analyze(img); err == nil {
		for _, fc := range faces {
			e.AgeFromCrop(fc.Crop, fc.CropBox)
		}
	}
	t.Logf("after one sequential photo: VRAM %s", procVRAM())
	const workers, perWorker = 24, 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error
	start := time.Now()
	for range workers {
		wg.Go(func() {
			for range perWorker {
				faces, err := e.Analyze(img)
				if err == nil {
					for _, fc := range faces {
						if _, _, err = e.AgeFromCrop(fc.Crop, fc.CropBox); err != nil {
							break
						}
					}
				}
				if err != nil {
					mu.Lock()
					errs = append(errs, err)
					mu.Unlock()
				}
			}
		})
	}
	wg.Wait()
	elapsed := time.Since(start)
	vram := procVRAM()
	t.Logf("%d photos (6 faces each, with ages) in %v = %.1f photos/s; VRAM %s; errors %d",
		workers*perWorker, elapsed.Round(time.Millisecond), float64(workers*perWorker)/elapsed.Seconds(), vram, len(errs))
	if len(errs) > 0 {
		t.Fatal(errs[0])
	}
}

func procVRAM() string {
	out, _ := exec.Command("nvidia-smi", "--query-compute-apps=pid,used_memory", "--format=csv,noheader").Output()
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, strconv.Itoa(os.Getpid())+",") {
			return strings.TrimSpace(strings.SplitN(l, ",", 2)[1])
		}
	}
	return "0"
}
