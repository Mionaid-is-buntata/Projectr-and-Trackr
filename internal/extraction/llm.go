package extraction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/campbell/projctr/internal/models"
)

// LLMExtractor implements pain point extraction using an LLM via Ollama.
// Falls back to an empty result (not an error) if the endpoint is unreachable.
type LLMExtractor struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewLLMExtractor creates an extractor that calls the configured LLM endpoint.
func NewLLMExtractor(endpoint, model string) *LLMExtractor {
	return &LLMExtractor{
		endpoint: endpoint,
		model:    model,
		client:   &http.Client{Timeout: 60 * time.Second},
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

type llmPainPoint struct {
	ChallengeText string   `json:"challenge_text"`
	Domain        string   `json:"domain"`
	OutcomeText   string   `json:"outcome_text"`
	Technologies  []string `json:"technologies"`
	Confidence    float64  `json:"confidence"`
}

// Extract sends rawText to the LLM and parses the structured response.
// Returns (nil, nil) if the endpoint is unreachable or the response is unparseable.
func (l *LLMExtractor) Extract(rawText string) ([]models.PainPoint, error) {
	text := rawText
	if len(text) > 3000 {
		text = text[:3000]
	}

	prompt := fmt.Sprintf(`You are a senior systems architect reviewing a job description to identify latent engineering challenges.

Work through three steps internally before writing output:

Step 1 — Extract signals: identify the tech stack, core responsibilities, implicit constraints, and anything the role needs to handle but does not explicitly name.
Step 2 — Infer problems: for each responsibility, ask "what engineering problem does this actually require solving?" Focus on system design, scalability, reliability, and integration challenges — not skills lists.
Step 3 — Reframe as project ideas: turn each inferred problem into a concrete, buildable project a developer could complete to demonstrate mastery.

Output rules:
- challenge_text must describe a specific engineering problem to solve, written as "Build a..." or "Create a..." — NEVER repeat the job requirement verbatim
- Penalise resume language: "experience with X" or "familiarity with Y" are NOT valid challenge_text values
- Prefer latent problems over obvious ones ("Handling distributed API consistency under unreliable networks" beats "Build a REST API")
- Ignore soft skills, degree requirements, and generic requirements

Return ONLY a JSON array. Each element must have:
- "challenge_text": the concrete project idea (one sentence)
- "domain": one of: language, framework, platform, tool, database, methodology, general
- "outcome_text": what this project proves to a hiring manager (one sentence)
- "technologies": array of technology names the project would use
- "confidence": float 0.0-1.0 — how strongly the JD signals this as a real unsolved problem vs a checkbox requirement

Job description:
"""
%s
"""

JSON output:`, text)

	body, err := json.Marshal(ollamaGenerateReq{
		Model:  l.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return nil, nil
	}

	resp, err := l.client.Post(l.endpoint+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("llm extractor: endpoint %s unreachable: %v", l.endpoint, err)
		return nil, nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("llm extractor: read response: %v", err)
		return nil, nil
	}

	var ollamaResp ollamaGenerateResp
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		log.Printf("llm extractor: parse ollama response: %v", err)
		return nil, nil
	}

	jsonStr := extractJSONArray(ollamaResp.Response)
	if jsonStr == "" {
		log.Printf("llm extractor: no JSON array in response")
		return nil, nil
	}

	var llmPoints []llmPainPoint
	if err := json.Unmarshal([]byte(jsonStr), &llmPoints); err != nil {
		log.Printf("llm extractor: parse pain points JSON: %v (raw: %.200s)", err, jsonStr)
		return nil, nil
	}

	now := time.Now()
	out := make([]models.PainPoint, 0, len(llmPoints))
	for _, lp := range llmPoints {
		if strings.TrimSpace(lp.ChallengeText) == "" {
			continue
		}
		out = append(out, models.PainPoint{
			ChallengeText: lp.ChallengeText,
			Domain:        normaliseDomain(lp.Domain),
			OutcomeText:   lp.OutcomeText,
			Confidence:    clampConfidence(lp.Confidence),
			DateExtracted: now,
		})
	}
	return out, nil
}

// extractJSONArray strips markdown fences and finds the [ ... ] array boundary.
func extractJSONArray(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl >= 0 {
			if end := strings.LastIndex(s, "```"); end > nl {
				s = s[nl+1 : end]
			}
		}
	}
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func normaliseDomain(d string) string {
	valid := map[string]bool{
		"language": true, "framework": true, "platform": true,
		"tool": true, "database": true, "methodology": true, "general": true,
	}
	d = strings.ToLower(strings.TrimSpace(d))
	if valid[d] {
		return d
	}
	return "general"
}

func clampConfidence(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
