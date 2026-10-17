package server

import (
	"errors"
	"net/http"
	"strconv"
	"sync"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// The Cleanup page: photos one may want rid of, by kind. Only lists; the
// page deletes what the user chooses through /api/images/delete, which
// moves the files to the deleted-photos folder.

func (s *Server) cleanupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cleanup/{kind}", s.cleanup)
}

// What receipts and papers look like, and what they are told apart from
// (screenshots, and the photos one keeps: people, places, nature), as
// texts for the embedding model; Danish and English alike.
var cleanupTexts = map[string]struct{ pos, neg []string }{
	"receipts": {
		pos: []string{"a photo of a paper receipt from a shop", "kvittering", "bon fra butik, kassebon"},
		neg: []string{"a screenshot of a phone screen", "a photo of people", "a landscape or a place"},
	},
	"papers": {
		pos: []string{"a photo of a printed document, letter or form", "et dokument eller brev på papir", "a handwritten note on paper", "a page of text, a sign or a whiteboard"},
		neg: []string{"a screenshot of a phone screen", "a photo of people", "a landscape or a place", "a paper receipt from a shop"},
	},
}

// cleanupLeast is how much more a photo must look like a receipt or a
// paper than like what they are told apart from, to be offered: below it,
// on his library, came photos of other things (seen on 2026-10-17).
const cleanupLeast = 0.05

var (
	cleanupVecMu sync.Mutex
	cleanupVecs  = map[string][]float32{} // the texts' vectors, asked once
)

func (s *Server) cleanupVectors(r *http.Request, texts []string) ([][]float32, error) {
	var out [][]float32
	for _, t := range texts {
		cleanupVecMu.Lock()
		v, ok := cleanupVecs[t]
		cleanupVecMu.Unlock()
		if !ok {
			var err error
			if v, err = s.Embed.Query(r.Context(), t); err != nil {
				return nil, err
			}
			cleanupVecMu.Lock()
			cleanupVecs[t] = v
			cleanupVecMu.Unlock()
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Server) cleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var items []store.CleanupItem
	switch kind := r.PathValue("kind"); kind {
	case "receipts", "papers":
		if s.Embed == nil {
			httpError(w, 409, errors.New("search by meaning is off, so receipts and papers can’t be told"))
			return
		}
		n := 400
		if v, err := strconv.Atoi(r.URL.Query().Get("n")); err == nil && v > 0 {
			n = min(v, 2000)
		}
		t := cleanupTexts[kind]
		pos, err := s.cleanupVectors(r, t.pos)
		if err == nil {
			var neg [][]float32
			if neg, err = s.cleanupVectors(r, t.neg); err == nil {
				items = s.Store.CleanupByMeaning(ctx, pos, neg, n, cleanupLeast)
			}
		}
		if err != nil {
			httpError(w, 503, err)
			return
		}
	case "shots":
		items = s.Store.CleanupShots(ctx)
	case "blurry":
		items = s.Store.CleanupBlurry(ctx)
	case "copies":
		items = s.Store.CleanupCopies(ctx)
	default:
		httpError(w, 404, errors.New("no such kind"))
		return
	}
	writeJSON(w, map[string]any{"items": nonNil(items)})
}
