// Package embed talks to a llama.cpp server running EmbeddingGemma 2, which
// maps text and images into one 768-dimensional space: a photo and a text
// query describing it end up close together.
package embed

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Client is safe for concurrent use.
type Client struct {
	http *http.Client

	mu     sync.Mutex
	base   string // server URL
	want   string // model asked for in requests ("" : whatever the server has loaded)
	marker string // llama.cpp's media placeholder; random per server start
	model  string

	cacheMu sync.Mutex
	cache   map[string][]float32 // recent query embeddings (pagination re-asks)
	order   []string
}

func New(baseURL, model string) *Client {
	return &Client{
		base:  strings.TrimSuffix(baseURL, "/"),
		want:  model,
		http:  &http.Client{Timeout: 2 * time.Minute},
		cache: map[string][]float32{},
	}
}

// Configure points the client at another server or model, effective for
// the next request. Vectors are tagged with the model, so a different
// model re-indexes the library.
func (c *Client) Configure(baseURL, model string) {
	c.mu.Lock()
	c.base, c.want, c.marker, c.model = strings.TrimSuffix(baseURL, "/"), model, "", ""
	c.mu.Unlock()
	c.cacheMu.Lock()
	c.cache, c.order = map[string][]float32{}, nil
	c.cacheMu.Unlock()
}

// URL is the server the client talks to.
func (c *Client) URL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.base
}

// Probe reports what a server at baseURL serves, without changing the
// client: the model file and whether it takes images.
func Probe(ctx context.Context, baseURL string) (model string, vision bool, err error) {
	t := New(baseURL, "")
	t.http.Timeout = 5 * time.Second
	_, model, err = t.props(ctx)
	if err != nil && strings.Contains(err.Error(), "no vision support") {
		return model, false, err
	}
	return model, err == nil, err
}

// props refreshes the media marker and model name from the server.
func (c *Client) props(ctx context.Context) (marker, model string, err error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.URL()+"/props", nil)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var p struct {
		MediaMarker string `json:"media_marker"`
		ModelPath   string `json:"model_path"`
		Modalities  struct {
			Vision bool `json:"vision"`
		} `json:"modalities"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return "", "", err
	}
	model = modelName(p.ModelPath)
	if !p.Modalities.Vision || p.MediaMarker == "" {
		return "", model, errors.New("embedding server has no vision support (start llama-server with --mmproj)")
	}
	c.mu.Lock()
	c.marker, c.model = p.MediaMarker, model
	c.mu.Unlock()
	return p.MediaMarker, model, nil
}

// Model returns the served model's file name, used to tag stored vectors so
// that switching models triggers re-indexing.
func (c *Client) Model(ctx context.Context) (string, error) {
	c.mu.Lock()
	m, want := c.model, c.want
	c.mu.Unlock()
	if want != "" {
		return want, nil // the model asked for is the one that answers
	}
	if m != "" {
		return m, nil
	}
	_, m, err := c.props(ctx)
	return m, err
}

// Image embeds an encoded image (JPEG/PNG). Images take no task prefix.
func (c *Client) Image(ctx context.Context, img []byte) ([]float32, error) {
	for attempt := 0; ; attempt++ {
		c.mu.Lock()
		marker := c.marker
		c.mu.Unlock()
		if marker == "" {
			var err error
			if marker, _, err = c.props(ctx); err != nil {
				return nil, err
			}
		}
		v, err := c.embed(ctx, map[string]any{
			"prompt_string":   marker,
			"multimodal_data": []string{base64.StdEncoding.EncodeToString(img)},
		})
		if err != nil && attempt == 0 && strings.Contains(err.Error(), "tokenize") {
			// The server restarted with a new marker; fetch it and retry.
			c.mu.Lock()
			c.marker = ""
			c.mu.Unlock()
			continue
		}
		return v, err
	}
}

// Query embeds a search query with EmbeddingGemma's retrieval prefix.
func (c *Client) Query(ctx context.Context, q string) ([]float32, error) {
	q = strings.TrimSpace(q)
	c.cacheMu.Lock()
	if v, ok := c.cache[q]; ok {
		c.cacheMu.Unlock()
		return v, nil
	}
	c.cacheMu.Unlock()
	v, err := c.embed(ctx, "task: search result | query: "+q)
	if err != nil {
		return nil, err
	}
	c.cacheMu.Lock()
	if len(c.order) >= 128 {
		delete(c.cache, c.order[0])
		c.order = c.order[1:]
	}
	c.cache[q] = v
	c.order = append(c.order, q)
	c.cacheMu.Unlock()
	return v, nil
}

func (c *Client) embed(ctx context.Context, content any) ([]float32, error) {
	c.mu.Lock()
	base, want := c.base, c.want
	c.mu.Unlock()
	payload := map[string]any{"content": content}
	if want != "" {
		payload["model"] = want
	}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", base+"/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding server: %s: %s", resp.Status, bytes.TrimSpace(data))
	}
	// Response: [{"index":0,"embedding":[[...]]}] (one row per pooled sequence).
	var out []struct {
		Embedding json.RawMessage `json:"embedding"`
	}
	if err := json.Unmarshal(data, &out); err != nil || len(out) == 0 {
		return nil, fmt.Errorf("embedding server: unexpected response: %.200s", data)
	}
	var rows [][]float32
	if err := json.Unmarshal(out[0].Embedding, &rows); err != nil {
		var flat []float32
		if err := json.Unmarshal(out[0].Embedding, &flat); err != nil {
			return nil, err
		}
		rows = [][]float32{flat}
	}
	if len(rows) == 0 || len(rows[0]) == 0 {
		return nil, errors.New("embedding server returned an empty vector")
	}
	return Normalize(rows[0]), nil
}

// Normalize scales v to unit length so a dot product is cosine similarity.
func Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}

// Dot is the cosine similarity of two normalized vectors.
func Dot(a, b []float32) float32 {
	var s float32
	for i := range min(len(a), len(b)) {
		s += a[i] * b[i]
	}
	return s
}

// modelName is the file name of the model the server loaded, whatever
// folder (and on Windows, separator) it was started with: the vectors are
// pinned to it, and a path changing between starts would re-index them all.
func modelName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		return path[i+1:]
	}
	return path
}
