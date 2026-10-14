package store

import (
	"context"

	bolt "go.etcd.io/bbolt"
	"testing"
)

// A folder removed while its scan runs leaves nothing behind: the scan's
// files aren't recorded, and any an older version left are forgotten at
// the next start.
func TestRemovedFolderLeavesNoFiles(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id, err := h.s.AddFolder(ctx, "/gone", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.s.RemoveFolder(ctx, id); err != nil {
		t.Fatal(err)
	}
	if n, _ := h.s.Discover(ctx, id, 1, []Discovered{{Path: "a.jpg", Size: 1, MTime: 1}}); n != 0 || h.s.PendingCount() != 0 {
		t.Fatalf("a removed folder's file was recorded: %d pending", h.s.PendingCount())
	}
	// As an older version did: a file of a folder that no longer exists.
	f := &fileRec{ID: 99, FolderID: id, Path: "b.jpg", Size: 1, MTime: 1, Pending: true}
	h.s.db.Update(func(tx *bolt.Tx) error { return put(tx, bucketFiles, f.ID, f) })
	path := h.s.db.Path()
	h.s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if s2.PendingCount() != 0 || len(s2.files) != 0 {
		t.Fatalf("left over: %d pending, %d files", s2.PendingCount(), len(s2.files))
	}
}
