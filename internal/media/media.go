// Package media extracts metadata from image files and renders thumbnails,
// entirely in-process.
package media

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta"
	"github.com/evanoberholster/imagemeta/meta/exif"
	_ "github.com/gen2brain/avif" // registers with image.Decode
	_ "github.com/gen2brain/heic" // registers with image.Decode
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

func init() {
	imagemeta.SetLogger(io.Discard, slog.LevelError)
}

type kind int

const (
	kindDecodable kind = iota // Go can decode the file directly
	kindTIFFRaw               // TIFF-based RAW: use the embedded JPEG preview
	kindCR3                   // Canon CR3: embedded JPEG preview
	kindVideo                 // a video: through ffmpeg (video.go)
)

var extensions = map[string]kind{
	".jpg": kindDecodable, ".jpeg": kindDecodable, ".jpe": kindDecodable,
	".png": kindDecodable, ".gif": kindDecodable, ".webp": kindDecodable, ".bmp": kindDecodable,
	".tif": kindDecodable, ".tiff": kindDecodable,
	".heic": kindDecodable, ".heif": kindDecodable, ".avif": kindDecodable,
	".cr2": kindTIFFRaw, ".nef": kindTIFFRaw, ".arw": kindTIFFRaw, ".dng": kindTIFFRaw,
	".orf": kindTIFFRaw, ".rw2": kindTIFFRaw, ".pef": kindTIFFRaw, ".srw": kindTIFFRaw,
	".cr3": kindCR3,
}

// Supported reports whether path has an image extension we index.
func Supported(path string) bool {
	_, ok := extensions[strings.ToLower(filepath.Ext(path))]
	return ok
}

// Info is the metadata extracted from one file.
type Info struct {
	Format      string
	Width       int
	Height      int
	TakenAt     *time.Time
	Lat, Lon    *float64
	CameraMake  string
	CameraModel string
	Lens        string
	ISO         int
	FNumber     float64
	Exposure    string
	Focal       float64
	Orientation int
	Duration    float32 // seconds, for a video
}

// Result of processing a file: metadata plus rendered thumbnail.
type Result struct {
	Info  Info
	Thumb []byte // JPEG, nil if the image could not be decoded
	PHash uint64
	// Work is an upright copy with a long edge of at most WorkSize, for
	// analysis (faces) while the photo is decoded anyway. Only set when
	// Process is asked for it.
	Work *image.RGBA
}

// ThumbSize is the long edge of generated thumbnails in pixels.
const ThumbSize = 512

// WorkSize is the long edge of the analysis copy.
const WorkSize = 2048

// Process extracts metadata and renders a thumbnail from the file contents.
// Decoding errors are not fatal: metadata is still returned so the file is
// indexed, just without a thumbnail.
//
// rot is an extra clockwise rotation (0, 90, 180, 270) on top of the EXIF
// orientation: the user's (or the face detector's) correction for photos
// whose camera recorded the wrong one.
func Process(path string, data []byte, wantWork bool, rot int) (Result, error) {
	var res Result
	k := extensions[strings.ToLower(filepath.Ext(path))]
	res.Info.Format = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if k == kindVideo {
		return res, fmt.Errorf("%s: a video (ProcessVideo)", path)
	}

	if e, err := imagemeta.Decode(bytes.NewReader(data)); err == nil {
		res.Info.fromExif(e)
	}

	img, err := decode(k, data)
	if err != nil {
		return res, fmt.Errorf("decode %s: %w", path, err)
	}
	// Decoded pixel size is authoritative unless we only decoded a RAW's
	// embedded preview, which may be smaller than the sensor image.
	if b := img.Bounds(); k == kindDecodable || res.Info.Width == 0 {
		res.Info.Width, res.Info.Height = b.Dx(), b.Dy()
	}
	switch res.Info.Format {
	case "heic", "heif", "avif":
		// The decoder already applies the container's rotation (irot/imir).
		res.Info.Orientation = 0
	}

	var thumb *image.RGBA
	if wantWork {
		// One resize to the analysis size; the thumbnail is made from it.
		res.Work = RotateCW(Orient(Resize(img, WorkSize), res.Info.Orientation), rot)
		thumb = Resize(res.Work, ThumbSize)
	} else {
		thumb = RotateCW(Orient(Resize(img, ThumbSize), res.Info.Orientation), rot)
	}
	if res.Info.Orientation >= 5 { // rotated 90°: report display dimensions
		res.Info.Width, res.Info.Height = res.Info.Height, res.Info.Width
	}
	if rot == 90 || rot == 270 {
		res.Info.Width, res.Info.Height = res.Info.Height, res.Info.Width
	}
	if res.Thumb, err = encodeJPEG(thumb, 82); err != nil {
		return res, err
	}
	res.PHash = DHash(thumb)
	return res, nil
}

// Thumbnail renders the thumbnail and perceptual hash of an upright
// picture (e.g. a Work copy after a rotation correction).
func Thumbnail(img *image.RGBA) ([]byte, uint64, error) {
	thumb := Resize(img, ThumbSize)
	data, err := encodeJPEG(thumb, 82)
	return data, DHash(thumb), err
}

// Render decodes the file and returns an upright JPEG with the given long
// edge, for formats browsers cannot display (HEIC, TIFF, RAW) and for
// photos with a rotation correction (rot, clockwise degrees).
func Render(path string, data []byte, size, rot int) ([]byte, error) {
	format := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	img, err := decode(extensions["."+format], data)
	if err != nil {
		return nil, err
	}
	orientation := 0
	if format != "heic" && format != "heif" && format != "avif" {
		if e, err := imagemeta.Decode(bytes.NewReader(data)); err == nil {
			var in Info
			in.fromExif(e)
			orientation = in.Orientation
		}
	}
	return encodeJPEG(RotateCW(Orient(Resize(img, size), orientation), rot), 88)
}

// BrowserNative reports whether browsers can display the format directly.
func BrowserNative(format string) bool {
	switch format {
	case "jpg", "jpeg", "jpe", "png", "gif", "webp", "avif", "bmp":
		return true
	}
	return false
}

func encodeJPEG(img image.Image, q int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// heic/avif decoders bundle a WASM runtime and are not safe for unbounded
// concurrency memory-wise; the scanner limits workers anyway.
func decode(k kind, data []byte) (image.Image, error) {
	switch k {
	case kindTIFFRaw:
		p, err := imagemeta.PreviewTIFF(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return jpeg.Decode(bytes.NewReader(p))
	case kindCR3:
		p, err := imagemeta.PreviewCR3(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		return jpeg.Decode(bytes.NewReader(p))
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if img == nil {
		return nil, errors.New("empty image")
	}
	return img, nil
}

func (in *Info) fromExif(e exif.Exif) {
	if t := e.SelectedDate(); !t.IsZero() && t.Year() > 1900 {
		in.TakenAt = &t
	}
	lat, lon := e.GPS.Latitude(), e.GPS.Longitude()
	if (lat != 0 || lon != 0) && math.Abs(lat) <= 90 && math.Abs(lon) <= 180 {
		in.Lat, in.Lon = &lat, &lon
	}
	in.CameraMake = strings.TrimSpace(e.IFD0.Make)
	in.CameraModel = strings.TrimSpace(e.IFD0.Model)
	in.Lens = strings.TrimSpace(e.ExifIFD.LensModel)
	in.ISO = int(e.ExifIFD.ISOSpeedRatings)
	in.FNumber = float64(e.ExifIFD.FNumber)
	if e.ExifIFD.ExposureTime > 0 {
		in.Exposure = e.ExifIFD.ExposureTime.String()
	}
	in.Focal = float64(e.ExifIFD.FocalLength)
	in.Width, in.Height = int(e.ExifIFD.PixelXDimension), int(e.ExifIFD.PixelYDimension)
	if o := int(e.IFD0.Orientation); o >= 1 && o <= 8 && o != int(meta.OrientationHorizontal) {
		in.Orientation = o
	}
}
