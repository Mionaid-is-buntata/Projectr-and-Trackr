package linkedin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/campbell/projctr/internal/models"
)

// ErrLLMUnavailable is returned when the Ollama endpoint is unreachable.
var ErrLLMUnavailable = errors.New("LLM unavailable")

// Generator creates LinkedIn post drafts via Ollama.
type Generator struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewGenerator creates a LinkedIn post generator.
func NewGenerator(endpoint, model string) *Generator {
	return &Generator{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

type ollamaGenerateReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResp struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Generate creates a LinkedIn post draft from a brief and its associated project.
func (g *Generator) Generate(b *models.Brief, p *models.Project) (string, error) {
	var linkParts []string
	if p != nil && p.GiteaURL != "" {
		linkParts = append(linkParts, "- Repository: "+p.GiteaURL)
	}
	if p != nil && p.LiveURL != "" {
		linkParts = append(linkParts, "- Live demo: "+p.LiveURL)
	}
	linksSection := strings.Join(linkParts, "\n")
	if linksSection == "" {
		linksSection = "(not yet published)"
	}

	notes := ""
	if p != nil && p.Notes != "" {
		notes = p.Notes
	}

	complexity := b.Complexity
	if p != nil && p.Complexity != "" {
		complexity = p.Complexity
	}

	sourceContext := ""
	if b.SourceRole != "" || b.SourceCompany != "" {
		sourceContext = fmt.Sprintf("This addresses a skills gap seen in %s roles at companies like %s", b.SourceRole, b.SourceCompany)
	}

	prompt := fmt.Sprintf(`You are a LinkedIn content writer for a software engineer. Write a professional LinkedIn post (150-250 words) announcing completion of a portfolio project.

## Project Details
- Title: %s
- Problem it solves: %s
- Approach taken: %s
- Technologies: %s
- Complexity: %s
%s

## Project Links
%s

## Additional Notes
%s

## LinkedIn Angle
%s

## Writing Guidelines
- Professional but conversational tone, written in first person
- Open with a hook that names the specific problem solved, not "Excited to announce..."
- Show technical depth: mention one specific design decision or challenge overcome
- End with a call to engagement (question to the reader)
- 3-5 relevant hashtags at the end
- No em-dashes, no bullet-point lists, no "I'm thrilled" cliches
- Keep paragraphs short (2-3 sentences max)`,
		b.Title, b.ProblemStatement, b.SuggestedApproach,
		b.TechnologyStack, complexity,
		sourceContext,
		linksSection,
		notes,
		b.LinkedInAngle,
	)

	return g.call(prompt)
}

// GenerateFromProject creates a LinkedIn post draft from a manual project.
func (g *Generator) GenerateFromProject(p *models.Project) (string, error) {
	var linkParts []string
	if p.GiteaURL != "" {
		linkParts = append(linkParts, "- Repository: "+p.GiteaURL)
	}
	if p.LiveURL != "" {
		linkParts = append(linkParts, "- Live demo: "+p.LiveURL)
	}
	linksSection := strings.Join(linkParts, "\n")
	if linksSection == "" {
		linksSection = "(not yet published)"
	}

	prompt := fmt.Sprintf(`You are a LinkedIn content writer for a software engineer. Write a professional LinkedIn post (150-250 words) announcing completion of a portfolio project.

## Project Details
- Title: %s
- Complexity: %s

## Project Links
%s

## Additional Context
%s

## Writing Guidelines
- Professional but conversational tone, written in first person
- Open with a hook that names the specific problem solved, not "Excited to announce..."
- Show technical depth: mention one specific design decision or challenge overcome
- End with a call to engagement (question to the reader)
- 3-5 relevant hashtags at the end
- No em-dashes, no bullet-point lists, no "I'm thrilled" cliches
- Keep paragraphs short (2-3 sentences max)`,
		p.Title, p.Complexity,
		linksSection,
		p.Notes,
	)

	return g.call(prompt)
}

func (g *Generator) call(prompt string) (string, error) {
	body, err := json.Marshal(ollamaGenerateReq{
		Model:  g.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", ErrLLMUnavailable
	}

	resp, err := g.client.Post(g.endpoint+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("linkedin generator: endpoint %s unreachable: %v", g.endpoint, err)
		return "", ErrLLMUnavailable
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("linkedin generator: read response: %v", err)
		return "", ErrLLMUnavailable
	}

	var ollamaResp ollamaGenerateResp
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		log.Printf("linkedin generator: parse response: %v", err)
		return "", ErrLLMUnavailable
	}

	draft := strings.TrimSpace(ollamaResp.Response)
	if draft == "" {
		return "", ErrLLMUnavailable
	}

	return draft, nil
}
