package api

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
)

// TrainRequest defines a synchronous distributed training job.
// The API keeps the model update centralised: nodes only return immutable partial gradients.
type TrainRequest struct {
	Iterations   int     `json:"iterations"`
	LearningRate float64 `json:"learning_rate"`
	ResetModel   bool    `json:"reset_model"`
}

// DistributedTrainReport is persisted in the model artifact and returned by POST /v1/train.
type DistributedTrainReport struct {
	StartedAt           string               `json:"started_at"`
	FinishedAt          string               `json:"finished_at"`
	DurationMS          int64                `json:"duration_ms"`
	Iterations          int                  `json:"iterations"`
	LearningRate        float64              `json:"learning_rate"`
	NodesUsed           int                  `json:"nodes_used"`
	SamplesPerIteration int                  `json:"samples_per_iteration"`
	InitialLoss         float64              `json:"initial_loss"`
	FinalLoss           float64              `json:"final_loss"`
	History             []ml.IterationMetric `json:"history"`
	Parallelism         string               `json:"parallelism"`
}

// ModelSnapshot is a safe representation used by GET /v1/model and GET /v1/metrics.
type ModelSnapshot struct {
	Ready            bool                    `json:"ready"`
	Version          int                     `json:"version"`
	FeatureNames     []string                `json:"feature_names"`
	DecisionBoundary float64                 `json:"decision_boundary"`
	Weights          []float64               `json:"weights,omitempty"`
	Bias             float64                 `json:"bias,omitempty"`
	LastTraining     *DistributedTrainReport `json:"last_training,omitempty"`
	ArtifactPath     string                  `json:"artifact_path,omitempty"`
}

// ConfigureModel loads the Fase 1 artifact. The artifact carries feature order,
// normalizer and threshold so later prediction endpoints use the same contract.
func (s *Service) ConfigureModel(artifact ml.ModelArtifact, artifactPath string) error {
	if err := artifact.Validate(); err != nil {
		return err
	}
	s.modelMu.Lock()
	defer s.modelMu.Unlock()
	s.artifact = artifact
	s.artifactPath = artifactPath
	s.model = cloneModel(artifact.Model)
	s.modelVersion = 1
	s.modelConfigured = true
	return nil
}

func cloneModel(in ml.LogisticRegression) ml.LogisticRegression {
	return ml.LogisticRegression{Weights: append([]float64(nil), in.Weights...), Bias: in.Bias}
}

func (s *Service) ModelSnapshot() ModelSnapshot {
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()
	snapshot := ModelSnapshot{
		Ready:            s.modelConfigured,
		Version:          s.modelVersion,
		FeatureNames:     append([]string(nil), s.artifact.FeatureNames...),
		DecisionBoundary: s.artifact.DecisionBoundary,
		Weights:          append([]float64(nil), s.model.Weights...),
		Bias:             s.model.Bias,
		ArtifactPath:     s.artifactPath,
	}
	if s.lastTraining != nil {
		copied := *s.lastTraining
		copied.History = append([]ml.IterationMetric(nil), s.lastTraining.History...)
		snapshot.LastTraining = &copied
	}
	return snapshot
}

// TrainDistributed sends the current model to every ready node on each iteration,
// waits for all partial results, reduces them, then updates the global model once.
func (s *Service) TrainDistributed(ctx context.Context, request TrainRequest) (DistributedTrainReport, error) {
	if request.Iterations <= 0 {
		request.Iterations = 30
	}
	if request.Iterations > 1000 {
		return DistributedTrainReport{}, fmt.Errorf("iterations no puede superar 1000")
	}
	if request.LearningRate <= 0 || math.IsNaN(request.LearningRate) || math.IsInf(request.LearningRate, 0) {
		return DistributedTrainReport{}, fmt.Errorf("learning_rate debe ser un número positivo")
	}

	s.trainMu.Lock()
	defer s.trainMu.Unlock()

	cluster := s.CheckCluster(ctx)
	if cluster.Status != "ready" {
		return DistributedTrainReport{}, fmt.Errorf("entrenamiento requiere cluster ready; estado actual: %s (%d/%d nodos disponibles)", cluster.Status, cluster.NodesAvailable, cluster.NodesTotal)
	}

	s.modelMu.RLock()
	if !s.modelConfigured {
		s.modelMu.RUnlock()
		return DistributedTrainReport{}, fmt.Errorf("modelo no configurado; inicie la API con -model")
	}
	model := cloneModel(s.model)
	artifact := s.artifact
	s.modelMu.RUnlock()

	if len(model.Weights) != cluster.FeatureCount {
		return DistributedTrainReport{}, fmt.Errorf("modelo con %d features y cluster con %d", len(model.Weights), cluster.FeatureCount)
	}
	if request.ResetModel {
		model = ml.LogisticRegression{Weights: make([]float64, cluster.FeatureCount)}
	}

	started := time.Now()
	report := DistributedTrainReport{
		StartedAt:           started.Format(time.RFC3339),
		Iterations:          request.Iterations,
		LearningRate:        request.LearningRate,
		NodesUsed:           cluster.NodesAvailable,
		SamplesPerIteration: cluster.TotalSamples,
		Parallelism:         "API coordinadora consulta nodos TCP en paralelo; cada nodo calcula gradientes locales con goroutines; la API reduce resultados y actualiza pesos en una única sección crítica",
		History:             make([]ml.IterationMetric, 0, request.Iterations),
	}

	for iteration := 1; iteration <= request.Iterations; iteration++ {
		aggregate, err := s.computeGlobalGradient(ctx, model, iteration)
		if err != nil {
			return DistributedTrainReport{}, fmt.Errorf("iteración %d: %w", iteration, err)
		}
		loss := aggregate.LossSum / float64(aggregate.Samples)
		if iteration == 1 {
			report.InitialLoss = loss
		}
		// All reductions happen before this sole global update.
		for i := range model.Weights {
			model.Weights[i] -= request.LearningRate * (aggregate.Weights[i] / float64(aggregate.Samples))
		}
		model.Bias -= request.LearningRate * (aggregate.Bias / float64(aggregate.Samples))
		report.FinalLoss = loss
		report.History = append(report.History, ml.IterationMetric{Iteration: iteration, Loss: loss})
	}
	report.FinishedAt = time.Now().Format(time.RFC3339)
	report.DurationMS = time.Since(started).Milliseconds()

	// Persist the updated model atomically from the API's point of view.
	s.modelMu.Lock()
	artifact.Model = cloneModel(model)
	artifact.CreatedAt = time.Now().Format(time.RFC3339)
	artifact.TrainReport = ml.TrainReport{
		Config:          ml.TrainConfig{Iterations: request.Iterations, LearningRate: request.LearningRate, Workers: cluster.NodesAvailable},
		InitialLoss:     report.InitialLoss,
		FinalLoss:       report.FinalLoss,
		TrainSamples:    report.SamplesPerIteration,
		FeatureCount:    len(model.Weights),
		TrainingHistory: append([]ml.IterationMetric(nil), report.History...),
		Parallelism:     report.Parallelism,
	}
	s.model = cloneModel(model)
	s.artifact = artifact
	s.modelVersion++
	s.lastTraining = &report
	artifactPath := s.artifactPath
	s.modelMu.Unlock()

	if artifactPath != "" {
		if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
			return DistributedTrainReport{}, fmt.Errorf("creando directorio de modelo: %w", err)
		}
		if err := ml.SaveArtifact(artifactPath, artifact); err != nil {
			return DistributedTrainReport{}, fmt.Errorf("guardando modelo distribuido: %w", err)
		}
	}
	// Storage is auxiliary: model training remains valid even if a history backend is temporarily unavailable.
	if s.storage != nil {
		_ = s.storage.SaveTraining(ctx, s.ModelSnapshot().Version, report, s.ModelSnapshot())
	}
	return report, nil
}

func (s *Service) computeGlobalGradient(ctx context.Context, model ml.LogisticRegression, iteration int) (ml.GradientResult, error) {
	type result struct {
		endpoint NodeEndpoint
		response distributed.Response
		err      error
	}
	channel := make(chan result, len(s.nodes))
	var wg sync.WaitGroup
	for _, endpoint := range s.nodes {
		wg.Add(1)
		go func(endpoint NodeEndpoint) {
			defer wg.Done()
			requestCtx, cancel := context.WithTimeout(ctx, s.nodeTimeout)
			defer cancel()
			response, err := distributed.SendRequest(requestCtx, endpoint.Address, distributed.Request{
				Type:      distributed.MessageGradient,
				RequestID: fmt.Sprintf("train-%d-%s", iteration, endpoint.ID),
				Iteration: iteration,
				Weights:   append([]float64(nil), model.Weights...),
				Bias:      model.Bias,
			})
			channel <- result{endpoint: endpoint, response: response, err: err}
		}(endpoint)
	}
	wg.Wait()
	close(channel)

	aggregate := ml.GradientResult{Weights: make([]float64, len(model.Weights))}
	seen := make(map[string]struct{}, len(s.nodes))
	for result := range channel {
		if result.err != nil {
			return ml.GradientResult{}, fmt.Errorf("nodo %s (%s): %w", result.endpoint.ID, result.endpoint.Address, result.err)
		}
		if result.response.NodeID != result.endpoint.ID {
			return ml.GradientResult{}, fmt.Errorf("respuesta de node_id inesperado: %s", result.response.NodeID)
		}
		if result.response.Type != distributed.MessageGradientResult {
			return ml.GradientResult{}, fmt.Errorf("nodo %s respondió tipo %s", result.endpoint.ID, result.response.Type)
		}
		if _, duplicate := seen[result.response.NodeID]; duplicate {
			return ml.GradientResult{}, fmt.Errorf("respuesta duplicada del nodo %s", result.response.NodeID)
		}
		seen[result.response.NodeID] = struct{}{}
		if result.response.Samples <= 0 || len(result.response.Gradient) != len(aggregate.Weights) {
			return ml.GradientResult{}, fmt.Errorf("respuesta de gradiente inválida de %s", result.endpoint.ID)
		}
		for i := range aggregate.Weights {
			aggregate.Weights[i] += result.response.Gradient[i]
		}
		aggregate.Bias += result.response.GradientBias
		aggregate.LossSum += result.response.LossSum
		aggregate.Samples += result.response.Samples
	}
	if len(seen) != len(s.nodes) || aggregate.Samples == 0 {
		return ml.GradientResult{}, fmt.Errorf("reducción incompleta: %d/%d nodos", len(seen), len(s.nodes))
	}
	return aggregate, nil
}
