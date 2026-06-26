package distributed

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
)

// NodeConfig defines the process-local characteristics of one distributed ML node.
type NodeConfig struct {
	NodeID    string
	Address   string
	ShardPath string
	Workers   int
}

// NodeServer owns an immutable local shard and serves TCP requests.
type NodeServer struct {
	nodeID       string
	address      string
	workers      int
	samples      []features.Sample
	featureNames []string
	listener     net.Listener
	listenerMu   sync.RWMutex
	ready        chan struct{}
	once         sync.Once
}

func NewNodeServer(cfg NodeConfig) (*NodeServer, error) {
	if cfg.NodeID == "" {
		return nil, fmt.Errorf("node_id es obligatorio")
	}
	if cfg.Address == "" {
		return nil, fmt.Errorf("address es obligatorio")
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	samples, names, err := ReadShardCSV(cfg.ShardPath)
	if err != nil {
		return nil, fmt.Errorf("cargando shard %s: %w", cfg.ShardPath, err)
	}
	return &NodeServer{nodeID: cfg.NodeID, address: cfg.Address, workers: cfg.Workers, samples: samples, featureNames: names, ready: make(chan struct{})}, nil
}

func (s *NodeServer) NodeID() string         { return s.nodeID }
func (s *NodeServer) Samples() int           { return len(s.samples) }
func (s *NodeServer) FeatureNames() []string { return append([]string(nil), s.featureNames...) }
func (s *NodeServer) Addr() string {
	s.listenerMu.RLock()
	defer s.listenerMu.RUnlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return s.address
}

// Ready is closed after the TCP listener has been created.
func (s *NodeServer) Ready() <-chan struct{} { return s.ready }

// Serve starts a TCP listener. Each connection carries exactly one JSON request line
// and receives exactly one JSON response line; this keeps protocol boundaries explicit.
func (s *NodeServer) Serve(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return err
	}
	s.listenerMu.Lock()
	s.listener = listener
	s.listenerMu.Unlock()
	close(s.ready)
	defer listener.Close()

	go func() { <-ctx.Done(); _ = listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return err
		}
		go s.handleConnection(conn)
	}
}

func (s *NodeServer) Close() {
	s.once.Do(func() {
		s.listenerMu.RLock()
		listener := s.listener
		s.listenerMu.RUnlock()
		if listener != nil {
			_ = listener.Close()
		}
	})
}

func (s *NodeServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(60 * time.Second))
	decoder := json.NewDecoder(bufio.NewReader(conn))
	encoder := json.NewEncoder(conn)
	var request Request
	if err := decoder.Decode(&request); err != nil {
		_ = encoder.Encode(Response{Type: MessageError, NodeID: s.nodeID, Error: "solicitud JSON invalida: " + err.Error()})
		return
	}
	response := s.Handle(request)
	_ = encoder.Encode(response)
}

// Handle is separated from networking so it can be tested without sockets.
func (s *NodeServer) Handle(request Request) Response {
	switch request.Type {
	case MessageHealth:
		return Response{Type: MessageHealthResponse, RequestID: request.RequestID, NodeID: s.nodeID, Status: "ready", Samples: len(s.samples), FeatureCount: len(s.featureNames)}
	case MessageGradient:
		if len(request.Weights) != len(s.featureNames) {
			return Response{Type: MessageError, RequestID: request.RequestID, NodeID: s.nodeID, Error: fmt.Sprintf("weights con %d elementos; se esperaban %d", len(request.Weights), len(s.featureNames))}
		}
		started := time.Now()
		result, err := ml.ComputeGradientPartial(s.samples, ml.LogisticRegression{Weights: append([]float64(nil), request.Weights...), Bias: request.Bias}, s.workers)
		if err != nil {
			return Response{Type: MessageError, RequestID: request.RequestID, NodeID: s.nodeID, Error: err.Error()}
		}
		return NewGradientResponse(request, s.nodeID, result, time.Since(started).Milliseconds())
	default:
		return Response{Type: MessageError, RequestID: request.RequestID, NodeID: s.nodeID, Error: "tipo de solicitud no soportado: " + request.Type}
	}
}
