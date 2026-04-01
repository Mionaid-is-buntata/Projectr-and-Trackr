package extraction

import (
	"strings"

	"github.com/campbell/projctr/internal/models"
)

// Service orchestrates extraction using rules, LLM, or both.
type Service struct {
	Rules *RulesExtractor
	LLM   *LLMExtractor // nil if disabled
	Mode  string        // "rules", "llm", "both"
}

// NewService creates an extraction service with the given mode and extractors.
func NewService(mode string, rules *RulesExtractor, llm *LLMExtractor) *Service {
	return &Service{Rules: rules, LLM: llm, Mode: mode}
}

// Extract runs the configured extraction strategy on rawText.
func (s *Service) Extract(rawText string) ([]models.PainPoint, error) {
	switch s.Mode {
	case "llm":
		if s.LLM != nil {
			pts, err := s.LLM.Extract(rawText)
			if err == nil && len(pts) > 0 {
				return pts, nil
			}
		}
		// Fall back to rules if LLM unavailable
		return s.Rules.Extract(rawText)

	case "both":
		rulesPoints, _ := s.Rules.Extract(rawText)
		var llmPoints []models.PainPoint
		if s.LLM != nil {
			llmPoints, _ = s.LLM.Extract(rawText)
		}
		return mergePainPoints(rulesPoints, llmPoints), nil

	default: // "rules"
		return s.Rules.Extract(rawText)
	}
}

// ExtractTechnologies returns technology matches for the given text.
func (s *Service) ExtractTechnologies(text string) []models.Technology {
	return s.Rules.ExtractTechnologies(text)
}

// mergePainPoints combines rules and LLM pain points, deduping by challenge text prefix.
// Higher-confidence entry wins when there is a collision.
func mergePainPoints(rules, llm []models.PainPoint) []models.PainPoint {
	seen := make(map[string]models.PainPoint, len(rules)+len(llm))

	addAll := func(pts []models.PainPoint) {
		for _, p := range pts {
			key := dedupeKey(p.ChallengeText)
			if existing, ok := seen[key]; !ok || p.Confidence > existing.Confidence {
				seen[key] = p
			}
		}
	}

	addAll(rules)
	addAll(llm)

	out := make([]models.PainPoint, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	return out
}

func dedupeKey(text string) string {
	lower := strings.ToLower(strings.TrimSpace(text))
	if len(lower) > 20 {
		return lower[:20]
	}
	return lower
}
