package huntr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// VectorClient provides read-only access to Huntr's ChromaDB CV collection.
// Projctr uses this exclusively for gap analysis — it never writes to this collection.
type VectorClient struct {
	baseURL    string
	collection string
	client     *http.Client
}

// NewVectorClient creates a read-only handle to Huntr's CV ChromaDB collection.
// baseURL is the Chroma HTTP server URL (e.g. http://localhost:8000).
// collectionName is from Huntr config (cv.active_collection).
func NewVectorClient(baseURL, collectionName string) (*VectorClient, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	if baseURL == "" {
		return nil, fmt.Errorf("chroma base URL cannot be empty")
	}
	return &VectorClient{
		baseURL:    baseURL,
		collection: collectionName,
		client:     &http.Client{},
	}, nil
}

// Close is a no-op for HTTP client; implements io.Closer for compatibility.
func (v *VectorClient) Close() error {
	return nil
}

// chromaGetRequest is the payload for Chroma's Get API.
type chromaGetRequest struct {
	Include []string `json:"include"`
	Limit   *int     `json:"limit,omitempty"`
	Offset  *int     `json:"offset,omitempty"`
}

// chromaGetResponse is the response from Chroma's Get API.
type chromaGetResponse struct {
	IDs        []string           `json:"ids"`
	Embeddings [][]float32        `json:"embeddings,omitempty"`
	Metadatas  []map[string]any   `json:"metadatas,omitempty"`
}

// FetchCVEmbeddings retrieves all CV embedding vectors from Huntr's collection.
// These are used by the gap analysis service for cosine similarity comparison.
func (v *VectorClient) FetchCVEmbeddings(ctx context.Context) ([]CVPoint, error) {
	// Chroma v2 API: POST /api/v2/tenants/{tenant}/databases/{database}/collections/{collection}/get
	// Default tenant/database for local Chroma
	tenant := "default_tenant"
	database := "default_database"
	url := fmt.Sprintf("%s/api/v2/tenants/%s/databases/%s/collections/%s/get",
		v.baseURL, tenant, database, v.collection)

	limit := 10000
	reqBody := chromaGetRequest{
		Include: []string{"embeddings", "metadatas"},
		Limit:   &limit,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chroma get failed: %s", resp.Status)
	}

	var result chromaGetResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	n := len(result.IDs)
	out := make([]CVPoint, n)
	for i := 0; i < n; i++ {
		out[i] = CVPoint{
			ID:     result.IDs[i],
			Vector: nil,
			Payload: nil,
		}
		if i < len(result.Embeddings) {
			out[i].Vector = result.Embeddings[i]
		}
		if i < len(result.Metadatas) && result.Metadatas[i] != nil {
			out[i].Payload = result.Metadatas[i]
		}
	}
	return out, nil
}

// CVPoint represents a single point from Huntr's CV collection.
type CVPoint struct {
	ID      string
	Vector  []float32
	Payload map[string]interface{}
}
