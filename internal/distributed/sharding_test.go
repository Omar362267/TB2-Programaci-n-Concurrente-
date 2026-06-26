package distributed

import (
	"path/filepath"
	"testing"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

func samples(n int) []features.Sample {
	out := make([]features.Sample, n)
	for i := range out {
		out[i] = features.Sample{X: []float64{float64(i)}, Y: i % 2}
	}
	return out
}

func TestSplitTrainTestDeterministicAndComplete(t *testing.T) {
	all := samples(10)
	cfg := SplitConfig{TestRatio: 0.2, Seed: 42}
	trainA, testA, err := SplitTrainTest(all, cfg)
	if err != nil {
		t.Fatal(err)
	}
	trainB, testB, err := SplitTrainTest(all, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(trainA) != 8 || len(testA) != 2 {
		t.Fatalf("split = %d/%d, want 8/2", len(trainA), len(testA))
	}
	if len(trainA)+len(testA) != len(all) {
		t.Fatal("se perdieron muestras")
	}
	for i := range trainA {
		if trainA[i].X[0] != trainB[i].X[0] {
			t.Fatal("split no determinista")
		}
	}
	for i := range testA {
		if testA[i].X[0] != testB[i].X[0] {
			t.Fatal("split no determinista")
		}
	}
}

func TestPartitionRoundRobinBalancedAndComplete(t *testing.T) {
	train := samples(10)
	shards, err := PartitionRoundRobin(train, 4)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	seen := map[float64]bool{}
	for _, shard := range shards {
		total += len(shard)
		for _, s := range shard {
			id := s.X[0]
			if seen[id] {
				t.Fatalf("muestra duplicada: %v", id)
			}
			seen[id] = true
		}
	}
	if total != 10 || len(seen) != 10 {
		t.Fatalf("cobertura %d/%d", total, len(seen))
	}
	if got := MaxMinShardDifference(shards); got != 1 {
		t.Fatalf("diferencia de shards = %d, want 1", got)
	}
}

func TestWriteSamplesCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "node-01-train.csv")
	err := WriteSamplesCSV(path, []features.Sample{{X: []float64{0.1, 0.2}, Y: 1}}, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	hash, err := FileSHA256(path)
	if err != nil || hash == "" {
		t.Fatalf("hash invalido: %q %v", hash, err)
	}
}
