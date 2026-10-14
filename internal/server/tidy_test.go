package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

func TestTidyFolder(t *testing.T) {
	taken := time.Date(2011, 5, 4, 23, 30, 0, 0, time.UTC).Unix()
	item := func(t *int64, src string) store.TidyItem {
		return store.TidyItem{ExportItem: store.ExportItem{Taken: t}, Source: src}
	}
	da, en := tidyLang["da"], tidyLang["en"]
	for _, c := range []struct {
		names tidyNames
		it    store.TidyItem
		want  string
	}{
		{da, item(&taken, "Canon"), filepath.Join("x", "2011", "05 Maj", "Canon")},
		{da, item(&taken, "screenshot"), filepath.Join("x", "2011", "05 Maj", "Skærmbilleder")},
		{da, item(nil, ""), filepath.Join("x", "Ukendt år", "Ukendt kamera")},
		{en, item(&taken, "iPhone"), filepath.Join("x", "2011", "05 May", "iPhone")},
	} {
		if got := c.names.folder("x", c.it); got != c.want {
			t.Errorf("%+v: %s, want %s", c.it, got, c.want)
		}
	}
}
