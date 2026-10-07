package scanner

import (
	"os"
	"path/filepath"
)

// ThumbCache stores thumbnails on disk keyed by content hash, so a moved file
// reuses its thumbnail.
type ThumbCache struct{ dir string }

func NewThumbCache(dir string) (*ThumbCache, error) {
	return &ThumbCache{dir: dir}, os.MkdirAll(dir, 0o755)
}

// Path returns where the thumbnail for hash lives (it may not exist).
func (c *ThumbCache) Path(hash string) string {
	return filepath.Join(c.dir, hash[:2], hash+".jpg")
}

// Put writes atomically so concurrent readers never see a partial file.
func (c *ThumbCache) Put(hash string, jpeg []byte) error {
	p := c.Path(hash)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, jpeg, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
