package clustering

import "testing"

func TestDBSCAN_Run(t *testing.T) {
	tests := []struct {
		name       string
		minPts     int
		epsilon    float64
		vectors    [][]float32
		wantNil    bool
		wantLabels func(labels []int) bool // custom validator
		desc       string
	}{
		{
			name:    "empty input returns nil",
			minPts:  2,
			epsilon: 0.3,
			vectors: nil,
			wantNil: true,
		},
		{
			name:    "single point is noise",
			minPts:  2,
			epsilon: 0.3,
			vectors: [][]float32{{1, 0, 0}},
			wantLabels: func(labels []int) bool {
				return len(labels) == 1 && labels[0] == -1
			},
			desc: "single point should be noise (-1)",
		},
		{
			name:    "two clear clusters",
			minPts:  2,
			epsilon: 0.3,
			vectors: [][]float32{
				{1, 0, 0},
				{0.95, 0.05, 0},
				{0.9, 0.1, 0},
				{0, 1, 0},
				{0.05, 0.95, 0},
				{0.1, 0.9, 0},
			},
			wantLabels: func(labels []int) bool {
				if len(labels) != 6 {
					return false
				}
				// First 3 should share a cluster, last 3 should share a different cluster
				if labels[0] == -1 || labels[3] == -1 {
					return false
				}
				if labels[0] != labels[1] || labels[1] != labels[2] {
					return false
				}
				if labels[3] != labels[4] || labels[4] != labels[5] {
					return false
				}
				if labels[0] == labels[3] {
					return false
				}
				return true
			},
			desc: "should produce 2 distinct clusters",
		},
		{
			name:    "all noise with scattered vectors",
			minPts:  3,
			epsilon: 0.01,
			vectors: [][]float32{
				{1, 0, 0},
				{0, 1, 0},
				{0, 0, 1},
				{-1, 0, 0},
			},
			wantLabels: func(labels []int) bool {
				for _, l := range labels {
					if l != -1 {
						return false
					}
				}
				return true
			},
			desc: "all points should be noise (-1)",
		},
		{
			name:    "identical vectors form single cluster",
			minPts:  2,
			epsilon: 0.1,
			vectors: [][]float32{
				{1, 0, 0},
				{1, 0, 0},
				{1, 0, 0},
			},
			wantLabels: func(labels []int) bool {
				if len(labels) != 3 {
					return false
				}
				if labels[0] == -1 {
					return false
				}
				return labels[0] == labels[1] && labels[1] == labels[2]
			},
			desc: "identical vectors should form one cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := NewDBSCAN(tt.minPts, tt.epsilon)
			labels, err := db.Run(tt.vectors)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantNil {
				if labels != nil {
					t.Errorf("expected nil labels, got %v", labels)
				}
				return
			}
			if !tt.wantLabels(labels) {
				t.Errorf("label validation failed (%s): got %v", tt.desc, labels)
			}
		})
	}
}

func TestDBSCAN_Run_LargeInputGuard(t *testing.T) {
	db := NewDBSCAN(2, 0.3)
	// Create 5001 vectors to exceed the guard
	vectors := make([][]float32, 5001)
	for i := range vectors {
		vectors[i] = []float32{1, 0, 0}
	}
	labels, err := db.Run(vectors)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if labels != nil {
		t.Errorf("expected nil for n>5000, got %d labels", len(labels))
	}
}
