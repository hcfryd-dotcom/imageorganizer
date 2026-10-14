package embed

import "testing"

func TestModelName(t *testing.T) {
	for _, p := range []string{
		"embeddinggemma-2-Q8_0.gguf",
		"/models/embeddinggemma-2-Q8_0.gguf",
		`models\embeddinggemma-2-Q8_0.gguf`,
		`llama\models\embeddinggemma-2-Q8_0.gguf`,
		`E:\ImageOrganizer-models\embeddinggemma-2-Q8_0.gguf`,
	} {
		if got := modelName(p); got != "embeddinggemma-2-Q8_0.gguf" {
			t.Errorf("%s: %s", p, got)
		}
	}
}
