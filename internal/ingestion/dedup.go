package ingestion

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ContentHash produces a SHA-256 hash of normalised description text.
// Used for exact-match deduplication. Cross-board near-duplicates are
// handled separately via embedding similarity (see FuzzyDeduplicate).
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

// FuzzyDeduplicate returns true if the given embedding is within the
// similarity threshold of any existing description embedding in Qdrant.
// Threshold from 05-data-flow-and-integration.md §5.2: cosine similarity > 0.95.
func FuzzyDeduplicate(embedding []float32 /* , qdrantClient *vectordb.Client */) (bool, error) {
	// TODO: query Qdrant for nearest neighbour; return true if similarity > 0.95
	return false, nil
}
