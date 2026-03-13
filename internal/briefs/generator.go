package briefs

import (
	"fmt"
	"strings"
	"time"

	"github.com/yourname/projctr/internal/models"
)

// Generator produces project briefs from clusters with ProjectLayout.
type Generator struct{}

// NewGenerator creates a brief generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// GenerateFromCluster creates a Brief from a cluster, including ProjectLayout.
func (g *Generator) GenerateFromCluster(c *models.Cluster) *models.Brief {
	now := time.Now()
	title := g.deriveTitle(c)
	techStack := g.deriveTechStack(c)
	complexity := g.deriveComplexity(c)
	layout := g.buildProjectLayout(complexity, techStack)

	return &models.Brief{
		ClusterID:         c.ID,
		Title:             title,
		ProblemStatement:  c.Summary,
		SuggestedApproach: g.deriveApproach(c),
		TechnologyStack:   techStack,
		ProjectLayout:    layout,
		Complexity:        complexity,
		ImpactScore:       c.GapScore,
		LinkedInAngle:     g.deriveLinkedInAngle(c),
		IsEdited:          false,
		DateGenerated:     now,
		DateModified:      nil,
	}
}

func (g *Generator) deriveTitle(c *models.Cluster) string {
	if c.Summary != "" {
		// Use first ~50 chars of summary as base
		s := strings.TrimSpace(c.Summary)
		if len(s) > 50 {
			s = s[:47] + "..."
		}
		return "Portfolio: " + s
	}
	return "Portfolio Project"
}

func (g *Generator) deriveTechStack(c *models.Cluster) string {
	// Placeholder: derive from gap_type or use generic
	switch c.GapType {
	case "skill_extension":
		return `["Go", "REST API", "SQLite"]`
	case "skill_acquisition":
		return `["Python", "FastAPI", "PostgreSQL"]`
	case "domain_expansion":
		return `["TypeScript", "React", "Node.js"]`
	default:
		return `["Go", "REST API"]`
	}
}

func (g *Generator) deriveComplexity(c *models.Cluster) string {
	if c.Frequency >= 10 {
		return "large"
	}
	if c.Frequency >= 5 {
		return "medium"
	}
	return "small"
}

func (g *Generator) deriveApproach(c *models.Cluster) string {
	return "1. Define API contract\n2. Implement core logic\n3. Add tests\n4. Document and deploy"
}

func (g *Generator) deriveLinkedInAngle(c *models.Cluster) string {
	return fmt.Sprintf("Demonstrates %s skills from %d job postings", c.GapType, c.Frequency)
}

func (g *Generator) buildProjectLayout(complexity, techStack string) string {
	switch complexity {
	case "large":
		return "# Project Layout (Large)\n\n" +
			"## Directory Structure\n\n" +
			"```\ncmd/\n  server/     # Entry point\ninternal/\n  handlers/   # HTTP handlers\n  service/    # Business logic\n  repository/ # Data access\npkg/\n  models/     # Shared types\nconfigs/\ntests/\n  integration/\n```\n\n" +
			"## Entry Point\n- cmd/server/main.go\n\n" +
			"## Key Files\n- internal/handlers/routes.go\n- internal/service/core.go\n- configs/config.toml\n"
	case "medium":
		return "# Project Layout (Medium)\n\n" +
			"## Directory Structure\n\n" +
			"```\ncmd/\n  main.go     # Entry point\ninternal/\n  handlers/\n  service/\nconfig/\n```\n\n" +
			"## Entry Point\n- cmd/main.go\n\n" +
			"## Key Files\n- internal/handlers/routes.go\n- internal/service/service.go\n"
	default:
		return "# Project Layout (Small)\n\n" +
			"## Directory Structure\n\n" +
			"```\n.\n  main.go     # Entry point\n  internal/\n    logic.go\n```\n\n" +
			"## Entry Point\n- main.go\n\n" +
			"## Key Files\n- main.go\n- internal/logic.go\n"
	}
}
