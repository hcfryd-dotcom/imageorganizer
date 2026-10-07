package embed

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/lkarlslund/imageorganizer/internal/pipeline"
	"github.com/lkarlslund/imageorganizer/internal/store"
)

// Stage returns the pipeline stage that embeds images from their cached
// thumbnails, so originals are never re-read from a (possibly remote) source.
// workers should match the llama.cpp server's --parallel slots.
func Stage(c *Client, st *store.Store, thumbPath func(hash string) string, workers int, log *slog.Logger) pipeline.Stage[store.VectorJob] {
	return pipeline.Stage[store.VectorJob]{
		Name:    "embed",
		Workers: workers,
		Ready: func(ctx context.Context) error {
			// Pin vectors to the served model; a new model re-indexes all.
			model, err := c.Model(ctx)
			if err != nil {
				return err
			}
			reset, err := st.SetVectorModel(ctx, model)
			if err != nil {
				return err
			}
			if reset {
				log.Info("embedding model changed; re-indexing all images", "model", model)
			}
			return nil
		},
		Next: st.NeedVectors,
		Key:  func(j store.VectorJob) int64 { return j.ID },
		Do: func(ctx context.Context, j store.VectorJob) error {
			thumb, err := os.ReadFile(thumbPath(j.Hash))
			if err != nil {
				return err
			}
			v, err := c.Image(ctx, thumb)
			if err != nil {
				return err
			}
			return st.SetVector(ctx, j.ID, j.Hash, v)
		},
		Transient: isConnErr,
		Log:       log,
	}
}

// isConnErr reports errors that mean the server is unreachable or busy
// restarting, as opposed to a problem with one image.
func isConnErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	msg := err.Error()
	for _, m := range []string{"connection refused", "connection reset", "EOF", "503 Service Unavailable"} {
		if strings.Contains(msg, m) {
			return true
		}
	}
	return false
}
