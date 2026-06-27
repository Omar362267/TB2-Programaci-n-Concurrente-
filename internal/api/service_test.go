package api

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
)

func startHealthNode(t *testing.T, nodeID string, samples int) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				var request distributed.Request
				if err := json.NewDecoder(bufio.NewReader(c)).Decode(&request); err != nil {
					return
				}
				_ = json.NewEncoder(c).Encode(distributed.Response{
					Type: distributed.MessageHealthResponse, RequestID: request.RequestID,
					NodeID: nodeID, Status: "ready", Samples: samples, FeatureCount: 11,
				})
			}(conn)
		}
	}()
	return listener.Addr().String(), func() { _ = listener.Close(); <-done }
}

func TestCheckClusterReady(t *testing.T) {
	a1, close1 := startHealthNode(t, "ml-node-1", 100)
	defer close1()
	a2, close2 := startHealthNode(t, "ml-node-2", 120)
	defer close2()
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: a1}, {ID: "ml-node-2", Address: a2}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	status := service.CheckCluster(context.Background())
	if status.Status != "ready" || status.NodesAvailable != 2 || status.TotalSamples != 220 || status.FeatureCount != 11 {
		t.Fatalf("estado inesperado: %+v", status)
	}
}

func TestClusterStatusDegradedWhenNodeUnavailable(t *testing.T) {
	a1, close1 := startHealthNode(t, "ml-node-1", 50)
	defer close1()
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: a1}, {ID: "ml-node-2", Address: "127.0.0.1:1"}}, 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	status := service.CheckCluster(context.Background())
	if status.Status != "degraded" || status.NodesAvailable != 1 {
		t.Fatalf("estado inesperado: %+v", status)
	}
}

func TestHealthEndpoint(t *testing.T) {
	address, closeNode := startHealthNode(t, "ml-node-1", 10)
	defer closeNode()
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: address}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	NewServer(service).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status HTTP: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ready" {
		t.Fatalf("payload inesperado: %+v", payload)
	}
}

func startGradientNode(t *testing.T, nodeID string) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	shard := filepath.Join(dir, nodeID+".csv")
	content := "hour,voltage,high_demand\n0.1,0.2,0\n0.8,0.9,1\n0.5,0.4,0\n"
	if err := os.WriteFile(shard, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	server, err := distributed.NewNodeServer(distributed.NodeConfig{NodeID: nodeID, Address: "127.0.0.1:0", ShardPath: shard, Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(time.Second):
		t.Fatal("nodo no inició")
	}
	return server.Addr(), func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(time.Second):
			t.Fatal("nodo no cerró")
		}
	}
}

func TestTrainDistributedReducesAndUpdatesModel(t *testing.T) {
	a1, close1 := startGradientNode(t, "ml-node-1")
	defer close1()
	a2, close2 := startGradientNode(t, "ml-node-2")
	defer close2()
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: a1}, {ID: "ml-node-2", Address: a2}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	artifact := ml.ModelArtifact{
		FeatureNames: []string{"hour", "voltage"}, DecisionBoundary: 0.5,
		Normalizer: features.Normalizer{Mins: []float64{0, 0}, Maxs: []float64{1, 1}},
		Model:      ml.LogisticRegression{Weights: []float64{0, 0}},
	}
	if err := service.ConfigureModel(artifact, ""); err != nil {
		t.Fatal(err)
	}
	report, err := service.TrainDistributed(context.Background(), TrainRequest{Iterations: 3, LearningRate: 0.5, ResetModel: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.NodesUsed != 2 || report.SamplesPerIteration != 6 || len(report.History) != 3 {
		t.Fatalf("reporte inesperado: %+v", report)
	}
	snapshot := service.ModelSnapshot()
	if snapshot.Version != 2 || snapshot.Bias == 0 {
		t.Fatalf("modelo no actualizado: %+v", snapshot)
	}
	if report.FinalLoss >= report.InitialLoss {
		t.Fatalf("loss no descendió: %.6f -> %.6f", report.InitialLoss, report.FinalLoss)
	}
}

func TestPredictUsesLatestModelAndFeatureContract(t *testing.T) {
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: "127.0.0.1:1"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	artifact := ml.ModelArtifact{
		FeatureNames:     []string{"hour", "voltage"},
		DecisionBoundary: 0.5,
		Normalizer:       features.Normalizer{Mins: []float64{0, 200}, Maxs: []float64{24, 260}},
		Model:            ml.LogisticRegression{Weights: []float64{2, 1}, Bias: -1},
	}
	if err := service.ConfigureModel(artifact, ""); err != nil {
		t.Fatal(err)
	}
	result, err := service.Predict(context.Background(), PredictRequest{Features: map[string]float64{"hour": 24, "voltage": 260}})
	if err != nil {
		t.Fatal(err)
	}
	if result.ModelVersion != 1 || result.PredictedHighDemand != 1 || result.ProbabilityHighDemand <= 0.5 {
		t.Fatalf("resultado inesperado: %+v", result)
	}
	if _, err := service.Predict(context.Background(), PredictRequest{Features: map[string]float64{"hour": 10}}); err == nil {
		t.Fatal("se esperaba error por feature faltante")
	}
	if _, err := service.Predict(context.Background(), PredictRequest{Features: map[string]float64{"hour": 10, "voltage": 240, "extra": 1}}); err == nil {
		t.Fatal("se esperaba error por feature no reconocida")
	}
}

func TestPredictEndpoint(t *testing.T) {
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: "127.0.0.1:1"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	artifact := ml.ModelArtifact{
		FeatureNames: []string{"hour", "voltage"}, DecisionBoundary: 0.5,
		Normalizer: features.Normalizer{Mins: []float64{0, 0}, Maxs: []float64{1, 1}},
		Model:      ml.LogisticRegression{Weights: []float64{1, 1}, Bias: 0},
	}
	if err := service.ConfigureModel(artifact, ""); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/predict", strings.NewReader(`{"features":{"hour":1,"voltage":1}}`))
	request.Header.Set("Content-Type", "application/json")
	NewServer(service).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var result PredictionResult
	if err := json.NewDecoder(recorder.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.PredictedHighDemand != 1 {
		t.Fatalf("predicción inesperada: %+v", result)
	}
}

func TestEvaluateCurrentModelUsesHoldOutOnly(t *testing.T) {
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: "127.0.0.1:1"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	artifact := ml.ModelArtifact{
		FeatureNames:     []string{"hour", "voltage"},
		DecisionBoundary: 0.5,
		Normalizer:       features.Normalizer{Mins: []float64{0, 0}, Maxs: []float64{1, 1}},
		Model:            ml.LogisticRegression{Weights: []float64{8, 0}, Bias: -4},
	}
	if err := service.ConfigureModel(artifact, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.csv")
	data := "hour,voltage,high_demand\n0.0,0.1,0\n1.0,0.4,1\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.ConfigureEvaluation(path); err != nil {
		t.Fatal(err)
	}
	report, err := service.EvaluateCurrentModel(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Samples != 2 || report.Classification.Accuracy != 1 || report.ModelVersion != 1 || report.Loss <= 0 {
		t.Fatalf("reporte inesperado: %+v", report)
	}
}

func TestEvaluateEndpoint(t *testing.T) {
	service, err := NewService([]NodeEndpoint{{ID: "ml-node-1", Address: "127.0.0.1:1"}}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	artifact := ml.ModelArtifact{FeatureNames: []string{"hour"}, DecisionBoundary: 0.5, Normalizer: features.Normalizer{Mins: []float64{0}, Maxs: []float64{1}}, Model: ml.LogisticRegression{Weights: []float64{6}, Bias: -3}}
	if err := service.ConfigureModel(artifact, ""); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.csv")
	if err := os.WriteFile(path, []byte("hour,high_demand\n0,0\n1,1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := service.ConfigureEvaluation(path); err != nil {
		t.Fatal(err)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/evaluate", nil)
	NewServer(service).Routes().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
