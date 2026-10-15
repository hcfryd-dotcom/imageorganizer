//go:build !cgo

package faces

import (
	"errors"
	"image"
)

// errNoRuntime is why face analysis is off in a build without cgo: the ONNX
// Runtime bindings need a C compiler, so such builds (e.g. a Windows .exe
// cross-compiled without MinGW) run without people.
var errNoRuntime = errors.New("face analysis is not included in this build (built without cgo, so ONNX Runtime cannot be loaded)")

// Engine is a placeholder; Open never returns one in this build.
type Engine struct{}

// Open always fails in a build without cgo.
func Open(c Config) (*Engine, error) { return nil, errNoRuntime }

func (e *Engine) Device() string                          { return "" }
func (e *Engine) Close()                                  {}
func (e *Engine) Analyze(img *image.RGBA) ([]Face, error) { return nil, errNoRuntime }
func (e *Engine) HasGender() bool                         { return false }
func (e *Engine) HasAge() bool                            { return false }
func (e *Engine) HasNSFW() bool                           { return false }
func (e *Engine) HasEyes() bool                           { return false }
func (e *Engine) GenderFromCrop(crop []byte, box [4]float32) (float32, bool, error) {
	return -1, false, errNoRuntime
}
func (e *Engine) EyesFromCrop(crop []byte, box [4]float32) (float32, bool, error) {
	return 0, false, errNoRuntime
}
func (e *Engine) NSFW(img image.Image) (float32, error) { return 0, errNoRuntime }
func (e *Engine) EstimateFull(img *image.RGBA, box [4]float32) (Estimate, error) {
	return Estimate{Female: -1}, errNoRuntime
}
func (e *Engine) AgeFull(img *image.RGBA, box [4]float32) (float32, bool, error) {
	return 0, false, errNoRuntime
}
func (e *Engine) EstimateFromCrop(crop []byte, box [4]float32) (Estimate, error) {
	return Estimate{Female: -1}, errNoRuntime
}
func (e *Engine) AgeFromCrop(crop []byte, box [4]float32) (float32, bool, error) {
	return 0, false, errNoRuntime
}
