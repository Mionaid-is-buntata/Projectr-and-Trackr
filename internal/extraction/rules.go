package extraction

import (
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/campbell/projctr/internal/models"
)

// triggerPhrases indicate a sentence is expressing a technical requirement or challenge.
var triggerPhrases = []string{
	"experience with", "experience in", "proficiency in", "knowledge of",
	"familiarity with", "understanding of", "expertise in", "strong background in",
	"hands-on experience", "deep understanding", "must have", "required",
	"essential", "you will need", "we require", "we are looking for",
	"you should have", "you must have", "we expect", "proven experience",
	"solid understanding", "working knowledge", "comfortable with",
	"demonstrable experience", "track record",
}

// RulesExtractor implements pain point extraction using keyword dictionaries
// and pattern matching. No external dependencies — works offline.
type RulesExtractor struct {
	dict []TechEntry
}

type techDictFile struct {
	Technologies []TechEntry `toml:"technologies"`
}

// NewRulesExtractor loads the technology keyword dictionary from the given path.
func NewRulesExtractor(dictPath string) (*RulesExtractor, error) {
	var f techDictFile
	if _, err := toml.DecodeFile(dictPath, &f); err != nil {
		return nil, err
	}
	return &RulesExtractor{dict: f.Technologies}, nil
}

// Extract analyses a raw job description and returns structured pain points.
func (r *RulesExtractor) Extract(rawText string) ([]models.PainPoint, error) {
	sentences := splitSentences(rawText)
	seen := map[string]models.PainPoint{} // keyed by first-tech canonical name or prefix

	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) < 10 {
			continue
		}
		lower := strings.ToLower(sentence)

		hasTrigger := false
		for _, phrase := range triggerPhrases {
			if strings.Contains(lower, phrase) {
				hasTrigger = true
				break
			}
		}

		techs := findTechsInText(sentence, r.dict)

		if !hasTrigger && len(techs) == 0 {
			continue
		}

		// Determine confidence
		confidence := 0.4
		if hasTrigger && len(techs) > 0 {
			confidence = 0.9
		} else if hasTrigger {
			confidence = 0.6
		}

		domain := "general"
		techKey := ""
		if len(techs) > 0 {
			domain = techs[0].Category
			techKey = techs[0].Canonical
		}

		pp := models.PainPoint{
			ChallengeText: sentence,
			Domain:        domain,
			OutcomeText:   extractOutcome(lower),
			Confidence:    confidence,
			DateExtracted: time.Now(),
		}

		// Dedup within description: keep higher confidence for same first-tech key
		key := techKey
		if key == "" {
			l := len(lower)
			if l > 30 {
				l = 30
			}
			key = lower[:l]
		}
		if existing, ok := seen[key]; !ok || pp.Confidence > existing.Confidence {
			seen[key] = pp
		}
	}

	out := make([]models.PainPoint, 0, len(seen))
	for _, pp := range seen {
		out = append(out, pp)
	}
	return out, nil
}

// ExtractTechnologies returns the tech dict entries matched in the given text.
// Used by the pipeline to link technologies to stored pain points.
func (r *RulesExtractor) ExtractTechnologies(text string) []models.Technology {
	entries := findTechsInText(text, r.dict)
	out := make([]models.Technology, 0, len(entries))
	for _, e := range entries {
		out = append(out, models.Technology{Name: e.Canonical, Category: e.Category})
	}
	return out
}

// splitSentences splits text into sentences on ., \n, ; and bullet markers.
func splitSentences(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var sentences []string
	current := strings.Builder{}
	runes := []rune(text)
	for i, r := range runes {
		current.WriteRune(r)
		end := false
		switch r {
		case '\n', ';':
			end = true
		case '.':
			if i+1 < len(runes) {
				next := runes[i+1]
				if next == ' ' || next == '\n' {
					end = true
				}
			} else {
				end = true
			}
		}
		if end {
			s := strings.TrimSpace(current.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

// extractOutcome looks for benefit/outcome phrases in a sentence.
func extractOutcome(lower string) string {
	phrases := []string{"to enable", "to support", "to improve", "in order to", "to help", "to build", "to deliver"}
	for _, p := range phrases {
		if idx := strings.Index(lower, p); idx >= 0 {
			end := idx + 80
			if end > len(lower) {
				end = len(lower)
			}
			return strings.TrimSpace(lower[idx:end])
		}
	}
	return ""
}
