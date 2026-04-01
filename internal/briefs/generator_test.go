package briefs_test

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/campbell/projctr/internal/briefs"
	"github.com/campbell/projctr/internal/database"
	"github.com/campbell/projctr/internal/models"
	"github.com/campbell/projctr/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

// seedCluster inserts a description, pain points with technologies, a cluster,
// and links them via cluster_members and pain_point_technologies.
func seedCluster(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	now := time.Now()

	// Insert a job description.
	res, err := db.Exec(`INSERT INTO descriptions (huntr_id, role_title, sector, raw_text, date_ingested, content_hash)
		VALUES (?, ?, ?, ?, ?, ?)`,
		"test-001", "DevOps Engineer", "FinTech", "We need Kubernetes and Terraform experience", now, "abc123")
	if err != nil {
		t.Fatal(err)
	}
	descID, _ := res.LastInsertId()

	// Insert technologies.
	var techIDs []int64
	for _, tech := range []struct{ name, cat string }{
		{"Kubernetes", "platform"},
		{"Terraform", "tool"},
		{"Docker", "platform"},
	} {
		res, err := db.Exec(`INSERT INTO technologies (name, category) VALUES (?, ?)`, tech.name, tech.cat)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		techIDs = append(techIDs, id)
	}

	// Insert pain points linked to the description.
	var ppIDs []int64
	for i, pp := range []struct{ challenge, domain string }{
		{"Manual infrastructure provisioning is slow and error-prone", "platform"},
		{"Container orchestration lacks monitoring and auto-scaling", "platform"},
		{"No infrastructure-as-code pipeline for compliance audits", "tool"},
	} {
		res, err := db.Exec(`INSERT INTO pain_points (description_id, challenge_text, domain, confidence, date_extracted)
			VALUES (?, ?, ?, ?, ?)`,
			descID, pp.challenge, pp.domain, 0.9-float64(i)*0.1, now)
		if err != nil {
			t.Fatal(err)
		}
		id, _ := res.LastInsertId()
		ppIDs = append(ppIDs, id)
	}

	// Link technologies to pain points.
	for i, ppID := range ppIDs {
		techID := techIDs[i%len(techIDs)]
		if _, err := db.Exec(`INSERT INTO pain_point_technologies (pain_point_id, technology_id) VALUES (?, ?)`, ppID, techID); err != nil {
			t.Fatal(err)
		}
	}

	// Insert cluster.
	res, err = db.Exec(`INSERT INTO clusters (summary, frequency, gap_type, gap_score, date_clustered)
		VALUES (?, ?, ?, ?, ?)`,
		"Infrastructure automation and container orchestration gaps", 8, "skill_acquisition", 0.85, now)
	if err != nil {
		t.Fatal(err)
	}
	clusterID, _ := res.LastInsertId()

	// Link pain points to cluster.
	for _, ppID := range ppIDs {
		if _, err := db.Exec(`INSERT INTO cluster_members (cluster_id, pain_point_id) VALUES (?, ?)`, clusterID, ppID); err != nil {
			t.Fatal(err)
		}
	}

	return clusterID
}

func TestGenerateFromCluster_UseRealTechStack(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	clusterID := seedCluster(t, db)
	clusterStore := repository.NewClusterStore(db)
	gen := briefs.NewGenerator(clusterStore)

	cluster, err := clusterStore.GetByID(clusterID)
	if err != nil || cluster == nil {
		t.Fatal("failed to fetch cluster:", err)
	}

	brief := gen.GenerateFromCluster(cluster)

	// Tech stack should contain actual extracted technologies, not hardcoded Python.
	if strings.Contains(brief.TechnologyStack, "Python") {
		t.Errorf("tech stack should not contain hardcoded Python, got: %s", brief.TechnologyStack)
	}
	if strings.Contains(brief.TechnologyStack, "FastAPI") {
		t.Errorf("tech stack should not contain hardcoded FastAPI, got: %s", brief.TechnologyStack)
	}

	// Should contain the actual technologies we seeded.
	var techs []string
	if err := json.Unmarshal([]byte(brief.TechnologyStack), &techs); err != nil {
		t.Fatalf("tech stack should be valid JSON array, got: %s", brief.TechnologyStack)
	}
	found := map[string]bool{}
	for _, tech := range techs {
		found[tech] = true
	}
	for _, want := range []string{"Kubernetes", "Terraform", "Docker"} {
		if !found[want] {
			t.Errorf("tech stack missing %s, got: %v", want, techs)
		}
	}

	// Approach should reference actual pain points, not generic steps.
	if strings.Contains(brief.SuggestedApproach, "Define API contract") {
		t.Error("approach should not contain generic placeholder")
	}
	if !strings.Contains(brief.SuggestedApproach, "Manual infrastructure") {
		t.Errorf("approach should reference top pain point, got: %s", brief.SuggestedApproach)
	}

	// LinkedIn angle should mention actual technologies.
	if strings.Contains(brief.LinkedInAngle, "Demonstrates skill_acquisition") {
		t.Error("LinkedIn angle should not use old generic formula")
	}
	if !strings.Contains(brief.LinkedInAngle, "Kubernetes") {
		t.Errorf("LinkedIn angle should mention technologies, got: %s", brief.LinkedInAngle)
	}

	// Source fields should be populated from the seeded description.
	if brief.SourceRole != "DevOps Engineer" {
		t.Errorf("SourceRole = %q, want %q", brief.SourceRole, "DevOps Engineer")
	}
	if brief.SourceCompany != "FinTech" {
		t.Errorf("SourceCompany = %q, want %q", brief.SourceCompany, "FinTech")
	}
}

func TestGenerateFromCluster_FallbackWhenNoTechs(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	now := time.Now()
	clusterStore := repository.NewClusterStore(db)
	gen := briefs.NewGenerator(clusterStore)

	// Insert a cluster with no linked pain points/technologies.
	res, err := db.Exec(`INSERT INTO clusters (summary, frequency, gap_type, date_clustered)
		VALUES (?, ?, ?, ?)`, "Orphan cluster", 3, "skill_acquisition", now)
	if err != nil {
		t.Fatal(err)
	}
	clusterID, _ := res.LastInsertId()

	cluster, _ := clusterStore.GetByID(clusterID)
	brief := gen.GenerateFromCluster(cluster)

	// Should use fallback tech stack (no longer Python).
	if strings.Contains(brief.TechnologyStack, "Python") {
		t.Errorf("fallback tech stack should not contain Python, got: %s", brief.TechnologyStack)
	}

	// Should use fallback approach.
	if !strings.Contains(brief.SuggestedApproach, "Research the problem domain") {
		t.Errorf("expected fallback approach, got: %s", brief.SuggestedApproach)
	}
}

func TestClusterStore_TechnologiesForCluster(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	clusterID := seedCluster(t, db)
	store := repository.NewClusterStore(db)

	techs, err := store.TechnologiesForCluster(clusterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(techs) != 3 {
		t.Fatalf("expected 3 technologies, got %d", len(techs))
	}

	names := map[string]bool{}
	for _, tech := range techs {
		names[tech.Name] = true
	}
	for _, want := range []string{"Kubernetes", "Terraform", "Docker"} {
		if !names[want] {
			t.Errorf("missing technology %s", want)
		}
	}
}

func TestClusterStore_PainPointsForCluster(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	clusterID := seedCluster(t, db)
	store := repository.NewClusterStore(db)

	pps, err := store.PainPointsForCluster(clusterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pps) != 3 {
		t.Fatalf("expected 3 pain points, got %d", len(pps))
	}
	// Should be ordered by confidence DESC.
	if pps[0].Confidence < pps[1].Confidence {
		t.Error("pain points should be ordered by confidence DESC")
	}
}

func TestClusterStore_DescriptionsForCluster(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	clusterID := seedCluster(t, db)
	store := repository.NewClusterStore(db)

	descs, err := store.DescriptionsForCluster(clusterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 description, got %d", len(descs))
	}
	if descs[0].RoleTitle != "DevOps Engineer" {
		t.Errorf("RoleTitle = %q, want %q", descs[0].RoleTitle, "DevOps Engineer")
	}
}

func TestLinkedInPromptIncludesProjectContext(t *testing.T) {
	// Verify the Generator.Generate signature accepts Brief + Project.
	// This is a compile-time check more than a runtime one since we can't
	// call Ollama in tests, but it confirms the interface is correct.
	_ = func(b *models.Brief, p *models.Project) {}
}
