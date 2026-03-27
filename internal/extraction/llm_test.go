package extraction

import (
	"testing"

	"github.com/yourname/projctr/internal/testutil"
)

func TestLLMExtractor_Extract_ValidResponse(t *testing.T) {
	response := `[{"challenge_text":"Need Kubernetes experience","domain":"platform","outcome_text":"to enable cloud deployments","technologies":["Kubernetes"],"confidence":0.9}]`
	srv := testutil.MockOllamaServer(t, response)
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("We need Kubernetes experience for cloud infrastructure")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("expected 1 pain point, got %d", len(pts))
	}
	if pts[0].ChallengeText != "Need Kubernetes experience" {
		t.Errorf("challenge = %q", pts[0].ChallengeText)
	}
	if pts[0].Domain != "platform" {
		t.Errorf("domain = %q, want platform", pts[0].Domain)
	}
	if pts[0].Confidence != 0.9 {
		t.Errorf("confidence = %f, want 0.9", pts[0].Confidence)
	}
}

func TestLLMExtractor_Extract_MarkdownFenced(t *testing.T) {
	response := "```json\n" +
		`[{"challenge_text":"Docker skills needed","domain":"platform","outcome_text":"","technologies":["Docker"],"confidence":0.8}]` +
		"\n```"
	srv := testutil.MockOllamaServer(t, response)
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("Docker is required")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("expected 1, got %d", len(pts))
	}
	if pts[0].ChallengeText != "Docker skills needed" {
		t.Errorf("challenge = %q", pts[0].ChallengeText)
	}
}

func TestLLMExtractor_Extract_EmptyResponse(t *testing.T) {
	srv := testutil.MockOllamaServer(t, "")
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("some text")
	if err != nil {
		t.Fatal(err)
	}
	if pts != nil {
		t.Errorf("expected nil, got %d points", len(pts))
	}
}

func TestLLMExtractor_Extract_UnreachableEndpoint(t *testing.T) {
	ext := NewLLMExtractor("http://127.0.0.1:1", "test-model")

	pts, err := ext.Extract("some text")
	if err != nil {
		t.Fatalf("expected nil error for unreachable, got %v", err)
	}
	if pts != nil {
		t.Errorf("expected nil points, got %d", len(pts))
	}
}

func TestLLMExtractor_Extract_MalformedJSON(t *testing.T) {
	srv := testutil.MockOllamaServer(t, "this is not json at all")
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("some text")
	if err != nil {
		t.Fatal(err)
	}
	if pts != nil {
		t.Errorf("expected nil, got %d", len(pts))
	}
}

func TestLLMExtractor_Extract_ConfidenceClamping(t *testing.T) {
	response := `[{"challenge_text":"test","domain":"general","outcome_text":"","technologies":[],"confidence":1.5},{"challenge_text":"test2","domain":"general","outcome_text":"","technologies":[],"confidence":-0.5}]`
	srv := testutil.MockOllamaServer(t, response)
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("text")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 2 {
		t.Fatalf("expected 2, got %d", len(pts))
	}
	if pts[0].Confidence != 1.0 {
		t.Errorf("confidence clamped to 1.0, got %f", pts[0].Confidence)
	}
	if pts[1].Confidence != 0.0 {
		t.Errorf("confidence clamped to 0.0, got %f", pts[1].Confidence)
	}
}

func TestLLMExtractor_Extract_SkipsEmptyChallenge(t *testing.T) {
	response := `[{"challenge_text":"","domain":"general","outcome_text":"","technologies":[],"confidence":0.5},{"challenge_text":"Real challenge","domain":"tool","outcome_text":"","technologies":[],"confidence":0.7}]`
	srv := testutil.MockOllamaServer(t, response)
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("text")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("expected 1 (empty skipped), got %d", len(pts))
	}
	if pts[0].ChallengeText != "Real challenge" {
		t.Errorf("challenge = %q", pts[0].ChallengeText)
	}
}

func TestLLMExtractor_Extract_DomainNormalization(t *testing.T) {
	response := `[{"challenge_text":"test","domain":"PLATFORM","outcome_text":"","technologies":[],"confidence":0.8}]`
	srv := testutil.MockOllamaServer(t, response)
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("text")
	if err != nil {
		t.Fatal(err)
	}
	if pts[0].Domain != "platform" {
		t.Errorf("domain = %q, want 'platform'", pts[0].Domain)
	}
}

func TestLLMExtractor_Extract_InvalidDomain(t *testing.T) {
	response := `[{"challenge_text":"test","domain":"unknown_thing","outcome_text":"","technologies":[],"confidence":0.8}]`
	srv := testutil.MockOllamaServer(t, response)
	ext := NewLLMExtractor(srv.URL, "test-model")

	pts, err := ext.Extract("text")
	if err != nil {
		t.Fatal(err)
	}
	if pts[0].Domain != "general" {
		t.Errorf("invalid domain should normalize to 'general', got %q", pts[0].Domain)
	}
}

func TestExtractJSONArray(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain array", `[{"a":1}]`, `[{"a":1}]`},
		{"with text before", `Here is the result: [{"a":1}]`, `[{"a":1}]`},
		{"markdown fenced", "```json\n[{\"a\":1}]\n```", `[{"a":1}]`},
		{"no array", "just some text", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONArray(tt.input)
			if got != tt.want {
				t.Errorf("extractJSONArray(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormaliseDomain(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"language", "language"},
		{"FRAMEWORK", "framework"},
		{"  Platform  ", "platform"},
		{"invalid", "general"},
		{"", "general"},
	}
	for _, tt := range tests {
		got := normaliseDomain(tt.input)
		if got != tt.want {
			t.Errorf("normaliseDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestClampConfidence(t *testing.T) {
	tests := []struct {
		input, want float64
	}{
		{0.5, 0.5},
		{0.0, 0.0},
		{1.0, 1.0},
		{-0.1, 0.0},
		{1.5, 1.0},
	}
	for _, tt := range tests {
		got := clampConfidence(tt.input)
		if got != tt.want {
			t.Errorf("clampConfidence(%f) = %f, want %f", tt.input, got, tt.want)
		}
	}
}
