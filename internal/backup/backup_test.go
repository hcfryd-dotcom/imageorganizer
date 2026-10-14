package backup

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

func TestBackup(t *testing.T) {
	data := t.TempDir()
	st, err := store.Open(filepath.Join(data, "library.bolt"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.AddFolder(context.Background(), "/photos", ""); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(data, "backups")
	clock := time.Date(2026, 10, 1, 12, 0, 0, 0, time.Local)
	r := &Runner{Store: st, DefaultDir: dir, Log: slog.New(slog.DiscardHandler), now: func() time.Time { return clock }}
	// A file of the user's in the folder is never touched.
	os.MkdirAll(dir, 0o700)
	mine := filepath.Join(dir, "library.bolt.before-switch")
	os.WriteFile(mine, []byte("mine"), 0o600)

	p, err := r.backup(false)
	if err != nil || filepath.Base(p) != "library-2026-10-01.bolt" {
		t.Fatalf("first: %q %v", p, err)
	}
	// The copy opens, with the folder in it (opened as a duplicate:
	// opening it would date it now).
	b, _ := os.ReadFile(p)
	check := filepath.Join(t.TempDir(), "check.bolt")
	os.WriteFile(check, b, 0o600)
	cp, err := store.Open(check)
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := cp.Folders(context.Background())
	cp.Close()
	if len(fs) != 1 {
		t.Fatalf("copy has %d folders", len(fs))
	}
	// Not again the same day, unless asked.
	if p, _ := r.backup(false); p != "" {
		t.Fatalf("second the same day: %s", p)
	}
	if p, _ := r.Now(); filepath.Base(p) != "library-2026-10-01-2.bolt" {
		t.Fatalf("on request: %s", p)
	}
	// Twenty days on, fourteen copies are kept.
	for i := 0; i < 20; i++ {
		clock = clock.Add(25 * time.Hour)
		if _, err := r.backup(false); err != nil {
			t.Fatal(err)
		}
	}
	st0 := r.Status()
	if st0.Copies != DefaultKeep || st0.Last != clock.Unix() || st0.Dir != dir {
		t.Fatalf("status %+v", st0)
	}
	if _, err := os.Stat(mine); err != nil {
		t.Fatalf("the user's file was removed: %v", err)
	}
	// Off: no copies, and a status saying so.
	s := st.Settings()
	s.BackupDir = "off"
	st.SaveSettings(s)
	clock = clock.Add(48 * time.Hour)
	if p, _ := r.backup(false); p != "" || r.Status().Dir != "" {
		t.Fatalf("off: %q %+v", p, r.Status())
	}
}
