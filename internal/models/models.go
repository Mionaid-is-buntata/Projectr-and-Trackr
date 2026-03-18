package models

import "time"

// ScoredJob matches the Huntr jobs/scored/*.json schema.
// Used when reading from Huntr's JSON files.
type ScoredJob struct {
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Location    string   `json:"location"`
	WorkType    string   `json:"work_type"`
	Salary      string   `json:"salary"`
	SalaryNum   float64  `json:"salary_num"`
	Description string   `json:"description"`
	Skills      string   `json:"skills"`
	Link        string   `json:"link"`
	Source      string   `json:"source"`
	Score       float64  `json:"score"`
	ScoreBreakdown *ScoreBreakdown `json:"score_breakdown,omitempty"`
}

// ScoreBreakdown holds Huntr's scoring breakdown.
type ScoreBreakdown struct {
	TechStackScore float64 `json:"tech_stack_score"`
	DomainScore    float64 `json:"domain_score"`
	LocationScore  float64 `json:"location_score"`
	SalaryScore    float64 `json:"salary_score"`
}

// Description is a job description ingested from Huntr.
type Description struct {
	ID           int64
	HuntrID      string
	RoleTitle    string
	Sector       string
	SalaryMin    *int
	SalaryMax    *int
	Location     string
	SourceBoard  string
	HuntrScore   float64
	RawText      string
	DateScraped  time.Time
	DateIngested time.Time
	ContentHash  string
}

// PainPoint is a structured pain point extracted from a Description.
type PainPoint struct {
	ID            int64
	DescriptionID int64
	ChallengeText string
	Domain        string
	OutcomeText   string
	Confidence    float64
	QdrantPointID string
	DateExtracted time.Time
}

// Technology is a normalised technology keyword.
type Technology struct {
	ID       int64
	Name     string
	Category string // language | framework | platform | tool | database | methodology
}

// Cluster is a group of semantically similar PainPoints.
type Cluster struct {
	ID           int64
	Summary      string
	Frequency    int
	AvgSalary    *float64
	RecencyScore *float64
	GapType      string // skill_extension | skill_acquisition | domain_expansion | mixed
	GapScore     *float64
	DateClustered time.Time
}

// Brief is a generated project brief derived from a Cluster.
type Brief struct {
	ID                int64      `json:"id"`
	ClusterID         int64      `json:"cluster_id"`
	SourceCompany     string     `json:"source_company"` // Company from the originating Huntr job
	SourceRole        string     `json:"source_role"`    // Job title from the originating Huntr job
	Title             string     `json:"title"`
	ProblemStatement  string     `json:"problem_statement"`
	SuggestedApproach string     `json:"suggested_approach"`
	TechnologyStack   string     `json:"technology_stack"` // JSON array
	ProjectLayout     string     `json:"project_layout"`   // Recommended directory structure (markdown)
	Complexity        string     `json:"complexity"`       // small | medium | large
	ImpactScore       *float64   `json:"impact_score"`
	LinkedInAngle     string     `json:"linkedin_angle"`
	IsEdited          bool       `json:"is_edited"`
	DateGenerated     time.Time  `json:"date_generated"`
	DateModified      *time.Time `json:"date_modified"`
}

// Project tracks a Brief through the portfolio pipeline.
type Project struct {
	ID            int64      `json:"id"`
	BriefID       int64      `json:"brief_id"`
	Stage         string     `json:"stage"` // candidate | in_progress | parked | published | archived
	Title         string     `json:"title"`
	Complexity    string     `json:"complexity"`
	RepositoryURL string     `json:"repository_url"`
	LinkedInURL   string     `json:"linkedin_url"`
	GiteaURL      string     `json:"gitea_url"`
	LiveURL       string     `json:"live_url"`
	LinkedInDraft string     `json:"linkedin_draft"`
	Notes         string     `json:"notes"`
	DateCreated   time.Time  `json:"date_created"`
	DateSelected  *time.Time `json:"date_selected,omitempty"`
	DateStarted   *time.Time `json:"date_started,omitempty"`
	DateCompleted *time.Time `json:"date_completed,omitempty"`
	DatePublished *time.Time `json:"date_published,omitempty"`
	DateParked    *time.Time `json:"date_parked,omitempty"`
}

// ProjectWithBrief is a Project joined with brief title and complexity for list views.
type ProjectWithBrief struct {
	Project
	BriefTitle      string `json:"brief_title"`
	BriefComplexity string `json:"brief_complexity"`
}

// DisplayTitle returns BriefTitle if non-empty, else the project's own Title.
func (pw *ProjectWithBrief) DisplayTitle() string {
	if pw.BriefTitle != "" {
		return pw.BriefTitle
	}
	return pw.Title
}

// DisplayComplexity returns BriefComplexity if non-empty, else the project's own Complexity.
func (pw *ProjectWithBrief) DisplayComplexity() string {
	if pw.BriefComplexity != "" {
		return pw.BriefComplexity
	}
	return pw.Complexity
}
