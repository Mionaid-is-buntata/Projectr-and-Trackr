package clustering

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yourname/projctr/internal/models"
	"github.com/yourname/projctr/internal/repository"
)

// Service orchestrates semantic clustering of pain point embeddings.
type Service struct {
	PainPoints *repository.PainPointStore
	Clusters   *repository.ClusterStore
	Embedder   *Embedder
	DBSCAN     *DBSCAN
}

// NewService creates a clustering service.
func NewService(
	pp *repository.PainPointStore,
	clusters *repository.ClusterStore,
	embedder *Embedder,
	dbscan *DBSCAN,
) *Service {
	return &Service{
		PainPoints: pp,
		Clusters:   clusters,
		Embedder:   embedder,
		DBSCAN:     dbscan,
	}
}

// Cluster groups all unassigned pain points into clusters.
// Pain points already in cluster_members are skipped (idempotent).
func (s *Service) Cluster() error {
	points, err := s.PainPoints.ListUnassigned()
	if err != nil {
		return fmt.Errorf("clustering: list unassigned pain points: %w", err)
	}
	if len(points) == 0 {
		log.Printf("clustering: no unassigned pain points to cluster")
		return nil
	}

	var labels []int

	if s.Embedder.Ready() {
		texts := make([]string, len(points))
		for i, p := range points {
			texts[i] = p.ChallengeText
		}
		vectors, err := s.Embedder.EmbedBatch(texts)
		if err != nil {
			log.Printf("clustering: embedding failed (%v), falling back to domain grouping", err)
			labels = domainGroupLabels(points)
		} else {
			labels, err = s.DBSCAN.Run(vectors)
			if err != nil || labels == nil {
				log.Printf("clustering: DBSCAN failed or dataset too large, falling back to domain grouping")
				labels = domainGroupLabels(points)
			}
		}
	} else {
		log.Printf("clustering: embedder not ready, using domain grouping")
		labels = domainGroupLabels(points)
	}

	return s.persistClusters(points, labels)
}

// persistClusters saves cluster rows and their members to the database.
func (s *Service) persistClusters(points []*models.PainPoint, labels []int) error {
	// Group point indices by label, skip noise (-1)
	groups := map[int][]int{}
	for i, label := range labels {
		if label >= 0 {
			groups[label] = append(groups[label], i)
		}
	}

	for _, indices := range groups {
		cluster := buildCluster(points, indices)
		clusterID, err := s.Clusters.Insert(cluster)
		if err != nil {
			log.Printf("clustering: insert cluster: %v", err)
			continue
		}
		for _, idx := range indices {
			if err := s.Clusters.InsertMember(clusterID, points[idx].ID); err != nil {
				log.Printf("clustering: insert member: %v", err)
			}
		}
	}
	return nil
}

// buildCluster derives a Cluster model from a group of pain points.
func buildCluster(points []*models.PainPoint, indices []int) *models.Cluster {
	domainCounts := map[string]int{}
	var recencySum float64

	for _, i := range indices {
		p := points[i]
		domainCounts[p.Domain]++
		// Recency: days since extraction, capped at 90
		age := time.Since(p.DateExtracted).Hours() / 24
		if age > 90 {
			age = 90
		}
		recencySum += (90 - age) / 90 // 1.0 = today, 0.0 = 90 days ago
	}

	topDomain := topKey(domainCounts)
	recency := recencySum / float64(len(indices))

	summary := buildSummary(points, indices, topDomain)
	gapType := domainToGapType(topDomain)
	freq := len(indices)

	return &models.Cluster{
		Summary:       summary,
		Frequency:     freq,
		RecencyScore:  &recency,
		GapType:       gapType,
		DateClustered: time.Now(),
	}
}

func buildSummary(points []*models.PainPoint, indices []int, topDomain string) string {
	// Collect the highest-confidence challenge text
	best := points[indices[0]]
	for _, i := range indices[1:] {
		if points[i].Confidence > best.Confidence {
			best = points[i]
		}
	}
	s := best.ChallengeText
	if len(s) > 200 {
		s = s[:197] + "..."
	}
	return fmt.Sprintf("[%s] %s", strings.Title(topDomain), s)
}

func domainToGapType(domain string) string {
	switch domain {
	case "language", "framework":
		return "skill_extension"
	case "platform", "tool":
		return "skill_acquisition"
	case "database", "methodology":
		return "domain_expansion"
	default:
		return "mixed"
	}
}

// domainGroupLabels assigns labels by domain field for fallback (no embeddings).
// Pain points in the same domain get the same label.
func domainGroupLabels(points []*models.PainPoint) []int {
	domains := map[string]int{}
	labels := make([]int, len(points))
	nextID := 0
	for i, p := range points {
		d := p.Domain
		if d == "" {
			d = "general"
		}
		if id, ok := domains[d]; ok {
			labels[i] = id
		} else {
			domains[d] = nextID
			labels[i] = nextID
			nextID++
		}
	}
	return labels
}

// topKey returns the key with the highest count in the map.
func topKey(counts map[string]int) string {
	type kv struct {
		k string
		v int
	}
	var pairs []kv
	for k, v := range counts {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	if len(pairs) == 0 {
		return "general"
	}
	return pairs[0].k
}

// ReclusterWithNewDescriptions clusters only newly added (unassigned) pain points.
// New pain points are grouped and added to existing clusters or form new ones.
func (s *Service) ReclusterWithNewDescriptions() error {
	return s.Cluster()
}
