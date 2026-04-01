package repository

import (
	"database/sql"

	"github.com/campbell/projctr/internal/models"
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

// TechnologiesForCluster returns distinct technologies linked to a cluster's pain points.
func (s *ClusterStore) TechnologiesForCluster(clusterID int64) ([]models.Technology, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT t.id, t.name, t.category
		FROM cluster_members cm
		JOIN pain_point_technologies ppt ON ppt.pain_point_id = cm.pain_point_id
		JOIN technologies t ON t.id = ppt.technology_id
		WHERE cm.cluster_id = ?
		ORDER BY t.category, t.name`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Technology
	for rows.Next() {
		var t models.Technology
		if err := rows.Scan(&t.ID, &t.Name, &t.Category); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// PainPointsForCluster returns pain points in a cluster ordered by confidence desc.
func (s *ClusterStore) PainPointsForCluster(clusterID int64) ([]models.PainPoint, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.description_id, p.challenge_text,
			COALESCE(p.domain,''), COALESCE(p.outcome_text,''),
			p.confidence, COALESCE(p.qdrant_point_id,''), p.date_extracted
		FROM cluster_members cm
		JOIN pain_points p ON p.id = cm.pain_point_id
		WHERE cm.cluster_id = ?
		ORDER BY p.confidence DESC`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.PainPoint
	for rows.Next() {
		var pp models.PainPoint
		if err := rows.Scan(
			&pp.ID, &pp.DescriptionID, &pp.ChallengeText, &pp.Domain,
			&pp.OutcomeText, &pp.Confidence, &pp.QdrantPointID, &pp.DateExtracted,
		); err != nil {
			return nil, err
		}
		out = append(out, pp)
	}
	return out, rows.Err()
}

// DescriptionsForCluster returns distinct job descriptions linked to a cluster's pain points.
func (s *ClusterStore) DescriptionsForCluster(clusterID int64) ([]models.Description, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT d.id, COALESCE(d.huntr_id,''), COALESCE(d.role_title,''), COALESCE(d.sector,''),
			d.salary_min, d.salary_max, COALESCE(d.location,''), COALESCE(d.source_board,''),
			COALESCE(d.huntr_score,0), COALESCE(d.raw_text,''),
			COALESCE(d.date_scraped, d.date_ingested), d.date_ingested, COALESCE(d.content_hash,'')
		FROM cluster_members cm
		JOIN pain_points pp ON pp.id = cm.pain_point_id
		JOIN descriptions d ON d.id = pp.description_id
		WHERE cm.cluster_id = ?`, clusterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Description
	for rows.Next() {
		var d models.Description
		var salMin, salMax sql.NullInt64
		var dateScraped sql.NullString
		if err := rows.Scan(
			&d.ID, &d.HuntrID, &d.RoleTitle, &d.Sector,
			&salMin, &salMax, &d.Location, &d.SourceBoard,
			&d.HuntrScore, &d.RawText, &dateScraped, &d.DateIngested, &d.ContentHash,
		); err != nil {
			return nil, err
		}
		if salMin.Valid {
			v := int(salMin.Int64)
			d.SalaryMin = &v
		}
		if salMax.Valid {
			v := int(salMax.Int64)
			d.SalaryMax = &v
		}
		if dateScraped.Valid {
			d.DateScraped = d.DateIngested // best effort
		}
		out = append(out, d)
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
