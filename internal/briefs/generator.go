package briefs

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
)

// ErrFrancisUnavailable is returned by Refine when the Francis LLM is not
// configured or could not be reached.
var ErrFrancisUnavailable = errors.New("Francis LLM unavailable")

// RefinedContent holds the fields updated when a brief is refined by Francis.
type RefinedContent struct {
	Title             string
	ProblemStatement  string
	SuggestedApproach string
	LinkedInAngle     string
	Complexity        string   // mapped from LLM difficulty_level; empty = keep existing
	ImpactScore       *float64 // mapped from LLM portfolio_value; nil = keep existing
	Source            string   // "local_llm" or "francis"
}

// Generator produces project briefs from clusters with ProjectLayout.
type Generator struct {
	clusters *repository.ClusterStore
	llm      *briefLLMClient // optional; nil means rule-based fallback
}

// NewGenerator creates a brief generator backed by cluster data.
func NewGenerator(clusters *repository.ClusterStore) *Generator {
	return &Generator{clusters: clusters}
}

// NewGeneratorWithLLM creates a brief generator that refines briefs via an LLM.
// source should be "local_llm" when using the local Ollama instance, or
// "francis" when using the Francis machine.
func NewGeneratorWithLLM(clusters *repository.ClusterStore, source, endpoint, model string) *Generator {
	return &Generator{
		clusters: clusters,
		llm:      newBriefLLMClient(source, endpoint, model),
	}
}

// CanRefine reports whether an LLM client is configured for brief refinement.
func (g *Generator) CanRefine() bool {
	return g.llm != nil
}

// Refine re-synthesises a brief's content from its cluster using the configured LLM.
// Returns ErrFrancisUnavailable if no LLM is configured or the endpoint is unreachable.
func (g *Generator) Refine(clusterID int64) (*RefinedContent, error) {
	if g.llm == nil {
		return nil, ErrFrancisUnavailable
	}

	cluster, err := g.clusters.GetByID(clusterID)
	if err != nil || cluster == nil {
		return nil, fmt.Errorf("cluster %d not found", clusterID)
	}

	painPoints, _ := g.clusters.PainPointsForCluster(clusterID)
	techs, _ := g.clusters.TechnologiesForCluster(clusterID)
	descs, _ := g.clusters.DescriptionsForCluster(clusterID)

	challenges := make([]string, 0, len(painPoints))
	for _, p := range painPoints {
		challenges = append(challenges, p.ChallengeText)
	}
	techNames := make([]string, 0, len(techs))
	for _, t := range techs {
		techNames = append(techNames, t.Name)
	}
	roles := make([]string, 0, len(descs))
	for _, d := range descs {
		if d.RoleTitle != "" {
			roles = append(roles, d.RoleTitle)
		}
	}

	draft := g.llm.refine(challenges, techNames, cluster.GapType, cluster.Frequency, roles)
	if draft == nil {
		return nil, ErrFrancisUnavailable
	}

	title := draft.Title
	if strings.TrimSpace(title) == "" {
		title = cluster.Summary
	}

	var impactScore *float64
	if draft.PortfolioValue > 0 {
		v := draft.PortfolioValue
		if v > 1 {
			v = 1
		}
		impactScore = &v
	}

	return &RefinedContent{
		Title:             title,
		ProblemStatement:  draft.ProblemStatement,
		SuggestedApproach: draft.SuggestedApproach,
		LinkedInAngle:     draft.LinkedInAngle,
		Complexity:        difficultyToComplexity(draft.DifficultyLevel),
		ImpactScore:       impactScore,
		Source:            g.llm.source,
	}, nil
}

// TitleMaxRunes is the maximum length of the summary portion in a brief title (before "Brief N: " prefix).
const TitleMaxRunes = 240

// GenerateFromCluster creates a Brief from a cluster, including ProjectLayout.
// When an LLM client is configured it synthesises a focused project brief;
// otherwise it falls back to rule-based generation.
func (g *Generator) GenerateFromCluster(c *models.Cluster) *models.Brief {
	now := time.Now()
	techStack := g.deriveTechStack(c)
	complexity := g.deriveComplexity(c)
	layout := g.buildProjectLayout(complexity, techStack)

	// Defaults from rule-based path.
	problemStatement := c.Summary
	suggestedApproach := g.deriveApproach(c)
	linkedInAngle := g.deriveLinkedInAngle(c)
	title := g.TitleBody(c.Summary)
	generationSource := "rules"

	// Attempt LLM refinement when a client is wired in.
	if g.llm != nil {
		painPoints, _ := g.clusters.PainPointsForCluster(c.ID)
		techs, _ := g.clusters.TechnologiesForCluster(c.ID)
		descs, _ := g.clusters.DescriptionsForCluster(c.ID)

		challenges := make([]string, 0, len(painPoints))
		for _, p := range painPoints {
			challenges = append(challenges, p.ChallengeText)
		}
		techNames := make([]string, 0, len(techs))
		for _, t := range techs {
			techNames = append(techNames, t.Name)
		}
		roles := make([]string, 0, len(descs))
		for _, d := range descs {
			if d.RoleTitle != "" {
				roles = append(roles, d.RoleTitle)
			}
		}

		if draft := g.llm.refine(challenges, techNames, c.GapType, c.Frequency, roles); draft != nil {
			if draft.Title != "" {
				title = draft.Title
			}
			problemStatement = draft.ProblemStatement
			suggestedApproach = draft.SuggestedApproach
			if draft.LinkedInAngle != "" {
				linkedInAngle = draft.LinkedInAngle
			}
			if mapped := difficultyToComplexity(draft.DifficultyLevel); mapped != "" {
				complexity = mapped
			}
			generationSource = g.llm.source
		}
	}

	brief := &models.Brief{
		ClusterID:         c.ID,
		Title:             title,
		ProblemStatement:  problemStatement,
		SuggestedApproach: suggestedApproach,
		TechnologyStack:   techStack,
		ProjectLayout:     layout,
		Complexity:        complexity,
		ImpactScore:       c.GapScore,
		LinkedInAngle:     linkedInAngle,
		GenerationSource:  generationSource,
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
	steps = append(steps, fmt.Sprintf("1. Core build: %s", truncate(painPoints[0].ChallengeText, 100)))
	if len(painPoints) > 1 {
		steps = append(steps, fmt.Sprintf("2. Extend with: %s", truncate(painPoints[1].ChallengeText, 100)))
	}
	if len(painPoints) > 2 {
		steps = append(steps, fmt.Sprintf("3. Integrate: %s", truncate(painPoints[2].ChallengeText, 100)))
	}
	steps = append(steps, fmt.Sprintf("%d. Write tests, README, and a short demo", len(steps)+1))
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

func difficultyToComplexity(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "beginner":
		return "small"
	case "intermediate":
		return "medium"
	case "advanced":
		return "large"
	}
	return ""
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
