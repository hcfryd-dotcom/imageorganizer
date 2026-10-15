// Package faces finds faces in photos and describes them: where they are,
// an identity embedding (ArcFace) and an apparent age (MiVOLO). The age is
// what lets look-alike siblings be told apart: capture date minus apparent
// age gives an implied birth year, which differs between siblings even when
// their faces at the same age are nearly identical.
//
// Models run in-process with ONNX Runtime (loaded at runtime from the system
// library). Model files are configurable; the defaults are InsightFace's
// buffalo_l (SCRFD-10G detector, ArcFace R50) and MiVOLO v2 for age.
//go:build cgo

package faces

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

// Engine runs the models. Safe for concurrent use.
type Engine struct {
	det, rec, age *ort.DynamicAdvancedSession
	nsfw          *ort.DynamicAdvancedSession
	lmk           *ort.DynamicAdvancedSession // 106 landmarks, for eyes (eyes.go)
	ga            *ort.DynamicAdvancedSession // gender, when the age model has none (genderage.go)
	gender        bool                        // the age model also gives gender (its "head" output)
	device        string                      // "cuda" or "cpu", as actually used
	// gpu feeds model runs to a few dedicated OS threads. ONNX Runtime's
	// CUDA provider keeps cuBLAS/cuDNN handles and workspaces per calling
	// OS thread; Go would spread runs over dozens of threads and VRAM grew
	// from 2 GB to 7 GB that way. Runs take milliseconds, so two threads
	// keep the GPU busy.
	gpu chan gpuJob
}

type gpuJob struct {
	s       *ort.DynamicAdvancedSession
	in, out []ort.Value
	done    chan error
}

// gpuThreads is how many OS threads issue GPU work.
const gpuThreads = 2

func (e *Engine) startGPUThreads() {
	e.gpu = make(chan gpuJob)
	for range gpuThreads {
		go func() {
			runtime.LockOSThread() // one stable thread = one set of CUDA handles
			for j := range e.gpu {
				j.done <- j.s.Run(j.in, j.out)
			}
		}()
	}
}

// run executes a session, on a dedicated GPU thread when on CUDA.
func (e *Engine) run(s *ort.DynamicAdvancedSession, in, out []ort.Value) error {
	if e.gpu == nil {
		return s.Run(in, out)
	}
	done := make(chan error, 1)
	e.gpu <- gpuJob{s, in, out, done}
	return <-done
}

// Device reports where the models run.
func (e *Engine) Device() string { return e.device }

var initOnce sync.Once
var initErr error

// Open loads the runtime and models.
func Open(c Config) (*Engine, error) {
	for _, p := range []string{c.Library, c.Detector, c.Recognize} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("face analysis unavailable: %w", err)
		}
	}
	initOnce.Do(func() {
		ort.SetSharedLibraryPath(c.Library)
		initErr = ort.InitializeEnvironment()
	})
	if initErr != nil {
		return nil, initErr
	}
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, err
	}
	defer opts.Destroy()
	if err := opts.SetIntraOpNumThreads(max(1, c.Threads)); err != nil {
		return nil, err
	}
	if err := opts.SetInterOpNumThreads(1); err != nil {
		return nil, err
	}
	// Many runs execute concurrently (one per import or age worker), so
	// threads spinning while they wait for work only steal CPU from the
	// other runs.
	if err := opts.AddSessionConfigEntry("session.intra_op.allow_spinning", "0"); err != nil {
		return nil, err
	}

	e := &Engine{device: "cpu"}
	if c.Device == "cuda" || c.Device == "auto" {
		if err := appendCUDA(opts, c); err != nil {
			if c.Device == "cuda" {
				return nil, fmt.Errorf("cuda: %w", err)
			}
			// auto: fall back to CPU, already configured.
		} else {
			e.device = "cuda"
			e.startGPUThreads()
		}
	}
	open := func(path string) (*ort.DynamicAdvancedSession, error) {
		in, out, err := ort.GetInputOutputInfo(path)
		if err != nil {
			return nil, err
		}
		names := func(infos []ort.InputOutputInfo) []string {
			var n []string
			for _, i := range infos {
				n = append(n, i.Name)
			}
			return n
		}
		return ort.NewDynamicAdvancedSession(path, names(in), names(out), opts)
	}
	if e.det, err = open(c.Detector); err != nil {
		return nil, fmt.Errorf("detector: %w", err)
	}
	if e.rec, err = open(c.Recognize); err != nil {
		e.Close()
		return nil, fmt.Errorf("recognizer: %w", err)
	}
	if c.Age != "" {
		if _, err := os.Stat(c.Age); err == nil {
			if e.age, err = open(c.Age); err != nil {
				e.Close()
				return nil, fmt.Errorf("age model: %w", err)
			}
			if _, out, err := ort.GetInputOutputInfo(c.Age); err == nil && len(out) == 2 && out[1].Name == "head" {
				e.gender = true
			}
		}
	}
	if c.NSFW != "" && fileExists(c.NSFW) {
		if e.nsfw, err = open(c.NSFW); err != nil {
			e.Close()
			return nil, fmt.Errorf("nsfw model: %w", err)
		}
	}
	// The gender model is only wanted when the age model gives no gender.
	if c.GenderAge != "" && fileExists(c.GenderAge) && e.age != nil && !e.gender {
		if e.ga, err = open(c.GenderAge); err != nil {
			e.Close()
			return nil, fmt.Errorf("gender model: %w", err)
		}
	}
	if c.Landmarks != "" && fileExists(c.Landmarks) {
		if e.lmk, err = open(c.Landmarks); err != nil {
			e.Close()
			return nil, fmt.Errorf("landmark model: %w", err)
		}
	}
	return e, nil
}

func appendCUDA(opts *ort.SessionOptions, c Config) error {
	co, err := ort.NewCUDAProviderOptions()
	if err != nil {
		return err
	}
	defer co.Destroy()
	mem := c.GPUMemory
	if mem <= 0 {
		mem = 1 << 30
	}
	if err := co.Update(map[string]string{
		"device_id":     strconv.Itoa(c.CUDADevice),
		"gpu_mem_limit": strconv.FormatInt(mem, 10),
		// Grow the memory arena only as needed, so VRAM use stays small.
		"arena_extend_strategy": "kSameAsRequested",
		// Detection input sizes vary per photo; exhaustive cuDNN
		// algorithm search per shape would cost more than it saves.
		"cudnn_conv_algo_search": "HEURISTIC",
		// cuDNN may otherwise grab its largest workspace per convolution,
		// which multiplied VRAM use several times for no real gain here.
		"cudnn_conv_use_max_workspace": "0",
	}); err != nil {
		return err
	}
	return opts.AppendExecutionProviderCUDA(co)
}

func (e *Engine) Close() {
	if e.gpu != nil {
		close(e.gpu)
	}
	for _, s := range []*ort.DynamicAdvancedSession{e.det, e.rec, e.age, e.nsfw, e.lmk, e.ga} {
		if s != nil {
			s.Destroy()
		}
	}
}

// Tunables.
const (
	detLongEdge   = 1024 // detector input size (long edge)
	minScore      = 0.5
	nmsIoU        = 0.4
	minFacePixels = 20 // ignore faces smaller than this in the work image
	minAgeFace    = 40 // only estimate age on faces at least this big
	cropSize      = 256
)

// Analyze finds and describes all faces in an upright image. Larger work
// images (around 2000 px) give better embeddings for small faces.
func (e *Engine) Analyze(img *image.RGBA) ([]Face, error) {
	dets, err := e.detect(img)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	W, H := float32(b.Dx()), float32(b.Dy())
	out := make([]Face, 0, len(dets))
	for _, d := range dets {
		if d.box[2]-d.box[0] < minFacePixels || d.box[3]-d.box[1] < minFacePixels {
			continue
		}
		emb, err := e.embed(img, d.kps)
		if err != nil {
			return nil, err
		}
		f := Face{Score: d.score, Embedding: emb}
		f.Box = [4]float32{d.box[0] / W, d.box[1] / H, d.box[2] / W, d.box[3] / H}
		for i, p := range d.kps {
			f.Landmarks[i] = [2]float32{p[0] / W, p[1] / H}
		}
		f.Crop, f.CropBox = cropJPEG(img, d.box)
		// On a GPU the full age estimate (face, body, mirrored) is cheap
		// enough to do here, while the full photo is at hand; on a CPU the
		// age stage estimates it later from the stored crop instead.
		if e.age != nil && e.device == "cuda" {
			est, err := e.EstimateFull(img, d.box)
			if err != nil {
				return nil, err
			}
			f.Age, f.Female, f.AgeDone = -1, -1, true
			if est.OK {
				f.Age, f.Female = est.Age, est.Female
			}
		}
		out = append(out, f)
	}
	return out, nil
}

type detection struct {
	box   [4]float32 // pixels in the work image
	kps   [5][2]float32
	score float32
}

// detect runs SCRFD. The image is scaled so its long edge is detLongEdge,
// padded to multiples of 32 (top-left aligned).
func (e *Engine) detect(img *image.RGBA) ([]detection, error) {
	b := img.Bounds()
	scale := float32(detLongEdge) / float32(max(b.Dx(), b.Dy()))
	if scale > 1 {
		scale = 1
	}
	w, h := int(float32(b.Dx())*scale), int(float32(b.Dy())*scale)
	pw, ph := (w+31)/32*32, (h+31)/32*32
	small := resizeRGBA(img, w, h)

	// RGB, (p - 127.5) / 128, NCHW; padding is zero after normalisation.
	in := make([]float32, 3*pw*ph)
	plane := pw * ph
	for y := range h {
		for x := range w {
			i := small.PixOffset(x, y)
			o := y*pw + x
			in[o] = (float32(small.Pix[i]) - 127.5) / 128
			in[plane+o] = (float32(small.Pix[i+1]) - 127.5) / 128
			in[2*plane+o] = (float32(small.Pix[i+2]) - 127.5) / 128
		}
	}
	t, err := ort.NewTensor(ort.NewShape(1, 3, int64(ph), int64(pw)), in)
	if err != nil {
		return nil, err
	}
	defer t.Destroy()
	outs := make([]ort.Value, 9)
	if err := e.run(e.det, []ort.Value{t}, outs); err != nil {
		return nil, err
	}
	defer func() {
		for _, o := range outs {
			if o != nil {
				o.Destroy()
			}
		}
	}()

	var dets []detection
	for si, stride := range []int{8, 16, 32} {
		scores := outs[si].(*ort.Tensor[float32]).GetData()
		boxes := outs[si+3].(*ort.Tensor[float32]).GetData()
		kps := outs[si+6].(*ort.Tensor[float32]).GetData()
		gw := pw / stride
		for i, s := range scores {
			if s < minScore {
				continue
			}
			cell := i / 2 // two anchors per grid cell
			cx, cy := float32(cell%gw*stride), float32(cell/gw*stride)
			st := float32(stride)
			var d detection
			d.score = s
			d.box = [4]float32{
				(cx - boxes[4*i]*st) / scale, (cy - boxes[4*i+1]*st) / scale,
				(cx + boxes[4*i+2]*st) / scale, (cy + boxes[4*i+3]*st) / scale,
			}
			for k := range 5 {
				d.kps[k] = [2]float32{(cx + kps[10*i+2*k]*st) / scale, (cy + kps[10*i+2*k+1]*st) / scale}
			}
			dets = append(dets, d)
		}
	}
	return nms(dets, nmsIoU), nil
}

func nms(d []detection, thr float32) []detection {
	slices.SortFunc(d, func(a, b detection) int {
		switch {
		case a.score > b.score:
			return -1
		case a.score < b.score:
			return 1
		}
		return 0
	})
	var keep []detection
	for _, c := range d {
		ok := true
		for _, k := range keep {
			if iou(c.box, k.box) > thr {
				ok = false
				break
			}
		}
		if ok {
			keep = append(keep, c)
		}
	}
	return keep
}

func iou(a, b [4]float32) float32 {
	x1, y1 := max(a[0], b[0]), max(a[1], b[1])
	x2, y2 := min(a[2], b[2]), min(a[3], b[3])
	inter := max(0, x2-x1) * max(0, y2-y1)
	ua := (a[2]-a[0])*(a[3]-a[1]) + (b[2]-b[0])*(b[3]-b[1]) - inter
	if ua <= 0 {
		return 0
	}
	return inter / ua
}

// arcfaceTemplate is where ArcFace expects the 5 landmarks in its 112×112 input.
var arcfaceTemplate = [5][2]float32{
	{38.2946, 51.6963}, {73.5318, 51.5014}, {56.0252, 71.7366}, {41.5493, 92.3655}, {70.7299, 92.2041},
}

// embed aligns the face to ArcFace's template and returns its unit embedding.
func (e *Engine) embed(img *image.RGBA, kps [5][2]float32) ([]float32, error) {
	m := similarity(kps, arcfaceTemplate)
	crop := warp(img, m, 112, 112)
	in := make([]float32, 3*112*112)
	plane := 112 * 112
	for y := range 112 {
		for x := range 112 {
			i := crop.PixOffset(x, y)
			o := y*112 + x
			in[o] = (float32(crop.Pix[i]) - 127.5) / 127.5
			in[plane+o] = (float32(crop.Pix[i+1]) - 127.5) / 127.5
			in[2*plane+o] = (float32(crop.Pix[i+2]) - 127.5) / 127.5
		}
	}
	t, err := ort.NewTensor(ort.NewShape(1, 3, 112, 112), in)
	if err != nil {
		return nil, err
	}
	defer t.Destroy()
	outs := []ort.Value{nil}
	if err := e.run(e.rec, []ort.Value{t}, outs); err != nil {
		return nil, err
	}
	defer outs[0].Destroy()
	v := slices.Clone(outs[0].(*ort.Tensor[float32]).GetData())
	var n float64
	for _, x := range v {
		n += float64(x) * float64(x)
	}
	if n == 0 {
		return nil, errors.New("zero embedding")
	}
	inv := float32(1 / math.Sqrt(n))
	for i := range v {
		v[i] *= inv
	}
	return v, nil
}

// estimateAge runs MiVOLO on the face crop (no body crop, which MiVOLO
// supports by passing a black image).
func (e *Engine) estimateAge(img *image.RGBA, box [4]float32) (float32, error) {
	a, _, err := e.runAge(letterbox(img, box, ageSize), nil, false)
	return a, err
}

// HasGender reports whether the age model also estimates gender.
func (e *Engine) HasGender() bool { return e.age != nil && (e.gender || e.ga != nil) }

// ageSize is MiVOLO's input size for each of the face and body crops.
const ageSize = 384

// AgeFull estimates apparent age from the full photo the way MiVOLO is
// meant to be used: the face together with the body, averaged with the
// mirrored image. The body is approximated from the face box (no person
// detector): a region about three faces wide and seven tall, starting just
// above the face. ok is false when the face is too small to judge.
func (e *Engine) AgeFull(img *image.RGBA, box [4]float32) (age float32, ok bool, err error) {
	est, err := e.EstimateFull(img, box)
	return est.Age, est.OK, err
}

// EstimateFull is AgeFull with gender.
func (e *Engine) EstimateFull(img *image.RGBA, box [4]float32) (Estimate, error) {
	if e.age == nil || box[2]-box[0] < minAgeFace {
		return Estimate{Female: -1}, nil
	}
	face := letterbox(img, box, ageSize)
	var body *image.RGBA
	w, h := box[2]-box[0], box[3]-box[1]
	cx := (box[0] + box[2]) / 2
	bb := [4]float32{cx - 1.6*w, box[1] - 0.3*h, cx + 1.6*w, box[1] + 6.5*h}
	b := img.Bounds()
	bb = [4]float32{max(bb[0], float32(b.Min.X)), max(bb[1], float32(b.Min.Y)), min(bb[2], float32(b.Max.X)), min(bb[3], float32(b.Max.Y))}
	// A close-up has no usable body; MiVOLO handles "no body" natively.
	if (bb[2]-bb[0])*(bb[3]-bb[1]) >= 2.5*w*h {
		body = letterbox(img, bb, ageSize)
	}
	a1, f1, err := e.runAge(face, body, false)
	if err != nil {
		return Estimate{}, err
	}
	a2, f2, err := e.runAge(face, body, true)
	if err != nil {
		return Estimate{}, err
	}
	fem := float32(-1)
	if f1 >= 0 && f2 >= 0 {
		fem = (f1 + f2) / 2
	} else if e.ga != nil {
		if fem, err = e.femaleFrom(img, box); err != nil {
			return Estimate{}, err
		}
	}
	return Estimate{Age: (a1 + a2) / 2, Female: fem, OK: true}, nil
}

// runAge runs MiVOLO on a letterboxed face and optional body crop (nil:
// black, MiVOLO's "no body"), optionally mirrored. female is the
// probability of a woman's or girl's face, -1 without a gender model.
func (e *Engine) runAge(face, body *image.RGBA, flip bool) (age, female float32, err error) {
	const S = ageSize
	mean := [3]float32{0.485, 0.456, 0.406}
	std := [3]float32{0.229, 0.224, 0.225}
	in := make([]float32, 6*S*S)
	plane := S * S
	fill := func(img *image.RGBA, first int) {
		for c := range 3 {
			if img == nil {
				black := (0 - mean[c]) / std[c]
				for i := range plane {
					in[(first+c)*plane+i] = black
				}
			}
		}
		if img == nil {
			return
		}
		for y := range S {
			for x := range S {
				sx := x
				if flip {
					sx = S - 1 - x
				}
				i := img.PixOffset(sx, y)
				o := y*S + x
				for c := range 3 {
					in[(first+c)*plane+o] = (float32(img.Pix[i+c])/255 - mean[c]) / std[c]
				}
			}
		}
	}
	fill(face, 0)
	fill(body, 3)
	t, err := ort.NewTensor(ort.NewShape(1, 6, S, S), in)
	if err != nil {
		return 0, 0, err
	}
	defer t.Destroy()
	outs := []ort.Value{nil}
	if e.gender {
		outs = append(outs, nil)
	}
	if err := e.run(e.age, []ort.Value{t}, outs); err != nil {
		return 0, 0, err
	}
	for _, o := range outs {
		defer o.Destroy()
	}
	age = outs[0].(*ort.Tensor[float32]).GetData()[0]
	female = -1
	if e.gender {
		// The head: male and female logits, then the raw age.
		h := outs[1].(*ort.Tensor[float32]).GetData()
		female = float32(1 / (1 + math.Exp(float64(h[0]-h[1]))))
	}
	return age, female, nil
}

// cropJPEG cuts a square around the face with some margin, like a contact
// photo, and reports where the face box lies inside it (0..1).
func cropJPEG(img *image.RGBA, box [4]float32) ([]byte, [4]float32) {
	cx, cy := (box[0]+box[2])/2, (box[1]+box[3])/2
	half := max(box[2]-box[0], box[3]-box[1]) * 0.8
	r := image.Rect(int(cx-half), int(cy-half), int(cx+half), int(cy+half)).Intersect(img.Bounds())
	if r.Empty() {
		return nil, [4]float32{}
	}
	w, h := float32(r.Dx()), float32(r.Dy())
	inCrop := [4]float32{
		(box[0] - float32(r.Min.X)) / w, (box[1] - float32(r.Min.Y)) / h,
		(box[2] - float32(r.Min.X)) / w, (box[3] - float32(r.Min.Y)) / h,
	}
	for i := range inCrop {
		inCrop[i] = min(1, max(0, inCrop[i]))
	}
	side := min(cropSize, max(r.Dx(), r.Dy())) // never upscale
	small := resizeRGBA(img.SubImage(r).(*image.RGBA), side*r.Dx()/max(r.Dx(), r.Dy()), side*r.Dy()/max(r.Dx(), r.Dy()))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: 88}); err != nil {
		return nil, [4]float32{}
	}
	return buf.Bytes(), inCrop
}

// HasAge reports whether an age model is loaded.
func (e *Engine) HasAge() bool { return e.age != nil }

// AgeFromCrop estimates the apparent age of the face at box (0..1) inside a
// stored face crop. ok is false when the face is too small to judge.
func (e *Engine) AgeFromCrop(crop []byte, box [4]float32) (age float32, ok bool, err error) {
	est, err := e.EstimateFromCrop(crop, box)
	return est.Age, est.OK, err
}

// EstimateFromCrop is AgeFromCrop with gender.
func (e *Engine) EstimateFromCrop(crop []byte, box [4]float32) (Estimate, error) {
	if e.age == nil {
		return Estimate{}, errors.New("no age model")
	}
	src, err := jpeg.Decode(bytes.NewReader(crop))
	if err != nil {
		return Estimate{}, err
	}
	b := src.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(img, img.Bounds(), src, b.Min, draw.Src)
	W, H := float32(b.Dx()), float32(b.Dy())
	px := [4]float32{box[0] * W, box[1] * H, box[2] * W, box[3] * H}
	if px[2]-px[0] < minAgeFace {
		return Estimate{Female: -1}, nil
	}
	a, fem, err := e.runAge(letterbox(img, px, ageSize), nil, false)
	if err == nil && fem < 0 && e.ga != nil {
		fem, err = e.femaleFrom(img, px)
	}
	return Estimate{Age: a, Female: fem, OK: err == nil}, err
}
