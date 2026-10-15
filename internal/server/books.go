package server

import (
	"encoding/json"
	"fmt"
	"image/jpeg"
	"math"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/lkarlslund/imageorganizer/internal/media"
	"github.com/lkarlslund/imageorganizer/internal/places"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Photo books (see store/books.go and web/book.html).

func (s *Server) bookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/books", func(w http.ResponseWriter, r *http.Request) {
		bs, err := s.Store.Books(r.Context())
		if err != nil {
			httpError(w, 500, err)
			return
		}
		writeJSON(w, nonNilBooks(bs))
	})
	mux.HandleFunc("GET /api/books/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		bk, err := s.Store.GetBook(r.Context(), id)
		if err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, bk)
	})
	mux.HandleFunc("POST /api/books", s.saveBook)
	mux.HandleFunc("PUT /api/books/{id}", s.saveBook)
	mux.HandleFunc("DELETE /api/books/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		if err := s.Store.DeleteBook(r.Context(), id); err != nil {
			storeError(w, err)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/books/info", s.bookInfo)
}

func nonNilBooks(bs []store.Book) []store.Book {
	if bs == nil {
		return []store.Book{}
	}
	return bs
}

func (s *Server) saveBook(w http.ResponseWriter, r *http.Request) {
	var bk store.Book
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&bk); err != nil {
		httpError(w, 400, err)
		return
	}
	bk.ID = 0
	if r.PathValue("id") != "" {
		id, err := pathID(r)
		if err != nil {
			httpError(w, 400, err)
			return
		}
		bk.ID = id
	}
	out, err := s.Store.SaveBook(r.Context(), bk)
	if err != nil {
		if bk.ID != 0 {
			storeError(w, err)
		} else {
			httpError(w, 400, err)
		}
		return
	}
	writeJSON(w, out)
}

// photoInfo is what a book page knows of a photo beyond the photo list:
// for captions, the town it was taken in and who is in it; for picking
// the best photos, how many faces and how big the biggest is, each face
// (whose, how wide, cut off by the photo's edge), and when asked (look)
// how light the photo is.
type photoInfo struct {
	ID      int64      `json:"id"`
	Place   string     `json:"place,omitempty"`
	People  []string   `json:"people,omitempty"`
	Faces   int        `json:"faces,omitempty"`
	BigFace float32    `json:"big_face,omitempty"` // the biggest face's width, as a share of the photo's
	List    []faceBit  `json:"list,omitempty"`
	PH      string     `json:"ph,omitempty"` // the perceptual hash, in hex (64 bits are too many for JavaScript numbers)
	Look    *photoLook `json:"look,omitempty"`
}

type faceBit struct {
	P   int64   `json:"p,omitempty"` // the person, if known
	W   float32 `json:"w"`           // width, as a share of the photo's
	Cut bool    `json:"cut,omitempty"`
	// B is the face's box: left, top, right, bottom, as shares of the
	// photo, for keeping faces in view when a photo is cropped.
	B [4]float32 `json:"b"`
}

type photoLook struct {
	Light float64 `json:"light"`
}

// cutEdge is how near the photo's edge (as a share of it) a face box may
// reach before the face counts as cut off.
const cutEdge = 0.004

// looks keeps photoLooks by image hash: measuring means reading the
// thumbnail.
var (
	looksMu sync.Mutex
	looks   = map[string]photoLook{}
)

// lookOf measures a photo from its thumbnail (nil if there is none).
func (s *Server) lookOf(hash string) *photoLook {
	looksMu.Lock()
	l, ok := looks[hash]
	looksMu.Unlock()
	if ok {
		return &l
	}
	f, err := os.Open(s.Thumbs.Path(hash))
	if err != nil {
		return nil
	}
	img, err := jpeg.Decode(f)
	f.Close()
	if err != nil {
		return nil
	}
	m := media.Looks(img)
	l = photoLook{Light: math.Round(m.Light*100) / 100}
	looksMu.Lock()
	looks[hash] = l
	looksMu.Unlock()
	return &l
}

// bookInfo answers {ids: [...], look: bool} with a photoInfo for each
// known photo.
func (s *Server) bookInfo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs  []int64 `json:"ids"`
		Look bool    `json:"look"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if len(req.IDs) > 20000 {
		req.IDs = req.IDs[:20000]
	}
	ims := s.Store.ImagesByID(r.Context(), req.IDs)
	out := make([]photoInfo, 0, len(ims))
	for _, im := range ims {
		pi := photoInfo{ID: im.ID}
		if im.PHash != 0 {
			pi.PH = fmt.Sprintf("%016x", im.PHash)
		}
		if im.Lat != nil && im.Lon != nil {
			if p, ok := places.Nearest(*im.Lat, *im.Lon); ok {
				pi.Place = p.Name
				if p.Country != "DK" {
					pi.Place += ", " + places.Country(p.Country)
				}
			}
		}
		for _, f := range s.Store.ImageFaces(r.Context(), im.ID) {
			if f.Bg {
				continue
			}
			pi.Faces++
			if f.Person != "" {
				pi.People = append(pi.People, f.Person)
			}
			pi.BigFace = max(pi.BigFace, f.Box[2]-f.Box[0]) // boxes are shares of the photo
			b := f.Box
			pi.List = append(pi.List, faceBit{P: f.PersonID, W: b[2] - b[0],
				Cut: b[0] < cutEdge || b[1] < cutEdge || b[2] > 1-cutEdge || b[3] > 1-cutEdge, B: round3(b)})
		}
		out = append(out, pi)
	}
	if req.Look {
		// Thumbnails are read side by side; a book has hundreds or thousands.
		var wg sync.WaitGroup
		next := make(chan int)
		for range max(1, runtime.NumCPU()-1) {
			wg.Go(func() {
				for i := range next {
					out[i].Look = s.lookOf(ims[i].Hash)
				}
			})
		}
		for i := range out {
			next <- i
		}
		close(next)
		wg.Wait()
	}
	writeJSON(w, out)
}

// round3 keeps three decimals of a box, enough for laying out a page.
func round3(b [4]float32) [4]float32 {
	for i, v := range b {
		b[i] = float32(math.Round(float64(v)*1000) / 1000)
	}
	return b
}
