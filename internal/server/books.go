package server

import (
	"encoding/json"
	"net/http"

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
// the best photos, how many faces and how big the biggest is.
type photoInfo struct {
	ID      int64    `json:"id"`
	Place   string   `json:"place,omitempty"`
	People  []string `json:"people,omitempty"`
	Faces   int      `json:"faces,omitempty"`
	BigFace float32  `json:"big_face,omitempty"` // the biggest face's width, as a share of the photo's
}

// bookInfo answers {ids: [...]} with a photoInfo for each known photo.
func (s *Server) bookInfo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []int64 `json:"ids"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&req); err != nil {
		httpError(w, 400, err)
		return
	}
	if len(req.IDs) > 20000 {
		req.IDs = req.IDs[:20000]
	}
	out := make([]photoInfo, 0, len(req.IDs))
	for _, im := range s.Store.ImagesByID(r.Context(), req.IDs) {
		pi := photoInfo{ID: im.ID}
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
		}
		out = append(out, pi)
	}
	writeJSON(w, out)
}
