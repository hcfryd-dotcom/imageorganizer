package store

import (
	"context"
	"time"

	bolt "go.etcd.io/bbolt"
)

// DeleteFile is a file of a photo, to be moved out of the library.
type DeleteFile struct {
	FileID   int64
	FolderID int64
	Path     string // relative to the folder
}

// PresentFiles lists the files of an image that are on disk: a photo
// copied into two folders has two.
func (s *Store) PresentFiles(ctx context.Context, imageID int64) ([]DeleteFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.images[imageID]; !ok {
		return nil, ErrNotFound
	}
	var out []DeleteFile
	for _, fid := range s.byImage[imageID] {
		if f := s.files[fid]; f.MissingSince == nil {
			out = append(out, DeleteFile{FileID: f.ID, FolderID: f.FolderID, Path: f.Path})
		}
	}
	return out, nil
}

// MarkFilesGone records that files were moved out of the library by the
// user, as a scan would on not finding them: a photo with no file left is
// no longer shown, but everything about it is kept, so it comes back as it
// was if the file is put back.
func (s *Store) MarkFilesGone(ctx context.Context, fileIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	var gone []*fileRec
	for _, id := range fileIDs {
		if f := s.files[id]; f != nil && f.MissingSince == nil {
			gone = append(gone, f)
		}
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, f := range gone {
			c := *f
			c.MissingSince = &now
			if err := put(tx, bucketFiles, c.ID, &c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, f := range gone {
		f.MissingSince = &now
	}
	return nil
}
