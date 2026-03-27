package huntr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func setupTestJobsDir(t *testing.T, jobs interface{}) string {
	t.Helper()
	dir := t.TempDir()
	data, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "jobs_scored_20260327_120000.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFetchSubThreshold_ScoreFiltering(t *testing.T) {
	jobs := []map[string]interface{}{
		{"title": "Low Score", "company": "A", "score": 50.0, "link": "http://a.com/1", "description": "desc1"},
		{"title": "In Range", "company": "B", "score": 150.0, "link": "http://b.com/1", "description": "desc2"},
		{"title": "Too High", "company": "C", "score": 350.0, "link": "http://c.com/1", "description": "desc3"},
	}
	dir := setupTestJobsDir(t, jobs)
	reader := NewJobReader(dir, 100, 300)

	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 job in range [100,300), got %d", len(descs))
	}
	if descs[0].RoleTitle != "In Range" {
		t.Errorf("expected 'In Range', got %q", descs[0].RoleTitle)
	}
}

func TestFetchSubThreshold_DedupeByLink(t *testing.T) {
	jobs := []map[string]interface{}{
		{"title": "Job A", "company": "A", "score": 150.0, "link": "http://same.com/job", "description": "desc1"},
		{"title": "Job B", "company": "B", "score": 200.0, "link": "http://same.com/job", "description": "desc2"},
	}
	dir := setupTestJobsDir(t, jobs)
	reader := NewJobReader(dir, 0, 300)

	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 (deduped by link), got %d", len(descs))
	}
}

func TestFetchSubThreshold_DedupeByTitleCompany(t *testing.T) {
	jobs := []map[string]interface{}{
		{"title": "Same Job", "company": "Same Co", "score": 150.0, "link": "", "description": "desc1"},
		{"title": "Same Job", "company": "Same Co", "score": 200.0, "link": "", "description": "desc2"},
	}
	dir := setupTestJobsDir(t, jobs)
	reader := NewJobReader(dir, 0, 300)

	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 (deduped by title|company), got %d", len(descs))
	}
}

func TestFetchSubThreshold_NaNHandling(t *testing.T) {
	dir := t.TempDir()
	data := `[{"title":"NaN Job","company":"X","score":150.0,"salary_num": NaN,"link":"http://x.com/1","description":"desc"}]`
	if err := os.WriteFile(filepath.Join(dir, "jobs_scored_20260327_120000.json"), []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	reader := NewJobReader(dir, 0, 300)

	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatalf("expected 1 job (NaN handled), got %d", len(descs))
	}
	if descs[0].SalaryMin != nil {
		t.Error("salary should be nil for NaN")
	}
}

func TestFetchSubThreshold_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	reader := NewJobReader(dir, 0, 300)

	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 0 {
		t.Fatalf("expected 0, got %d", len(descs))
	}
}

func TestFetchSubThreshold_MissingDir(t *testing.T) {
	reader := NewJobReader("/nonexistent/path", 0, 300)

	_, err := reader.FetchSubThreshold()
	if err == nil {
		t.Fatal("expected error for missing directory")
	}
}

func TestSetRange_And_Range(t *testing.T) {
	reader := NewJobReader(t.TempDir(), 0, 300)
	reader.SetRange(100, 500)

	min, max := reader.Range()
	if min != 100 || max != 500 {
		t.Errorf("Range() = (%f, %f), want (100, 500)", min, max)
	}
}

func TestSetRange_ThreadSafety(t *testing.T) {
	reader := NewJobReader(t.TempDir(), 0, 300)
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(v float64) {
			defer wg.Done()
			reader.SetRange(v, v+100)
		}(float64(i))
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reader.Range()
		}()
	}

	wg.Wait()
	// No race condition — test passes if it doesn't panic
}

func TestFetchSubThreshold_SalaryConversion(t *testing.T) {
	jobs := []map[string]interface{}{
		{"title": "With Salary", "company": "A", "score": 150.0, "salary_num": 75000.0, "link": "http://a.com/1", "description": "desc"},
	}
	dir := setupTestJobsDir(t, jobs)
	reader := NewJobReader(dir, 0, 300)

	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 1 {
		t.Fatal("expected 1")
	}
	if descs[0].SalaryMin == nil || *descs[0].SalaryMin != 75000 {
		t.Errorf("salary_min = %v, want 75000", descs[0].SalaryMin)
	}
}

func TestFetchSubThreshold_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	older := []map[string]interface{}{
		{"title": "Old Job", "company": "A", "score": 150.0, "link": "http://old.com/1", "description": "old desc"},
	}
	newer := []map[string]interface{}{
		{"title": "New Job", "company": "B", "score": 200.0, "link": "http://new.com/1", "description": "new desc"},
	}

	writeJSON := func(name string, data interface{}) {
		b, _ := json.Marshal(data)
		os.WriteFile(filepath.Join(dir, name), b, 0644)
	}
	writeJSON("jobs_scored_20260326_120000.json", older)
	writeJSON("jobs_scored_20260327_120000.json", newer)

	reader := NewJobReader(dir, 0, 300)
	descs, err := reader.FetchSubThreshold()
	if err != nil {
		t.Fatal(err)
	}
	if len(descs) != 2 {
		t.Fatalf("expected 2 from 2 files, got %d", len(descs))
	}
}
