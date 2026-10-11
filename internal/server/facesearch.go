package server

import (
	"cmp"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/lkarlslund/imageorganizer/internal/faces"
	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// maxSearchUpload bounds a picture sent to find a face.
const maxSearchUpload = 40 << 20

type foundFace struct {
	Box     [4]float32           `json:"box"`
	Crop    string               `json:"crop"` // data: URL of the face
	Age     float32              `json:"age"`  // -1 unknown
	Score   float32              `json:"score"`
	Matches []store.SimilarGroup `json:"matches"`
}

// faceSearch finds the faces in an uploaded picture and, for each, the
// library's most alike faces. The picture is only looked at, in memory:
// it is not kept, and not added to the library.
func (s *Server) faceSearch(w http.ResponseWriter, r *http.Request) {
	if s.Faces == nil {
		httpError(w, http.StatusServiceUnavailable, fmt.Errorf("face analysis isn’t available (no face models)"))
		return
	}
	name, data, err := readUpload(r)
	if err != nil {
		httpError(w, 400, err)
		return
	}
	res, err := media.Process(name, data, true, 0)
	if err != nil || res.Work == nil {
		httpError(w, 400, fmt.Errorf("can’t read that picture"))
		return
	}
	found, err := s.Faces.Analyze(res.Work)
	if err != nil {
		httpError(w, 500, err)
		return
	}
	// Biggest first: usually the face meant.
	area := func(f faces.Face) float32 { return (f.Box[2] - f.Box[0]) * (f.Box[3] - f.Box[1]) }
	slices.SortFunc(found, func(a, b faces.Face) int { return cmp.Compare(area(b), area(a)) })
	W, H := float32(res.Work.Bounds().Dx()), float32(res.Work.Bounds().Dy())
	found = found[:min(len(found), 12)]
	out := make([]foundFace, 0, len(found))
	for _, f := range found {
		age := f.Age
		if !f.AgeDone {
			age = -1
			if s.Faces.HasAge() {
				px := [4]float32{f.Box[0] * W, f.Box[1] * H, f.Box[2] * W, f.Box[3] * H}
				if a, ok, err := s.Faces.AgeFull(res.Work, px); err == nil && ok {
					age = a
				}
			}
		}
		out = append(out, foundFace{
			Box: f.Box, Age: age, Score: f.Score,
			Crop:    "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(f.Crop),
			Matches: s.Store.SimilarFaces(r.Context(), f.Embedding, age, 12),
		})
	}
	writeJSON(w, map[string]any{"faces": out})
}

// readUpload reads the picture from a multipart form (field "file") or the
// raw request body, with a file name whose extension says how to decode it.
func readUpload(r *http.Request) (string, []byte, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxSearchUpload)
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	var name string
	var src io.Reader = r.Body
	if ct == "multipart/form-data" {
		f, h, err := r.FormFile("file")
		if err != nil {
			return "", nil, err
		}
		defer f.Close()
		name, src = h.Filename, f
		if hct := h.Header.Get("Content-Type"); !media.Supported(name) && hct != "" {
			ct = hct
		}
	}
	if !media.Supported(name) {
		ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp", "image/gif": ".gif",
			"image/heic": ".heic", "image/heif": ".heif", "image/avif": ".avif", "image/bmp": ".bmp", "image/tiff": ".tif"}[ct]
		if ext == "" {
			ext = ".jpg"
		}
		name = "upload" + ext
	}
	data, err := io.ReadAll(src)
	if err != nil {
		return "", nil, err
	}
	if len(data) == 0 {
		return "", nil, fmt.Errorf("no picture")
	}
	return path.Base(strings.ReplaceAll(name, "\\", "/")), data, nil
}
