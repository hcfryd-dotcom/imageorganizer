package store

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"
)

// Settings are the user's adjustments, kept in the database and edited in
// the UI. Zero values mean "the default". Command-line flags override them.
type Settings struct {
	// Search by meaning: a llama.cpp server running a multimodal embedding
	// model (EmbeddingGemma 2 with --mmproj).
	EmbedURL     string `json:"embed_url"`
	EmbedModel   string `json:"embed_model"` // sent as "model"; empty: whatever the server has loaded
	EmbedWorkers int    `json:"embed_workers"`

	// Face recognition (applies after a restart).
	FaceModels   string `json:"face_models"`
	ONNXRuntime  string `json:"onnxruntime"`
	FaceDevice   string `json:"face_device"` // auto, cuda, cpu
	FaceGPUMemMB int    `json:"face_gpu_mem_mb"`

	// Import (applies after a restart).
	Workers       int `json:"workers"`
	Conns         int `json:"conns"`
	RescanMinutes int `json:"rescan_minutes"` // -1: never

	// People (applies at once).
	AutoFileOff bool    `json:"auto_file_off"`
	AutoFileMin float32 `json:"auto_file_min"` // similarity to file a group without asking
	ReviewMin   int     `json:"review_min"`    // smallest group shown in Review

	// ShowNsfw lists photos the NSFW classifier (or the user) marked; by
	// default they are left out of browsing and search ($nsfw lists them).
	ShowNsfw bool `json:"show_nsfw"`

	// Map tiles: a URL template with {z}/{x}/{y} (and optionally {s}).
	MapTiles string `json:"map_tiles"`
}

var keySettings = []byte("settings")

// Settings returns the saved settings.
func (s *Store) Settings() Settings {
	var out Settings
	_ = s.db.View(func(tx *bolt.Tx) error {
		if v := tx.Bucket(bucketMeta).Get(keySettings); v != nil {
			return json.Unmarshal(v, &out)
		}
		return nil
	})
	return out
}

// SaveSettings stores the settings and applies the ones that take effect
// at once (see Tuning).
func (s *Store) SaveSettings(st Settings) error {
	data, err := json.Marshal(st)
	if err != nil {
		return err
	}
	if err := s.db.Update(func(tx *bolt.Tx) error { return tx.Bucket(bucketMeta).Put(keySettings, data) }); err != nil {
		return err
	}
	s.SetTuning(st)
	return nil
}

// tuning holds the people settings in effect.
type tuning struct {
	autoOff   bool
	autoMin   float32
	reviewMin int
}

// SetTuning applies the people settings: automatic filing and Review size.
func (s *Store) SetTuning(st Settings) {
	t := tuning{autoOff: st.AutoFileOff, autoMin: autoAssign, reviewMin: 2}
	if st.AutoFileMin > 0 {
		t.autoMin = st.AutoFileMin
	}
	if st.ReviewMin > 0 {
		t.reviewMin = st.ReviewMin
	}
	s.mu.Lock()
	s.tune = t
	s.mu.Unlock()
}

// ReviewMin is the smallest group shown in Review.
func (s *Store) ReviewMin() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reviewMinLocked()
}

func (s *Store) reviewMinLocked() int {
	if s.tune.reviewMin > 0 {
		return s.tune.reviewMin
	}
	return 2
}

// autoMinLocked was the threshold for filing whole groups automatically;
// faces are filed one by one now (FileFaces), so it is always off.
func (s *Store) autoMinLocked() float32 { return 0 }
