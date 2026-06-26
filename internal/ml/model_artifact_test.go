package ml

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

func TestArtifactRoundTripAndRawPrediction(t *testing.T) {
	normalizer, err := features.FitMinMax([]features.Sample{
		{X: []float64{0, 10}},
		{X: []float64{10, 20}},
	})
	if err != nil {
		t.Fatal(err)
	}
	artifact := ModelArtifact{
		ModelName:        "test",
		ProblemType:      "clasificacion_binaria",
		FeatureNames:     []string{"f0", "f1"},
		Normalizer:       normalizer,
		DecisionBoundary: 0.5,
		Model: LogisticRegression{
			Weights: []float64{2, -1},
			Bias:    0.1,
		},
	}

	path := filepath.Join(t.TempDir(), "model.json")
	if err := SaveArtifact(path, artifact); err != nil {
		t.Fatalf("SaveArtifact() error = %v", err)
	}
	loaded, err := LoadArtifact(path)
	if err != nil {
		t.Fatalf("LoadArtifact() error = %v", err)
	}

	raw := []float64{10, 10}
	probability, label, err := loaded.PredictRaw(raw)
	if err != nil {
		t.Fatalf("PredictRaw() error = %v", err)
	}
	normalized, _ := normalizer.TransformVector(raw)
	expected := artifact.Model.PredictProbability(normalized)
	if math.Abs(probability-expected) > 1e-12 {
		t.Fatalf("probability = %.15f, want %.15f", probability, expected)
	}
	if label != 1 {
		t.Fatalf("label = %d, want 1", label)
	}
}

func TestTrainParallelProducesUsableModel(t *testing.T) {
	samples := []features.Sample{
		{X: []float64{0, 0}, Y: 0},
		{X: []float64{0.1, 0.2}, Y: 0},
		{X: []float64{0.8, 0.9}, Y: 1},
		{X: []float64{1, 1}, Y: 1},
	}
	model, report := TrainParallel(samples, TrainConfig{Iterations: 100, LearningRate: 0.8, Workers: 2})
	if len(model.Weights) != 2 {
		t.Fatalf("weights = %d, want 2", len(model.Weights))
	}
	if report.FinalLoss >= report.InitialLoss {
		t.Fatalf("loss did not improve: initial=%f final=%f", report.InitialLoss, report.FinalLoss)
	}
	if model.PredictLabel([]float64{1, 1}) != 1 {
		t.Fatalf("trained model did not classify positive example")
	}
}
