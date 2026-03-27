package briefs

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yourname/projctr/internal/models"
	"github.com/yourname/projctr/internal/repository"
)

// Generator produces project briefs from clusters with ProjectLayout.
type Generator struct {
	clusters *repository.ClusterStore
}

// NewGenerator creates a brief generator backed by cluster data.
func NewGenerator(clusters *repository.ClusterStore) *Generator {
	return &Generator{clusters: clusters}
}

// TitleMaxRunes is the maximum length of the summary portion in a brief title (before "Brief N: " prefix).
const TitleMaxRunes = 240

// GenerateFromCluster creates a Brief from a cluster, including ProjectLayout.
func (g *Generator) GenerateFromCluster(c *models.Cluster) *models.Brief {
	now := time.Now()
	// Placeholder until DB assigns id; FinalizeBriefTitle applied after insert.
	title := g.TitleBody(c.Summary)
	techStack := g.deriveTechStack(c)
	complexity := g.deriveComplexity(c)
	layout := g.buildProjectLayout(complexity, techStack)

	brief := &models.Brief{
		ClusterID:         c.ID,
		Title:             title,
		ProblemStatement:  c.Summary,
		SuggestedApproach: g.deriveApproach(c),
		TechnologyStack:   techStack,
		ProjectLayout:     layout,
		Complexity:        complexity,
		ImpactScore:       c.GapScore,
		LinkedInAngle:     g.deriveLinkedInAngle(c),
		IsEdited:          false,
		DateGenerated:     now,
		DateModified:      nil,
	}

	// Populate source fields from the cluster's originating job descriptions.
	descs, _ := g.clusters.DescriptionsForCluster(c.ID)
	if len(descs) > 0 {
		brief.SourceCompany = descs[0].Sector
		brief.SourceRole = descs[0].RoleTitle
	}

	return brief
}

// TitleBody returns a trimmed summary suitable for embedding in a brief title (length-capped).
func (g *Generator) TitleBody(summary string) string {
	s := strings.TrimSpace(summary)
	if s == "" {
		return "Untitled idea"
	}
	runes := []rune(s)
	if len(runes) <= TitleMaxRunes {
		return s
	}
	return string(runes[:TitleMaxRunes-1]) + "…"
}

// FinalizeBriefTitle sets the stored title after insert when the brief id is known.
func (g *Generator) FinalizeBriefTitle(briefID int64, problemStatement string) string {
	return fmt.Sprintf("Brief %d: %s", briefID, g.TitleBody(problemStatement))
}

func (g *Generator) deriveTechStack(c *models.Cluster) string {
	techs, err := g.clusters.TechnologiesForCluster(c.ID)
	if err != nil || len(techs) == 0 {
		return g.fallbackTechStack(c.GapType)
	}
	names := make([]string, 0, len(techs))
	for _, t := range techs {
		names = append(names, t.Name)
	}
	if len(names) > 8 {
		names = names[:8]
	}
	b, _ := json.Marshal(names)
	return string(b)
}

func (g *Generator) fallbackTechStack(gapType string) string {
	switch gapType {
	case "skill_extension":
		return `["Go","REST API","SQLite"]`
	case "skill_acquisition":
		return `["Go","Docker","PostgreSQL"]`
	case "domain_expansion":
		return `["TypeScript","React","Node.js"]`
	default:
		return `["Go","REST API"]`
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
	painPoints, err := g.clusters.PainPointsForCluster(c.ID)
	if err != nil || len(painPoints) == 0 {
		return "1. Research the problem domain\n2. Design solution architecture\n3. Implement core functionality\n4. Add tests and documentation"
	}
	var steps []string
	steps = append(steps, fmt.Sprintf("1. Address: %s", truncate(painPoints[0].ChallengeText, 100)))
	if len(painPoints) > 1 {
		steps = append(steps, fmt.Sprintf("2. Solve: %s", truncate(painPoints[1].ChallengeText, 100)))
	}
	if len(painPoints) > 2 {
		steps = append(steps, fmt.Sprintf("3. Handle: %s", truncate(painPoints[2].ChallengeText, 100)))
	}
	steps = append(steps, fmt.Sprintf("%d. Add tests and documentation", len(steps)+1))
	return strings.Join(steps, "\n")
}

func (g *Generator) deriveLinkedInAngle(c *models.Cluster) string {
	techs, _ := g.clusters.TechnologiesForCluster(c.ID)
	var techNames []string
	for _, t := range techs {
		if len(techNames) >= 3 {
			break
		}
		techNames = append(techNames, t.Name)
	}
	techStr := strings.Join(techNames, ", ")
	if techStr == "" {
		techStr = c.GapType
	}
	return fmt.Sprintf(
		"Solving a real hiring gap: %d+ job postings need %s skills — this project proves hands-on capability",
		c.Frequency, techStr,
	)
}

func truncate(s string, maxLen int) string {
	runes := []rune(strings.TrimSpace(s))
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen-1]) + "…"
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
