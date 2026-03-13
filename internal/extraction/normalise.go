package extraction

import (
	"strings"
	"unicode"
)

// NormaliseTech maps a raw technology mention to its canonical name using
// the loaded dictionary. Returns the raw string unchanged if no match is found.
// Matching is case-insensitive. Variants are defined in tech-dict.toml.
// Example: "JS", "javascript", "node.js" → "JavaScript"
func NormaliseTech(raw string, dict []TechEntry) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	for _, entry := range dict {
		for _, v := range entry.Variants {
			if strings.ToLower(v) == lower {
				return entry.Canonical
			}
		}
	}
	return raw
}

// TechEntry represents one entry from tech-dict.toml.
type TechEntry struct {
	Canonical string   `toml:"canonical"`
	Category  string   `toml:"category"`
	Variants  []string `toml:"variants"`
}

// tokenise splits text into word tokens, preserving +, #, . for C++/C#/etc.
func tokenise(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) &&
			r != '+' && r != '#' && r != '.'
	})
}

// findTechsInText scans text for tech dict matches, returning matched entries.
// Multi-word variants are checked first via substring, then single tokens.
func findTechsInText(text string, dict []TechEntry) []TechEntry {
	lower := strings.ToLower(text)
	seen := map[string]bool{}
	var result []TechEntry

	for _, entry := range dict {
		if seen[entry.Canonical] {
			continue
		}
		matched := false
		// Multi-word variants: substring match on full lowercased text
		for _, v := range entry.Variants {
			lv := strings.ToLower(v)
			if strings.Contains(lv, " ") {
				if strings.Contains(lower, lv) {
					matched = true
					break
				}
			}
		}
		// Single-token variants: exact token match
		if !matched {
			tokens := tokenise(lower)
			for _, tok := range tokens {
				for _, v := range entry.Variants {
					if strings.ToLower(v) == tok {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
		}
		if matched {
			seen[entry.Canonical] = true
			result = append(result, entry)
		}
	}
	return result
}
