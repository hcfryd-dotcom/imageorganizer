package server

import (
	"encoding/json"
	"testing"

	"github.com/lkarlslund/imageorganizer/internal/store"
)

// A face with no alike group must give an empty list, not null: the page
// drew a blank group card for null, and naming it sent group "undefined".
func TestFoundFaceNoMatchesIsEmptyList(t *testing.T) {
	var none []store.SimilarGroup
	b, err := json.Marshal(foundFace{Matches: nonNil(none)})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if m, ok := got["matches"].([]any); !ok || len(m) != 0 {
		t.Fatalf("matches = %v, want []", got["matches"])
	}
}
