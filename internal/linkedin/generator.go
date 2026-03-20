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

	"github.com/yourname/projctr/internal/models"
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

// Generate creates a LinkedIn post draft from a brief.
func (g *Generator) Generate(b *models.Brief) (string, error) {
	prompt := fmt.Sprintf(`Write a professional LinkedIn post (150-250 words) announcing that I built %s.

Problem it solves: %s
My approach: %s
Technologies used: %s
Angle: %s

Requirements:
- Professional but conversational tone
- First person
- 3-5 relevant hashtags at the end
- No em-dashes`,
		b.Title, b.ProblemStatement, b.SuggestedApproach,
		b.TechnologyStack, b.LinkedInAngle,
	)

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

// GenerateFromProject creates a LinkedIn post draft from a manual project's title and notes.
func (g *Generator) GenerateFromProject(title, notes string) (string, error) {
	prompt := fmt.Sprintf(`Write a professional LinkedIn post (150-250 words) announcing that I built %s.

Additional context: %s

Requirements:
- Professional but conversational tone
- First person
- 3-5 relevant hashtags at the end
- No em-dashes`,
		title, notes,
	)

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
