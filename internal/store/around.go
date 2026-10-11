package store

import (
	"cmp"
	"context"
	"path"
	"slices"
)

// Around is a photo's neighbourhood: the photos taken just before and
// after it, or, for a photo without a date, the ones next to it in its
// folder.
type Around struct {
	By    string  `json:"by"` // "time" or "folder"
	Items []Image `json:"items"`
}

// Around returns up to n photos on each side of an image, the image itself
// included, in order: by time when it has a date (across all folders),
// otherwise by name within its folder.
func (s *Store) Around(ctx context.Context, id int64, n int) (Around, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	im, ok := s.images[id]
	if !ok {
		return Around{}, ErrNotFound
	}
	var list []*imageRec
	out := Around{By: "time"}
	if im.TakenAt != nil {
		for _, o := range s.images {
			if o.TakenAt != nil && s.present(o.ID) {
				list = append(list, o)
			}
		}
		sortImages(list, "taken_asc")
	} else {
		out.By = "folder"
		f := s.primaryFile(id)
		if f == nil {
			return Around{}, ErrNotFound
		}
		dir := path.Dir(f.Path)
		names := map[int64]string{}
		for _, o := range s.images {
			if of := s.primaryFile(o.ID); of != nil && of.FolderID == f.FolderID && path.Dir(of.Path) == dir {
				list = append(list, o)
				names[o.ID] = of.Path
			}
		}
		slices.SortFunc(list, func(a, b *imageRec) int {
			return cmp.Or(cmp.Compare(names[a.ID], names[b.ID]), cmp.Compare(a.ID, b.ID))
		})
	}
	i := slices.Index(list, im)
	if i < 0 {
		return Around{}, ErrNotFound
	}
	for _, o := range list[max(i-n, 0):min(i+n+1, len(list))] {
		out.Items = append(out.Items, s.view(o))
	}
	return out, nil
}
