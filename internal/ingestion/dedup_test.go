package ingestion

import "testing"

func TestContentHash(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect func(hash string) bool
		desc   string
	}{
		{
			name:  "deterministic same input same output",
			input: "hello world",
			expect: func(hash string) bool {
				return hash == ContentHash("hello world")
			},
			desc: "same input should produce same hash",
		},
		{
			name:  "whitespace normalisation",
			input: "  hello  world  ",
			expect: func(hash string) bool {
				return hash == ContentHash("hello world")
			},
			desc: "extra whitespace should be normalised",
		},
		{
			name:  "case normalisation",
			input: "HELLO",
			expect: func(hash string) bool {
				return hash == ContentHash("hello")
			},
			desc: "case should be normalised",
		},
		{
			name:  "different text produces different hash",
			input: "alpha",
			expect: func(hash string) bool {
				return hash != ContentHash("beta")
			},
			desc: "different text should have different hashes",
		},
		{
			name:  "empty string produces a hash",
			input: "",
			expect: func(hash string) bool {
				return len(hash) == 64 // SHA-256 hex is 64 chars
			},
			desc: "empty string should produce valid hex hash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContentHash(tt.input)
			if !tt.expect(got) {
				t.Errorf("ContentHash(%q) = %q — %s", tt.input, got, tt.desc)
			}
		})
	}
}

func TestNormalise(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "lowercase",
			input: "Hello World",
			want:  "hello world",
		},
		{
			name:  "collapse whitespace",
			input: "  foo   bar  baz  ",
			want:  "foo bar baz",
		},
		{
			name:  "tabs and newlines collapsed",
			input: "line1\t\tline2\nline3",
			want:  "line1 line2 line3",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "already normalised",
			input: "already clean",
			want:  "already clean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalise(tt.input)
			if got != tt.want {
				t.Errorf("normalise(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
