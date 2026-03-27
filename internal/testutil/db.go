package testutil

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yourname/projctr/internal/database"
	"github.com/yourname/projctr/internal/models"
)

// NewTestDB creates an in-memory SQLite database with all migrations applied.
func NewTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// SeedDescription inserts a test job description and returns its ID.
func SeedDescription(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	return SeedDescriptionWith(t, db, "test-001", "DevOps Engineer", "FinTech",
		"We need Kubernetes and Terraform experience for infrastructure automation")
}

// SeedDescriptionWith inserts a description with custom fields and returns its ID.
func SeedDescriptionWith(t *testing.T, db *sql.DB, huntrID, roleTitle, sector, rawText string) int64 {
	t.Helper()
	now := time.Now()
	res, err := db.Exec(`INSERT INTO descriptions (huntr_id, role_title, sector, location, source_board, huntr_score, raw_text, date_scraped, date_ingested, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		huntrID, roleTitle, sector, "London", "LinkedIn", 150.0, rawText, now, now, "hash-"+huntrID)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// SeedTechnology inserts a technology and returns its ID.
func SeedTechnology(t *testing.T, db *sql.DB, name, category string) int64 {
	t.Helper()
	res, err := db.Exec(`INSERT INTO technologies (name, category) VALUES (?, ?)`, name, category)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// SeedPainPoint inserts a pain point linked to a description and returns its ID.
func SeedPainPoint(t *testing.T, db *sql.DB, descID int64, challenge, domain string, confidence float64) int64 {
	t.Helper()
	now := time.Now()
	res, err := db.Exec(`INSERT INTO pain_points (description_id, challenge_text, domain, confidence, date_extracted)
		VALUES (?, ?, ?, ?, ?)`, descID, challenge, domain, confidence, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// LinkTech links a pain point to a technology.
func LinkTech(t *testing.T, db *sql.DB, ppID, techID int64) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO pain_point_technologies (pain_point_id, technology_id) VALUES (?, ?)`, ppID, techID); err != nil {
		t.Fatal(err)
	}
}

// SeedCluster inserts a cluster with linked pain points and technologies.
// Returns the cluster ID.
func SeedCluster(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	descID := SeedDescription(t, db)

	techK8s := SeedTechnology(t, db, "Kubernetes", "platform")
	techTF := SeedTechnology(t, db, "Terraform", "tool")
	techDocker := SeedTechnology(t, db, "Docker", "platform")

	pp1 := SeedPainPoint(t, db, descID, "Manual infrastructure provisioning is slow", "platform", 0.9)
	pp2 := SeedPainPoint(t, db, descID, "Container orchestration lacks monitoring", "platform", 0.8)
	pp3 := SeedPainPoint(t, db, descID, "No infrastructure-as-code pipeline", "tool", 0.7)

	LinkTech(t, db, pp1, techK8s)
	LinkTech(t, db, pp2, techDocker)
	LinkTech(t, db, pp3, techTF)

	now := time.Now()
	res, err := db.Exec(`INSERT INTO clusters (summary, frequency, gap_type, gap_score, date_clustered)
		VALUES (?, ?, ?, ?, ?)`,
		"Infrastructure automation gaps", 8, "skill_acquisition", 0.85, now)
	if err != nil {
		t.Fatal(err)
	}
	clusterID, _ := res.LastInsertId()

	for _, ppID := range []int64{pp1, pp2, pp3} {
		if _, err := db.Exec(`INSERT INTO cluster_members (cluster_id, pain_point_id) VALUES (?, ?)`, clusterID, ppID); err != nil {
			t.Fatal(err)
		}
	}

	return clusterID
}

// SeedBrief inserts a brief for a cluster and returns its ID.
func SeedBrief(t *testing.T, db *sql.DB, clusterID int64) int64 {
	t.Helper()
	now := time.Now()
	res, err := db.Exec(`INSERT INTO briefs (cluster_id, title, problem_statement, suggested_approach,
		technology_stack, complexity, impact_score, linkedin_angle, is_edited, date_generated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		clusterID, "Brief 1: Test brief", "Test problem", "Test approach",
		`["Go","Docker"]`, "medium", 0.85, "Test angle", 0, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// SeedProject inserts a candidate project for a brief and returns its ID.
func SeedProject(t *testing.T, db *sql.DB, briefID int64) int64 {
	t.Helper()
	now := time.Now()
	res, err := db.Exec(`INSERT INTO projects (brief_id, stage, title, complexity, date_created)
		VALUES (?, ?, ?, ?, ?)`, briefID, "candidate", "", "", now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	return id
}

// SeedFullChain creates description -> pain points -> cluster -> brief -> project chain.
// Returns all IDs.
func SeedFullChain(t *testing.T, db *sql.DB) (descID, clusterID, briefID, projectID int64) {
	t.Helper()
	clusterID = SeedCluster(t, db)
	briefID = SeedBrief(t, db, clusterID)
	projectID = SeedProject(t, db, briefID)

	// Get the description ID that SeedCluster created
	var dID int64
	err := db.QueryRow(`SELECT id FROM descriptions LIMIT 1`).Scan(&dID)
	if err != nil {
		t.Fatal(err)
	}
	return dID, clusterID, briefID, projectID
}

// MustSeedBrief creates a minimal brief (without cluster chain) for testing handlers.
func MustSeedBrief(t *testing.T, db *sql.DB) *models.Brief {
	t.Helper()
	now := time.Now()
	b := &models.Brief{
		ClusterID:        1,
		Title:            "Test Brief",
		ProblemStatement: "Test problem",
		TechnologyStack:  `["Go"]`,
		Complexity:       "small",
		DateGenerated:    now,
	}
	res, err := db.Exec(`INSERT INTO briefs (cluster_id, title, problem_statement, technology_stack, complexity, is_edited, date_generated)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, b.ClusterID, b.Title, b.ProblemStatement, b.TechnologyStack, b.Complexity, 0, now)
	if err != nil {
		t.Fatal(err)
	}
	b.ID, _ = res.LastInsertId()
	return b
}
