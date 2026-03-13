package repository

import (
	"database/sql"
	"fmt"
	"strconv"
)

// SettingsStore reads and writes key/value settings from the settings table.
type SettingsStore struct {
	db *sql.DB
}

func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{db: db}
}

func (s *SettingsStore) GetFloat(key string, defaultVal float64) float64 {
	var raw string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&raw)
	if err != nil {
		return defaultVal
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultVal
	}
	return v
}

func (s *SettingsStore) SetFloat(key string, val float64) error {
	_, err := s.db.Exec(
		`INSERT INTO settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, fmt.Sprintf("%g", val),
	)
	return err
}
