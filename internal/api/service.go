// Package api contains the HTTP coordination layer for the distributed ML cluster.
package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/storage"
)

// NodeEndpoint identifies one ML node reachable through the internal TCP network.
type NodeEndpoint struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

// NodeStatus is a transport-safe snapshot of a node health response.
type NodeStatus struct {
	ID           string `json:"id"`
	Address      string `json:"address"`
	Status       string `json:"status"`
	Available    bool   `json:"available"`
	Samples      int    `json:"samples,omitempty"`
	FeatureCount int    `json:"feature_count,omitempty"`
	LatencyMS    int64  `json:"latency_ms"`
	Error        string `json:"error,omitempty"`
}

// ClusterStatus aggregates health observations from all configured nodes.
type ClusterStatus struct {
	Status         string       `json:"status"`
	NodesTotal     int          `json:"nodes_total"`
	NodesAvailable int          `json:"nodes_available"`
	TotalSamples   int          `json:"total_samples"`
	FeatureCount   int          `json:"feature_count,omitempty"`
	Nodes          []NodeStatus `json:"nodes"`
}

// Service owns immutable node configuration. Model coordination will be added in the next phase.
type Service struct {
	nodes       []NodeEndpoint
	nodeTimeout time.Duration

	// modelMu protects the API-owned global model. Nodes never mutate this state.
	modelMu         sync.RWMutex
	trainMu         sync.Mutex
	modelConfigured bool
	model           ml.LogisticRegression
	artifact        ml.ModelArtifact
	artifactPath    string
	modelVersion    int
	lastTraining    *DistributedTrainReport
	storage         *storage.Repository

	// evaluation data is immutable after startup and independent from training shards.
	evaluationMu      sync.RWMutex
	evaluationSamples []features.Sample
	evaluationPath    string
}

func NewService(nodes []NodeEndpoint, nodeTimeout time.Duration) (*Service, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("se requiere al menos un nodo ML")
	}
	if nodeTimeout <= 0 {
		nodeTimeout = 3 * time.Second
	}
	ids := make(map[string]struct{}, len(nodes))
	addresses := make(map[string]struct{}, len(nodes))
	copied := make([]NodeEndpoint, len(nodes))
	for i, node := range nodes {
		if node.ID == "" || node.Address == "" {
			return nil, fmt.Errorf("nodo %d requiere id y address", i+1)
		}
		if _, exists := ids[node.ID]; exists {
			return nil, fmt.Errorf("node id duplicado: %s", node.ID)
		}
		if _, exists := addresses[node.Address]; exists {
			return nil, fmt.Errorf("address duplicado: %s", node.Address)
		}
		ids[node.ID] = struct{}{}
		addresses[node.Address] = struct{}{}
		copied[i] = node
	}
	return &Service{nodes: copied, nodeTimeout: nodeTimeout}, nil
}

// ConfigureStorage attaches optional MongoDB/Redis infrastructure.
func (s *Service) ConfigureStorage(repo *storage.Repository) { s.storage = repo }

// StorageHealth exposes the health of optional persistence and cache backends.
func (s *Service) StorageHealth(ctx context.Context) storage.Health {
	if s.storage == nil {
		return storage.Health{MongoStatus: "disabled", RedisStatus: "disabled"}
	}
	return s.storage.Health(ctx)
}

func (s *Service) Nodes() []NodeEndpoint {
	return append([]NodeEndpoint(nil), s.nodes...)
}

// CheckCluster contacts every node concurrently over TCP. An unavailable node does not
// abort the whole response: it is reported explicitly so callers can observe degraded state.
func (s *Service) CheckCluster(ctx context.Context) ClusterStatus {
	nodes := make([]NodeStatus, len(s.nodes))
	var wg sync.WaitGroup
	for i, endpoint := range s.nodes {
		wg.Add(1)
		go func(index int, endpoint NodeEndpoint) {
			defer wg.Done()
			started := time.Now()
			requestCtx, cancel := context.WithTimeout(ctx, s.nodeTimeout)
			defer cancel()
			response, err := distributed.SendRequest(requestCtx, endpoint.Address, distributed.Request{
				Type:      distributed.MessageHealth,
				RequestID: fmt.Sprintf("api-health-%s-%d", endpoint.ID, started.UnixNano()),
			})
			status := NodeStatus{ID: endpoint.ID, Address: endpoint.Address, LatencyMS: time.Since(started).Milliseconds()}
			if err != nil {
				status.Status = "unavailable"
				status.Error = err.Error()
				nodes[index] = status
				return
			}
			if response.NodeID != "" && response.NodeID != endpoint.ID {
				status.Status = "unavailable"
				status.Error = fmt.Sprintf("node_id inesperado: se esperaba %s y respondió %s", endpoint.ID, response.NodeID)
				nodes[index] = status
				return
			}
			status.Status = response.Status
			status.Available = response.Status == "ready"
			status.Samples = response.Samples
			status.FeatureCount = response.FeatureCount
			nodes[index] = status
		}(i, endpoint)
	}
	wg.Wait()

	summary := ClusterStatus{NodesTotal: len(nodes), Nodes: nodes}
	featureCount := -1
	consistentFeatures := true
	for _, node := range nodes {
		if node.Available {
			summary.NodesAvailable++
			summary.TotalSamples += node.Samples
			if featureCount == -1 {
				featureCount = node.FeatureCount
			} else if featureCount != node.FeatureCount {
				consistentFeatures = false
			}
		}
	}
	if featureCount > 0 && consistentFeatures {
		summary.FeatureCount = featureCount
	}
	if summary.NodesAvailable == summary.NodesTotal {
		summary.Status = "ready"
	} else if summary.NodesAvailable > 0 {
		summary.Status = "degraded"
	} else {
		summary.Status = "unavailable"
	}
	return summary
}
