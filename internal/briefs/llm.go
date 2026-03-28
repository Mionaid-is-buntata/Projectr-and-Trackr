package briefs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type ollamaGenerateReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaGenerateResp struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type llmBriefDraft struct {
	Title             string  `json:"title"`
	ProblemStatement  string  `json:"problem_statement"`
	SuggestedApproach string  `json:"suggested_approach"`
	LinkedInAngle     string  `json:"linkedin_angle"`
	DifficultyLevel   string  `json:"difficulty_level"`   // beginner | intermediate | advanced
	PortfolioValue    float64 `json:"portfolio_value"`    // 0.0-1.0
}

type briefLLMClient struct {
	// source is recorded on the brief after a successful call: "local_llm" or "francis".
	source   string
	endpoint string
	model    string
	client   *http.Client
}

// dialTimeout is the maximum time to establish a TCP connection to Francis.
// Kept short so an offline Francis fails fast rather than blocking brief generation.
const dialTimeout = 5 * time.Second

func newBriefLLMClient(source, endpoint, model string) *briefLLMClient {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return (&net.Dialer{Timeout: dialTimeout}).DialContext(ctx, network, addr)
		},
	}
	return &briefLLMClient{
		source:   source,
		endpoint: endpoint,
		model:    model,
		// No overall Timeout so long-running generation is not cut short,
		// but the dial step fails fast if Francis is unreachable.
		client: &http.Client{Transport: transport},
	}
}

// refine calls the LLM to synthesise a focused project brief from cluster context.
// Returns nil if the endpoint is unreachable or the response cannot be parsed.
func (c *briefLLMClient) refine(painPoints []string, techs []string, gapType string, frequency int, sourceRoles []string) *llmBriefDraft {
	challengeList := formatList(painPoints, 6)
	techList := strings.Join(unique(techs, 8), ", ")
	rolesCtx := ""
	if len(sourceRoles) > 0 {
		rolesCtx = fmt.Sprintf("Roles signalling this gap: %s.\n", strings.Join(unique(sourceRoles, 4), ", "))
	}

	prompt := fmt.Sprintf(`You are a senior systems architect helping a developer build a focused portfolio project that directly addresses real industry demand.

You have been given context from %d job postings that all signal the same engineering gap.
%sGap type: %s
Technologies involved: %s

The following engineering challenges were inferred from those job descriptions:
%s

Your task: synthesise these into ONE sharp, buildable portfolio project.

Rules:
- Solve a specific latent engineering problem — not a vague "build an app with X"
- Avoid resume language and buzzwords — the project title and problem statement must name the actual system challenge
- Focus on system design, scalability, reliability, or integration — the kind of problem that reveals engineering depth
- A developer should be able to complete it in 2-4 weeks and have something demonstrable
- The suggested approach must be concrete numbered steps: what to build first, what to wire up second, etc.
- Penalise surface-level extractions: "Build a REST API" is too generic; "Build a rate-limited API gateway with per-tenant circuit breaking" is what you want

Return ONLY a JSON object with these fields:
- "title": a concise project title (max 10 words) that names the problem, not the tech stack
- "problem_statement": 2-3 sentences describing the specific real-world engineering problem this project solves
- "suggested_approach": numbered build steps as a single string with \n separators (4-6 steps, each starting with a concrete action verb)
- "linkedin_angle": one sentence framing this as a market signal — name the specific problem and what the project proves
- "difficulty_level": one of "beginner", "intermediate", or "advanced" based on the depth of system knowledge required
- "portfolio_value": float 0.0-1.0 — how strongly this project differentiates a candidate for roles signalling this gap

JSON output:`,
		frequency, rolesCtx, gapType, techList, challengeList,
	)

	body, err := json.Marshal(ollamaGenerateReq{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return nil
	}

	resp, err := c.client.Post(c.endpoint+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("briefs llm: endpoint %s unreachable: %v", c.endpoint, err)
		return nil
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("briefs llm: read response: %v", err)
		return nil
	}

	var ollamaResp ollamaGenerateResp
	if err := json.Unmarshal(raw, &ollamaResp); err != nil {
		log.Printf("briefs llm: parse ollama response: %v", err)
		return nil
	}

	jsonStr := extractJSONObject(ollamaResp.Response)
	if jsonStr == "" {
		log.Printf("briefs llm: no JSON object in response (%.200s)", ollamaResp.Response)
		return nil
	}

	var draft llmBriefDraft
	if err := json.Unmarshal([]byte(jsonStr), &draft); err != nil {
		log.Printf("briefs llm: parse draft JSON: %v (raw: %.200s)", err, jsonStr)
		return nil
	}

	if strings.TrimSpace(draft.ProblemStatement) == "" || strings.TrimSpace(draft.SuggestedApproach) == "" {
		return nil
	}

	return &draft
}

// extractJSONObject strips markdown fences and finds the { ... } object boundary.
func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if nl := strings.Index(s, "\n"); nl >= 0 {
			if end := strings.LastIndex(s, "```"); end > nl {
				s = s[nl+1 : end]
			}
		}
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}

func formatList(items []string, max int) string {
	var sb strings.Builder
	for i, item := range items {
		if i >= max {
			break
		}
		fmt.Fprintf(&sb, "- %s\n", item)
	}
	return sb.String()
}

func unique(items []string, max int) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range items {
		if s = strings.TrimSpace(s); s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}
