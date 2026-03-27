package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// MockOllamaServer creates an httptest server that mimics Ollama's /api/generate endpoint.
// It returns the given response text in the Ollama response format.
func MockOllamaServer(t *testing.T, response string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": response,
			"done":     true,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// MockEmbeddingServer creates an httptest server that mimics Ollama's embedding endpoint.
// Returns a vector of the given dimension filled with 0.1 values.
func MockEmbeddingServer(t *testing.T, dims int) *httptest.Server {
	t.Helper()
	vec := make([]float32, dims)
	for i := range vec {
		vec[i] = 0.1
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"embedding": vec,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// MockOllamaServerWithValidation creates a mock that also captures and validates the prompt.
func MockOllamaServerWithValidation(t *testing.T, response string, validatePrompt func(prompt string)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Prompt string `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		if validatePrompt != nil {
			validatePrompt(req.Prompt)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"response": response,
			"done":     true,
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// MockOllamaServerError creates a mock that returns HTTP errors.
func MockOllamaServerError(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, fmt.Sprintf("error %d", statusCode), statusCode)
	}))
	t.Cleanup(srv.Close)
	return srv
}
