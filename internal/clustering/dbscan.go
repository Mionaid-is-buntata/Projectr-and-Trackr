package clustering

import "math"

// DBSCAN implements density-based spatial clustering for pain point vectors.
// Points that do not belong to any cluster are classified as noise (label -1).
// Parameters minPts and epsilon come from ClusteringConfig.
type DBSCAN struct {
	minPts  int
	epsilon float64
}

// NewDBSCAN creates a DBSCAN instance with the given parameters.
func NewDBSCAN(minPts int, epsilon float64) *DBSCAN {
	return &DBSCAN{minPts: minPts, epsilon: epsilon}
}

// Run clusters the given vectors using cosine distance and returns cluster assignments.
// Index i in the returned slice is the cluster ID for point i.
// A cluster ID of -1 indicates noise.
func (d *DBSCAN) Run(vectors [][]float32) ([]int, error) {
	n := len(vectors)
	if n == 0 {
		return nil, nil
	}

	// For large datasets fall back to domain-grouping in the service layer;
	// here we just cap computation to keep Raspberry Pi memory sane.
	if n > 5000 {
		return nil, nil
	}

	// Precompute pairwise cosine distances.
	dist := make([][]float64, n)
	for i := range dist {
		dist[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			d := cosineDistance(vectors[i], vectors[j])
			dist[i][j] = d
			dist[j][i] = d
		}
	}

	labels := make([]int, n)
	for i := range labels {
		labels[i] = -1 // unvisited / noise
	}

	clusterID := 0
	visited := make([]bool, n)

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		visited[i] = true

		neighbours := d.regionQuery(dist, i, n)
		if len(neighbours) < d.minPts {
			// Noise point
			continue
		}

		labels[i] = clusterID
		seed := make([]int, len(neighbours))
		copy(seed, neighbours)

		for len(seed) > 0 {
			q := seed[0]
			seed = seed[1:]

			if !visited[q] {
				visited[q] = true
				qNeighbours := d.regionQuery(dist, q, n)
				if len(qNeighbours) >= d.minPts {
					seed = append(seed, qNeighbours...)
				}
			}
			if labels[q] == -1 {
				labels[q] = clusterID
			}
		}
		clusterID++
	}

	return labels, nil
}

// regionQuery returns the indices of all points within epsilon distance of point idx.
func (d *DBSCAN) regionQuery(dist [][]float64, idx, n int) []int {
	var result []int
	for j := 0; j < n; j++ {
		if j != idx && dist[idx][j] <= d.epsilon {
			result = append(result, j)
		}
	}
	return result
}

// cosineDistance returns 1 - cosine_similarity(a, b).
// Returns 1.0 (maximum distance) if either vector is zero-length.
func cosineDistance(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		ai, bi := float64(a[i]), float64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	sim := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	// Clamp to [-1, 1] to handle floating point drift
	if sim > 1.0 {
		sim = 1.0
	} else if sim < -1.0 {
		sim = -1.0
	}
	return 1.0 - sim
}
