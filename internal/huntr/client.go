package huntr

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yourname/projctr/internal/models"
)

// JobReader reads job descriptions from Huntr's JSON files in jobs/scored/.
// It never writes to or modifies Huntr's data.
type JobReader struct {
	jobsPath string
	mu       sync.RWMutex
	scoreMin float64 // inclusive lower bound
	scoreMax float64 // exclusive upper bound
}

// NewJobReader creates a reader for Huntr's scored jobs JSON files.
// jobsPath is the path to jobs/scored/ (e.g. /mnt/nas/huntr-data/jobs/scored).
// scoreMin is inclusive; scoreMax is exclusive.
func NewJobReader(jobsPath string, scoreMin, scoreMax float64) *JobReader {
	return &JobReader{
		jobsPath: strings.TrimSuffix(jobsPath, "/"),
		scoreMin: scoreMin,
		scoreMax: scoreMax,
	}
}

// SetRange updates the score band at runtime (thread-safe).
func (r *JobReader) SetRange(min, max float64) {
	r.mu.Lock()
	r.scoreMin = min
	r.scoreMax = max
	r.mu.Unlock()
}

// Range returns the current score band.
func (r *JobReader) Range() (min, max float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.scoreMin, r.scoreMax
}

// RawDescription holds the fields Projctr reads from Huntr's JSON.
// Maps to models.Description for ingestion.
type RawDescription struct {
	HuntrID     string
	RoleTitle   string
	Sector      string
	SalaryMin   *int
	SalaryMax   *int
	Location    string
	SourceBoard string
	HuntrScore  float64
	RawText     string
	DateScraped string // From filename or empty
}

// FetchSubThreshold returns job descriptions whose Huntr score falls within the
// configured band [scoreMin, scoreMax). Reads from the most recent JSON files first.
func (r *JobReader) FetchSubThreshold() ([]RawDescription, error) {
	r.mu.RLock()
	scoreMin, scoreMax := r.scoreMin, r.scoreMax
	r.mu.RUnlock()

	files, err := r.listScoredFiles()
	if err != nil {
		return nil, err
	}

	var jobs []models.ScoredJob
	seen := make(map[string]bool) // Dedupe by link

	for _, f := range files {
		batch, err := r.readFile(f)
		if err != nil {
			continue // Skip malformed files
		}
		for _, j := range batch {
			if j.Score < scoreMin || j.Score >= scoreMax {
				continue
			}
			key := j.Link
			if key == "" {
				key = j.Title + "|" + j.Company
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			jobs = append(jobs, j)
		}
	}

	return r.toRawDescriptions(jobs), nil
}

func (r *JobReader) listScoredFiles() ([]string, error) {
	entries, err := os.ReadDir(r.jobsPath)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), "jobs_scored_") && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(r.jobsPath, e.Name()))
		}
	}

	// sort newest first
	sort.Slice(files, func(i, j int) bool {
		return files[i] > files[j]
	})
	return files, nil
}

func (r *JobReader) readFile(path string) ([]models.ScoredJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Huntr writes NaN for missing numeric fields (e.g. salary_num).
	// Go's JSON parser rejects NaN — replace with null before decoding.
	data = []byte(strings.ReplaceAll(string(data), ": NaN", ": null"))
	data = []byte(strings.ReplaceAll(string(data), ":NaN", ":null"))

	var jobs []models.ScoredJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *JobReader) toRawDescriptions(jobs []models.ScoredJob) []RawDescription {
	out := make([]RawDescription, len(jobs))
	for i, j := range jobs {
		var salaryMin, salaryMax *int
		if j.SalaryNum > 0 {
			s := int(j.SalaryNum)
			salaryMin = &s
			salaryMax = &s
		}
		out[i] = RawDescription{
			HuntrID:     j.Link,
			RoleTitle:   j.Title,
			Sector:      j.Company,
			SalaryMin:   salaryMin,
			SalaryMax:   salaryMax,
			Location:    j.Location,
			SourceBoard: j.Source,
			HuntrScore:  j.Score,
			RawText:     j.Description,
		}
	}
	return out
}
