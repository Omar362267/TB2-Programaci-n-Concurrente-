package distributed

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTinyShard(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "shard.csv")
	content := "hour,voltage,high_demand\n0.1,0.5,0\n0.9,0.7,1\n0.5,0.6,0\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNodeServerHandleHealthAndGradient(t *testing.T) {
	server, err := NewNodeServer(NodeConfig{NodeID: "test-node", Address: "127.0.0.1:0", ShardPath: writeTinyShard(t), Workers: 2})
	if err != nil {
		t.Fatal(err)
	}
	health := server.Handle(Request{Type: MessageHealth, RequestID: "h"})
	if health.Type != MessageHealthResponse || health.Samples != 3 || health.Status != "ready" {
		t.Fatalf("health inesperado: %+v", health)
	}
	response := server.Handle(Request{Type: MessageGradient, RequestID: "g", Weights: []float64{0, 0}})
	if response.Type != MessageGradientResult || response.Samples != 3 || len(response.Gradient) != 2 {
		t.Fatalf("gradiente inesperado: %+v", response)
	}
}

func TestNodeServerTCP(t *testing.T) {
	server, err := NewNodeServer(NodeConfig{NodeID: "tcp-node", Address: "127.0.0.1:0", ShardPath: writeTinyShard(t), Workers: 1})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case <-time.After(2 * time.Second):
		t.Fatal("listener no inició")
	}
	requestCtx, cancelReq := context.WithTimeout(context.Background(), time.Second)
	defer cancelReq()
	response, err := SendRequest(requestCtx, server.Addr(), Request{Type: MessageHealth, RequestID: "tcp-health"})
	if err != nil {
		t.Fatal(err)
	}
	if response.NodeID != "tcp-node" || response.Type != MessageHealthResponse {
		t.Fatalf("respuesta inesperada: %+v", response)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("servidor no cerró")
	}
}
