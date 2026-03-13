package clustering

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Embedder generates vector embeddings for pain point text.
// Uses the same model as Huntr to ensure vectors are in the same semantic space.
// Model: sentence-transformers/all-MiniLM-L6-v2 (384 dims). Must match Huntr.
// Supports Ollama-style API: POST {endpoint} with {"model","prompt"} -> {"embedding":[]}
type Embedder struct {
	model    string
	endpoint string
	client   *http.Client
}

// NewEmbedder creates an Embedder using the configured model and endpoint.
func NewEmbedder(model, endpoint string) *Embedder {
	return &Embedder{
		model:    model,
		endpoint: endpoint,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

// Ready returns true if the embedder has a configured endpoint.
func (e *Embedder) Ready() bool {
	return e.endpoint != ""
}

type ollamaEmbedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResp struct {
	Embedding []float64 `json:"embedding"`
}

// Embed generates a vector embedding for the given text.
// Returns an error if the embedding endpoint is unreachable or empty.
func (e *Embedder) Embed(text string) ([]float32, error) {
	if !e.Ready() {
		return nil, fmt.Errorf("embedding endpoint not configured")
	}
	body, _ := json.Marshal(ollamaEmbedReq{Model: e.model, Prompt: text})
	req, err := http.NewRequestWithContext(context.Background(), "POST", e.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embedding API: %s", resp.Status)
	}
	var out ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	vec := make([]float32, len(out.Embedding))
	for i, v := range out.Embedding {
		vec[i] = float32(v)
	}
	return vec, nil
}

// EmbedBatch generates embeddings for multiple texts sequentially.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		vec, err := e.Embed(t)
		if err != nil {
			return nil, err
		}
		out[i] = vec
	}
	return out, nil
}
