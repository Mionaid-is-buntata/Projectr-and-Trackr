package config

import (
	"os"
	"path/filepath"
	"testing"
)

const testTOML = `
[server]
port = "9090"
host = "127.0.0.1"

[database]
path = "./test.db"

[huntr]
jobs_path = "./testdata"
score_min = 50
score_max = 250

[qdrant]
host = "localhost"
port = 6334
collection = "test"
description_collection = "test_desc"
vector_dimensions = 384

[embedding]
model = "test-model"
endpoint = "http://localhost:11434/api/embeddings"

[extraction]
mode = "rules"
tech_dictionary = "./config/tech-dict.toml"

[extraction.llm]
enabled = false
endpoint = "http://localhost:11434"
model = "gemma2:9b"

[clustering]
min_cluster_size = 3
similarity_threshold = 0.65

[trackr]
enabled = true

[trackr.llm]
endpoint = ""
model = ""
`

func writeTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(testTOML), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_BasicFields(t *testing.T) {
	cfg, err := Load(writeTempConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != "9090" {
		t.Errorf("port = %q, want 9090", cfg.Server.Port)
	}
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("host = %q", cfg.Server.Host)
	}
	if cfg.Database.Path != "./test.db" {
		t.Errorf("db path = %q", cfg.Database.Path)
	}
	if cfg.Huntr.ScoreMin != 50 {
		t.Errorf("score_min = %f", cfg.Huntr.ScoreMin)
	}
	if cfg.Huntr.ScoreMax != 250 {
		t.Errorf("score_max = %f", cfg.Huntr.ScoreMax)
	}
	if cfg.Clustering.MinClusterSize != 3 {
		t.Errorf("min_cluster_size = %d", cfg.Clustering.MinClusterSize)
	}
	if cfg.Trackr.Enabled != true {
		t.Error("trackr should be enabled")
	}
}

func TestLoad_EnvOverride_DatabasePath(t *testing.T) {
	t.Setenv("DATABASE_PATH", "/override/db.sqlite")
	cfg, err := Load(writeTempConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Path != "/override/db.sqlite" {
		t.Errorf("expected override, got %q", cfg.Database.Path)
	}
}

func TestLoad_EnvOverride_HuntrJobsPath(t *testing.T) {
	t.Setenv("HUNTR_JOBS_PATH", "/override/jobs")
	cfg, err := Load(writeTempConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Huntr.JobsPath != "/override/jobs" {
		t.Errorf("expected override, got %q", cfg.Huntr.JobsPath)
	}
}

func TestLoad_EnvOverride_TrackrLLM(t *testing.T) {
	t.Setenv("TRACKR_LLM_ENDPOINT", "http://francis.local:11434")
	t.Setenv("TRACKR_LLM_MODEL", "qwen2.5:15b")
	cfg, err := Load(writeTempConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Trackr.LLM.Endpoint != "http://francis.local:11434" {
		t.Errorf("trackr llm endpoint = %q", cfg.Trackr.LLM.Endpoint)
	}
	if cfg.Trackr.LLM.Model != "qwen2.5:15b" {
		t.Errorf("trackr llm model = %q", cfg.Trackr.LLM.Model)
	}
}

func TestLoad_EnvOverride_QdrantPort(t *testing.T) {
	t.Setenv("QDRANT_PORT", "6335")
	cfg, err := Load(writeTempConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Qdrant.Port != 6335 {
		t.Errorf("qdrant port = %d, want 6335", cfg.Qdrant.Port)
	}
}

func TestLoad_EnvOverride_LLMEndpoint(t *testing.T) {
	t.Setenv("LLM_ENDPOINT", "http://custom:11434")
	cfg, err := Load(writeTempConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Extraction.LLM.Endpoint != "http://custom:11434" {
		t.Errorf("llm endpoint = %q", cfg.Extraction.LLM.Endpoint)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	os.WriteFile(path, []byte("this is not valid toml [[["), 0644)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}
