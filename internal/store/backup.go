package store

import (
	"os"

	bolt "go.etcd.io/bbolt"
)

// Backup writes a consistent copy of the database to path while the
// library stays in use, through a temporary file, so a copy cut short
// never passes for a whole one.
func (s *Store) Backup(path string) error {
	tmp := path + ".tmp"
	if err := s.db.View(func(tx *bolt.Tx) error { return tx.CopyFile(tmp, 0o600) }); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}
