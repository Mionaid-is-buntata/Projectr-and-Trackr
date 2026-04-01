package linkedin

import (
	"strings"
	"testing"

	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/testutil"
)

func TestGenerate_IncludesBriefFields(t *testing.T) {
	var capturedPrompt string
	srv := testutil.MockOllamaServerWithValidation(t, "Great LinkedIn post about the project!", func(prompt string) {
		capturedPrompt = prompt
	})

	gen := NewGenerator(srv.URL, "test-model")
	brief := &models.Brief{
		Title:             "Infrastructure Automator",
		ProblemStatement:  "Manual provisioning is slow",
		SuggestedApproach: "1. Automate with Terraform",
		TechnologyStack:   `["Terraform","Docker"]`,
		Complexity:        "medium",
		LinkedInAngle:     "Solving real hiring gaps",
		SourceRole:        "DevOps Engineer",
		SourceCompany:     "FinTech Corp",
	}
	project := &models.Project{
		GiteaURL: "https://gitea.local/infra-automator",
		LiveURL:  "https://demo.local",
		Notes:    "Built as a portfolio piece",
	}

	draft, err := gen.Generate(brief, project)
	if err != nil {
		t.Fatal(err)
	}
	if draft == "" {
		t.Fatal("expected non-empty draft")
	}

	// Verify prompt contains all key fields
	checks := []string{
		"Infrastructure Automator",
		"Manual provisioning is slow",
		"Terraform",
		"medium",
		"DevOps Engineer",
		"FinTech Corp",
		"https://gitea.local/infra-automator",
		"https://demo.local",
		"Built as a portfolio piece",
		"Solving real hiring gaps",
	}
	for _, want := range checks {
		if !strings.Contains(capturedPrompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestGenerate_NilProject(t *testing.T) {
	srv := testutil.MockOllamaServer(t, "A good post")
	gen := NewGenerator(srv.URL, "test-model")

	brief := &models.Brief{
		Title:            "Test",
		ProblemStatement: "Test problem",
		TechnologyStack:  `["Go"]`,
		Complexity:       "small",
	}

	draft, err := gen.Generate(brief, nil)
	if err != nil {
		t.Fatal(err)
	}
	if draft != "A good post" {
		t.Errorf("draft = %q", draft)
	}
}

func TestGenerateFromProject_IncludesProjectFields(t *testing.T) {
	var capturedPrompt string
	srv := testutil.MockOllamaServerWithValidation(t, "Manual project post", func(prompt string) {
		capturedPrompt = prompt
	})

	gen := NewGenerator(srv.URL, "test-model")
	project := &models.Project{
		Title:      "Custom Tool",
		Complexity: "large",
		GiteaURL:   "https://gitea.local/custom-tool",
		LiveURL:    "https://live.local/tool",
		Notes:      "Side project for learning",
	}

	draft, err := gen.GenerateFromProject(project)
	if err != nil {
		t.Fatal(err)
	}
	if draft == "" {
		t.Fatal("expected non-empty draft")
	}

	for _, want := range []string{"Custom Tool", "large", "https://gitea.local/custom-tool", "Side project for learning"} {
		if !strings.Contains(capturedPrompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestGenerate_UnreachableEndpoint(t *testing.T) {
	gen := NewGenerator("http://127.0.0.1:1", "test-model")
	brief := &models.Brief{Title: "Test", TechnologyStack: `["Go"]`}

	_, err := gen.Generate(brief, nil)
	if err != ErrLLMUnavailable {
		t.Errorf("expected ErrLLMUnavailable, got %v", err)
	}
}

func TestGenerate_EmptyResponse(t *testing.T) {
	srv := testutil.MockOllamaServer(t, "")
	gen := NewGenerator(srv.URL, "test-model")
	brief := &models.Brief{Title: "Test", TechnologyStack: `["Go"]`}

	_, err := gen.Generate(brief, nil)
	if err != ErrLLMUnavailable {
		t.Errorf("expected ErrLLMUnavailable for empty response, got %v", err)
	}
}

func TestGenerate_WritingGuidelines(t *testing.T) {
	var capturedPrompt string
	srv := testutil.MockOllamaServerWithValidation(t, "post", func(prompt string) {
		capturedPrompt = prompt
	})

	gen := NewGenerator(srv.URL, "test-model")
	brief := &models.Brief{Title: "Test", TechnologyStack: `["Go"]`, Complexity: "small"}

	gen.Generate(brief, nil)

	guidelines := []string{
		"Professional but conversational",
		"first person",
		"No em-dashes",
		"hashtags",
	}
	for _, g := range guidelines {
		if !strings.Contains(capturedPrompt, g) {
			t.Errorf("prompt missing guideline %q", g)
		}
	}
}

func TestGenerateFromProject_NoLinks(t *testing.T) {
	var capturedPrompt string
	srv := testutil.MockOllamaServerWithValidation(t, "post", func(prompt string) {
		capturedPrompt = prompt
	})

	gen := NewGenerator(srv.URL, "test-model")
	project := &models.Project{Title: "No Links Project"}

	gen.GenerateFromProject(project)

	if !strings.Contains(capturedPrompt, "(not yet published)") {
		t.Error("expected '(not yet published)' when no URLs set")
	}
}
