package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ContentHash produces a SHA-256 hash of normalised description text.
// Used for exact-match deduplication.
func ContentHash(text string) string {
	normalised := normalise(text)
	sum := sha256.Sum256([]byte(normalised))
	return hex.EncodeToString(sum[:])
}

// normalise lowercases text and collapses runs of whitespace.
// Extend with boilerplate stripping as patterns emerge from real data.
func normalise(text string) string {
	text = strings.ToLower(text)
	text = strings.Join(strings.Fields(text), " ")
	return text
}
