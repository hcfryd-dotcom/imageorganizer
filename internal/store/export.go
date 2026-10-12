package store

import (
	"cmp"
	"context"
	"slices"
)

// ExportItem is one photo of a person to copy out of the library.
type ExportItem struct {
	ImageID  int64
	FolderID int64
	Path     string // relative to the folder
	Taken    *int64 // capture time (Unix seconds), nil when unknown
}

// PersonExport lists the photos of person pid, oldest first: those whose
// faces are filed under them, and with confirmedOnly just those the user
// confirmed (named or ticked), leaving out automatic filing. Paintings and
// posters of the person are left out. It also returns the person's name.
func (s *Store) PersonExport(ctx context.Context, pid int64, confirmedOnly bool) (string, []ExportItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := s.people[pid]
	if p == nil {
		return "", nil, ErrNotFound
	}
	seen := map[int64]bool{}
	var out []ExportItem
	for _, f := range s.faces {
		if seen[f.ImageID] || f.Picture || f.NotFace {
			continue
		}
		c := s.clusters[f.ClusterID]
		filed := c != nil && c.PersonID == pid
		if !filed && f.Tagged != pid {
			continue
		}
		if confirmedOnly && f.Tagged != pid && (c == nil || c.Auto) {
			continue
		}
		im := s.images[f.ImageID]
		file := s.primaryFile(f.ImageID)
		if im == nil || file == nil {
			continue
		}
		seen[f.ImageID] = true
		out = append(out, ExportItem{ImageID: im.ID, FolderID: file.FolderID, Path: file.Path, Taken: im.TakenAt})
	}
	slices.SortFunc(out, func(a, b ExportItem) int {
		switch {
		case a.Taken == nil && b.Taken == nil:
			return cmp.Compare(a.ImageID, b.ImageID)
		case a.Taken == nil:
			return 1
		case b.Taken == nil:
			return -1
		}
		return cmp.Or(cmp.Compare(*a.Taken, *b.Taken), cmp.Compare(a.ImageID, b.ImageID))
	})
	return p.Name, out, nil
}
