package extraction

import (
	"testing"

	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/testutil"
)

func newTestRulesExtractor(t *testing.T) *RulesExtractor {
	t.Helper()
	return &RulesExtractor{dict: []TechEntry{
		{Canonical: "Python", Category: "language", Variants: []string{"python", "python3"}},
		{Canonical: "Go", Category: "language", Variants: []string{"go", "golang"}},
		{Canonical: "Docker", Category: "platform", Variants: []string{"docker"}},
	}}
}

func TestService_Extract_RulesMode(t *testing.T) {
	rules := newTestRulesExtractor(t)
	svc := NewService("rules", rules, nil)

	pts, err := svc.Extract("Experience with Python and Docker required for this role")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) == 0 {
		t.Fatal("expected pain points from rules extraction")
	}
}

func TestService_Extract_LLMMode_FallsBackToRules(t *testing.T) {
	rules := newTestRulesExtractor(t)
	// LLM with unreachable endpoint — should fall back to rules
	llm := NewLLMExtractor("http://127.0.0.1:1", "test")
	svc := NewService("llm", rules, llm)

	pts, err := svc.Extract("Experience with Python required")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) == 0 {
		t.Fatal("expected fallback to rules extraction")
	}
}

func TestService_Extract_LLMMode_UsesLLMWhenAvailable(t *testing.T) {
	rules := newTestRulesExtractor(t)
	response := `[{"challenge_text":"LLM extracted point","domain":"language","outcome_text":"","technologies":["Python"],"confidence":0.95}]`
	srv := testutil.MockOllamaServer(t, response)
	llm := NewLLMExtractor(srv.URL, "test")
	svc := NewService("llm", rules, llm)

	pts, err := svc.Extract("Python experience needed")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 1 {
		t.Fatalf("expected 1 point from LLM, got %d", len(pts))
	}
	if pts[0].ChallengeText != "LLM extracted point" {
		t.Errorf("expected LLM point, got rules point: %q", pts[0].ChallengeText)
	}
}

func TestService_Extract_BothMode_Merges(t *testing.T) {
	rules := newTestRulesExtractor(t)
	// LLM returns a point with the same 20-char prefix as rules but higher confidence
	response := `[{"challenge_text":"Experience with Python is essential for our backend","domain":"language","outcome_text":"","technologies":["Python"],"confidence":0.99}]`
	srv := testutil.MockOllamaServer(t, response)
	llm := NewLLMExtractor(srv.URL, "test")
	svc := NewService("both", rules, llm)

	pts, err := svc.Extract("Experience with Python is essential for building scalable systems")
	if err != nil {
		t.Fatal(err)
	}
	// Should have merged — dedup by 20-char prefix, higher confidence wins
	for _, p := range pts {
		key := dedupeKey(p.ChallengeText)
		if key == "experience with pyth" && p.Confidence < 0.99 {
			t.Errorf("expected LLM's higher confidence to win for key %q, got %f", key, p.Confidence)
		}
	}
}

func TestService_Extract_BothMode_NilLLM(t *testing.T) {
	rules := newTestRulesExtractor(t)
	svc := NewService("both", rules, nil)

	pts, err := svc.Extract("Experience with Docker required")
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) == 0 {
		t.Fatal("expected rules results even with nil LLM")
	}
}

func TestMergePainPoints_HigherConfidenceWins(t *testing.T) {
	rulesPoints := []models.PainPoint{
		{ChallengeText: "Experience with Python needed", Confidence: 0.6},
	}
	llmPoints := []models.PainPoint{
		{ChallengeText: "Experience with Python is critical for ML pipeline", Confidence: 0.95},
	}

	merged := mergePainPoints(rulesPoints, llmPoints)
	// Both share the 20-char prefix "experience with pyth"
	for _, p := range merged {
		if dedupeKey(p.ChallengeText) == "experience with pyth" {
			if p.Confidence != 0.95 {
				t.Errorf("expected higher confidence 0.95, got %f", p.Confidence)
			}
		}
	}
}

func TestDedupeKey(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Short", "short"},
		{"This is exactly twenty!", "this is exactly twen"},
		{"  Trim whitespace  ", "trim whitespace"},
		{"UPPERCASE TEXT HERE", "uppercase text here"},
	}
	for _, tt := range tests {
		got := dedupeKey(tt.input)
		if got != tt.want {
			t.Errorf("dedupeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
