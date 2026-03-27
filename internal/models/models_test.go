package models

import "testing"

func TestDisplayBriefTitle(t *testing.T) {
	tests := []struct {
		name    string
		briefID int64
		stored  string
		want    string
	}{
		{
			name:    "briefID 0 returns stored title unchanged",
			briefID: 0,
			stored:  "Some Title",
			want:    "Some Title",
		},
		{
			name:    "empty title returns Brief N",
			briefID: 5,
			stored:  "",
			want:    "Brief 5",
		},
		{
			name:    "Portfolio: X rewrites to Brief N: X",
			briefID: 3,
			stored:  "Portfolio: Cool Project",
			want:    "Brief 3: Cool Project",
		},
		{
			name:    "Portfolio Project rewrites to Brief N: Untitled idea",
			briefID: 7,
			stored:  "Portfolio Project",
			want:    "Brief 7: Untitled idea",
		},
		{
			name:    "already Brief N: X unchanged",
			briefID: 4,
			stored:  "Brief 4: Existing Title",
			want:    "Brief 4: Existing Title",
		},
		{
			name:    "normal title returned as-is",
			briefID: 2,
			stored:  "Build a REST API",
			want:    "Build a REST API",
		},
		{
			name:    "Portfolio: with empty rest rewrites to Untitled idea",
			briefID: 1,
			stored:  "Portfolio:  ",
			want:    "Brief 1: Untitled idea",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DisplayBriefTitle(tt.briefID, tt.stored)
			if got != tt.want {
				t.Errorf("DisplayBriefTitle(%d, %q) = %q, want %q", tt.briefID, tt.stored, got, tt.want)
			}
		})
	}
}

func TestProjectWithBrief_DisplayTitle(t *testing.T) {
	tests := []struct {
		name string
		pw   ProjectWithBrief
		want string
	}{
		{
			name: "uses BriefTitle when set",
			pw: ProjectWithBrief{
				Project:    Project{ID: 1, BriefID: 0, Title: "Project Title"},
				BriefTitle: "Brief Title",
			},
			want: "Brief Title",
		},
		{
			name: "falls back to project Title when BriefTitle empty",
			pw: ProjectWithBrief{
				Project:    Project{ID: 1, BriefID: 0, Title: "My Project"},
				BriefTitle: "",
			},
			want: "My Project",
		},
		{
			name: "returns Project #N when both empty",
			pw: ProjectWithBrief{
				Project:    Project{ID: 42, BriefID: 0, Title: ""},
				BriefTitle: "",
			},
			want: "Project #42",
		},
		{
			name: "applies DisplayBriefTitle rewrite when BriefID set",
			pw: ProjectWithBrief{
				Project:    Project{ID: 1, BriefID: 5, Title: "Fallback"},
				BriefTitle: "Portfolio: Something",
			},
			want: "Brief 5: Something",
		},
		{
			name: "BriefID set with normal title passes through",
			pw: ProjectWithBrief{
				Project:    Project{ID: 1, BriefID: 3, Title: "Fallback"},
				BriefTitle: "A Normal Title",
			},
			want: "A Normal Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pw.DisplayTitle()
			if got != tt.want {
				t.Errorf("DisplayTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProjectWithBrief_DisplayComplexity(t *testing.T) {
	tests := []struct {
		name string
		pw   ProjectWithBrief
		want string
	}{
		{
			name: "uses BriefComplexity when set",
			pw: ProjectWithBrief{
				Project:         Project{Complexity: "small"},
				BriefComplexity: "large",
			},
			want: "large",
		},
		{
			name: "falls back to project Complexity",
			pw: ProjectWithBrief{
				Project:         Project{Complexity: "medium"},
				BriefComplexity: "",
			},
			want: "medium",
		},
		{
			name: "both empty returns empty",
			pw: ProjectWithBrief{
				Project:         Project{Complexity: ""},
				BriefComplexity: "",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pw.DisplayComplexity()
			if got != tt.want {
				t.Errorf("DisplayComplexity() = %q, want %q", got, tt.want)
			}
		})
	}
}
