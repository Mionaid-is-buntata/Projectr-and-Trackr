package extraction

import "testing"

var testDict = []TechEntry{
	{Canonical: "Python", Category: "language", Variants: []string{"python", "python3", "py"}},
	{Canonical: "Go", Category: "language", Variants: []string{"go", "golang"}},
	{Canonical: "Node.js", Category: "framework", Variants: []string{"node.js", "nodejs", "node js"}},
	{Canonical: "C++", Category: "language", Variants: []string{"c++", "cpp"}},
	{Canonical: "C#", Category: "language", Variants: []string{"c#", "csharp"}},
}

func TestNormaliseTech(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "exact variant match returns canonical",
			raw:  "python3",
			want: "Python",
		},
		{
			name: "case insensitive",
			raw:  "PYTHON",
			want: "Python",
		},
		{
			name: "unknown tech returns raw string",
			raw:  "Haskell",
			want: "Haskell",
		},
		{
			name: "trimming whitespace",
			raw:  "  golang  ",
			want: "Go",
		},
		{
			name: "golang variant",
			raw:  "golang",
			want: "Go",
		},
		{
			name: "c++ variant",
			raw:  "cpp",
			want: "C++",
		},
		{
			name: "c# variant",
			raw:  "csharp",
			want: "C#",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormaliseTech(tt.raw, testDict)
			if got != tt.want {
				t.Errorf("NormaliseTech(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestTokenise(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "splits on spaces",
			input: "hello world",
			want:  []string{"hello", "world"},
		},
		{
			name:  "preserves C++",
			input: "I use c++ daily",
			want:  []string{"I", "use", "c++", "daily"},
		},
		{
			name:  "preserves C#",
			input: "writing c# code",
			want:  []string{"writing", "c#", "code"},
		},
		{
			name:  "preserves .NET",
			input: "using .net framework",
			want:  []string{"using", ".net", "framework"},
		},
		{
			name:  "handles punctuation",
			input: "python, go, and rust",
			want:  []string{"python", "go", "and", "rust"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenise(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenise(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("tokenise(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindTechsInText(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantKeys []string // expected canonical names
	}{
		{
			name:     "multi-word variant node.js matched",
			text:     "We use node js for the backend",
			wantKeys: []string{"Node.js"},
		},
		{
			name:     "single token python matched",
			text:     "Experience with python required",
			wantKeys: []string{"Python"},
		},
		{
			name:     "dedupes by canonical name",
			text:     "python python3 py scripting",
			wantKeys: []string{"Python"},
		},
		{
			name:     "multiple techs found",
			text:     "Build microservices in golang with python scripts",
			wantKeys: []string{"Go", "Python"},
		},
		{
			name:     "no matches returns empty",
			text:     "no technology mentioned here",
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findTechsInText(tt.text, testDict)

			gotMap := make(map[string]bool)
			for _, e := range got {
				gotMap[e.Canonical] = true
			}

			if len(tt.wantKeys) == 0 {
				if len(got) != 0 {
					t.Errorf("findTechsInText(%q) returned %d entries, want 0", tt.text, len(got))
				}
				return
			}

			if len(got) != len(tt.wantKeys) {
				t.Fatalf("findTechsInText(%q) returned %d entries, want %d: got %v", tt.text, len(got), len(tt.wantKeys), gotMap)
			}

			for _, key := range tt.wantKeys {
				if !gotMap[key] {
					t.Errorf("findTechsInText(%q) missing %q, got %v", tt.text, key, gotMap)
				}
			}
		})
	}
}
