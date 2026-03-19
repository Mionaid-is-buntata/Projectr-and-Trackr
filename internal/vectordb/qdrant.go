package vectordb

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/yourname/projctr/internal/config"
	"google.golang.org/grpc"
)

// Client wraps the Qdrant client for Projctr's vector collections.
type Client struct {
	client   *qdrant.Client
	cfg      config.QdrantConfig
	descColl string
}

// New creates a Qdrant client and ensures collections exist.
func New(cfg config.QdrantConfig) (*Client, error) {
	port := cfg.Port
	if port == 0 {
		port = 6334
	}
	// Slow links (e.g. Qdrant linux/amd64 under QEMU on ARM) need a longer gRPC
	// handshake than the default. PoolSize 1 avoids three parallel dials.
	client, err := qdrant.NewClient(&qdrant.Config{
		Host:                   cfg.Host,
		Port:                   port,
		PoolSize:               1,
		SkipCompatibilityCheck: true,
		GrpcOptions: []grpc.DialOption{
			grpc.WithConnectParams(grpc.ConnectParams{
				MinConnectTimeout: 90 * time.Second,
			}),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("qdrant client: %w", err)
	}

	descColl := cfg.DescriptionCollection
	if descColl == "" {
		descColl = "projctr_descriptions"
	}

	c := &Client{client: client, cfg: cfg, descColl: descColl}
	initCtx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := c.ensureDescriptionCollection(initCtx); err != nil {
		client.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) ensureDescriptionCollection(ctx context.Context) error {
	exists, err := c.client.CollectionExists(ctx, c.descColl)
	if err != nil {
		return fmt.Errorf("check collection: %w", err)
	}
	if exists {
		return nil
	}
	dim := uint64(c.cfg.VectorDimensions)
	if dim == 0 {
		dim = 384
	}
	err = c.client.CreateCollection(ctx, &qdrant.CreateCollection{
		CollectionName: c.descColl,
		VectorsConfig:  qdrant.NewVectorsConfig(&qdrant.VectorParams{Size: dim, Distance: qdrant.Distance_Cosine}),
	})
	if err != nil {
		return fmt.Errorf("create collection %s: %w", c.descColl, err)
	}
	return nil
}

// IsSimilarDescription returns true if the embedding is within threshold of any
// existing description in the collection. Threshold: cosine similarity > 0.95.
func (c *Client) IsSimilarDescription(ctx context.Context, embedding []float32) (bool, error) {
	if len(embedding) == 0 {
		return false, nil
	}
	limit := uint64(1)
	result, err := c.client.Query(ctx, &qdrant.QueryPoints{
		CollectionName: c.descColl,
		Query:          qdrant.NewQuery(embedding...),
		Limit:          &limit,
		WithPayload:    qdrant.NewWithPayload(false),
	})
	if err != nil {
		return false, err
	}
	if len(result) == 0 {
		return false, nil
	}
	// Qdrant returns cosine similarity as score (0-1 for cosine)
	score := result[0].Score
	return score > 0.95, nil
}

// UpsertDescription stores a description embedding. id is the SQLite description ID.
func (c *Client) UpsertDescription(ctx context.Context, id int64, embedding []float32) error {
	if len(embedding) == 0 {
		return nil
	}
	_, err := c.client.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: c.descColl,
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewIDNum(uint64(id)),
				Vectors: qdrant.NewVectors(embedding...),
				Payload: qdrant.NewValueMap(map[string]any{"description_id": strconv.FormatInt(id, 10)}),
			},
		},
	})
	return err
}

// Close closes the Qdrant client.
func (c *Client) Close() error {
	return c.client.Close()
}
