package clustering

import (
	"testing"

	"github.com/yourname/projctr/internal/testutil"
)

func TestEmbedder_Ready(t *testing.T) {
	tests := []struct {
		endpoint string
		want     bool
	}{
		{"http://localhost:11434", true},
		{"", false},
	}
	for _, tt := range tests {
		e := NewEmbedder("test-model", tt.endpoint)
		if got := e.Ready(); got != tt.want {
			t.Errorf("Ready() with endpoint=%q: got %v, want %v", tt.endpoint, got, tt.want)
		}
	}
}

func TestEmbedder_Embed(t *testing.T) {
	srv := testutil.MockEmbeddingServer(t, 384)
	e := NewEmbedder("test-model", srv.URL)

	vec, err := e.Embed("test text")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 384 {
		t.Fatalf("expected 384-dim vector, got %d", len(vec))
	}
	if vec[0] == 0 {
		t.Error("vector values should be non-zero")
	}
}

func TestEmbedder_Embed_NotConfigured(t *testing.T) {
	e := NewEmbedder("test-model", "")

	_, err := e.Embed("test text")
	if err == nil {
		t.Fatal("expected error for unconfigured endpoint")
	}
}

func TestEmbedder_Embed_UnreachableEndpoint(t *testing.T) {
	e := NewEmbedder("test-model", "http://127.0.0.1:1")

	_, err := e.Embed("test text")
	if err == nil {
		t.Fatal("expected error for unreachable endpoint")
	}
}

func TestEmbedder_EmbedBatch(t *testing.T) {
	srv := testutil.MockEmbeddingServer(t, 384)
	e := NewEmbedder("test-model", srv.URL)

	texts := []string{"hello", "world", "test"}
	vecs, err := e.EmbedBatch(texts)
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 3 {
		t.Fatalf("expected 3 vectors, got %d", len(vecs))
	}
	for i, v := range vecs {
		if len(v) != 384 {
			t.Errorf("vector[%d] has %d dims, want 384", i, len(v))
		}
	}
}

func TestEmbedder_Embed_ErrorResponse(t *testing.T) {
	srv := testutil.MockOllamaServerError(t, 500)
	e := NewEmbedder("test-model", srv.URL)

	_, err := e.Embed("test text")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
