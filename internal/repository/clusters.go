package repository

import (
	"database/sql"

	"github.com/yourname/projctr/internal/models"
)

// ClusterStore persists and queries clusters.
type ClusterStore struct {
	db *sql.DB
}

// NewClusterStore creates a store for the clusters table.
func NewClusterStore(db *sql.DB) *ClusterStore {
	return &ClusterStore{db: db}
}

// GetByID returns a cluster by ID.
func (s *ClusterStore) GetByID(id int64) (*models.Cluster, error) {
	var c models.Cluster
	var avgSal, recScore, gapScore sql.NullFloat64
	err := s.db.QueryRow(`
		SELECT id, summary, frequency, avg_salary, recency_score,
			gap_type, gap_score, date_clustered
		FROM clusters WHERE id = ?`, id,
	).Scan(
		&c.ID, &c.Summary, &c.Frequency, &avgSal, &recScore,
		&c.GapType, &gapScore, &c.DateClustered,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if avgSal.Valid {
		c.AvgSalary = &avgSal.Float64
	}
	if recScore.Valid {
		c.RecencyScore = &recScore.Float64
	}
	if gapScore.Valid {
		c.GapScore = &gapScore.Float64
	}
	return &c, nil
}

// List returns all clusters ordered by date_clustered desc.
func (s *ClusterStore) List() ([]*models.Cluster, error) {
	rows, err := s.db.Query(`
		SELECT id, summary, frequency, avg_salary, recency_score,
			gap_type, gap_score, date_clustered
		FROM clusters ORDER BY date_clustered DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Cluster
	for rows.Next() {
		var c models.Cluster
		var avgSal, recScore, gapScore sql.NullFloat64
		if err := rows.Scan(
			&c.ID, &c.Summary, &c.Frequency, &avgSal, &recScore,
			&c.GapType, &gapScore, &c.DateClustered,
		); err != nil {
			return nil, err
		}
		if avgSal.Valid {
			c.AvgSalary = &avgSal.Float64
		}
		if recScore.Valid {
			c.RecencyScore = &recScore.Float64
		}
		if gapScore.Valid {
			c.GapScore = &gapScore.Float64
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// Insert inserts a cluster and returns the new ID.
func (s *ClusterStore) Insert(c *models.Cluster) (int64, error) {
	res, err := s.db.Exec(`
		INSERT INTO clusters (summary, frequency, avg_salary, recency_score, gap_type, gap_score, date_clustered)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Summary, c.Frequency, float64ToNull(c.AvgSalary), float64ToNull(c.RecencyScore),
		c.GapType, float64ToNull(c.GapScore), c.DateClustered,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// InsertMember links a pain point to a cluster.
func (s *ClusterStore) InsertMember(clusterID, painPointID int64) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO cluster_members (cluster_id, pain_point_id) VALUES (?, ?)`,
		clusterID, painPointID,
	)
	return err
}

// Count returns the total number of clusters.
func (s *ClusterStore) Count() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM clusters`).Scan(&n)
	return n, err
}

// ListWithoutBriefs returns clusters that have no brief generated yet.
func (s *ClusterStore) ListWithoutBriefs() ([]*models.Cluster, error) {
	rows, err := s.db.Query(`
		SELECT c.id, c.summary, c.frequency, c.avg_salary, c.recency_score,
			c.gap_type, c.gap_score, c.date_clustered
		FROM clusters c
		LEFT JOIN briefs b ON b.cluster_id = c.id
		WHERE b.id IS NULL
		ORDER BY c.date_clustered DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Cluster
	for rows.Next() {
		var c models.Cluster
		var avgSal, recScore, gapScore sql.NullFloat64
		if err := rows.Scan(
			&c.ID, &c.Summary, &c.Frequency, &avgSal, &recScore,
			&c.GapType, &gapScore, &c.DateClustered,
		); err != nil {
			return nil, err
		}
		if avgSal.Valid {
			c.AvgSalary = &avgSal.Float64
		}
		if recScore.Valid {
			c.RecencyScore = &recScore.Float64
		}
		if gapScore.Valid {
			c.GapScore = &gapScore.Float64
		}
		out = append(out, &c)
	}
	return out, rows.Err()
}

// Clear deletes all cluster members and clusters.
func (s *ClusterStore) Clear() error {
	if _, err := s.db.Exec(`DELETE FROM cluster_members`); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM clusters`)
	return err
}

func float64ToNull(f *float64) interface{} {
	if f == nil {
		return nil
	}
	return *f
}
