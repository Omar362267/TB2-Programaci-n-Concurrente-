package api

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/storage"
)

type PredictRequest struct {
	Features map[string]float64 `json:"features"`
}

type PredictionResult struct {
	ModelVersion          int                `json:"model_version"`
	ProbabilityHighDemand float64            `json:"probability_high_demand"`
	PredictedHighDemand   int                `json:"predicted_high_demand"`
	DecisionBoundary      float64            `json:"decision_boundary"`
	FeatureOrder          []string           `json:"feature_order"`
	Input                 map[string]float64 `json:"input"`
	LatencyMS             int64              `json:"latency_ms"`
	CacheHit              bool               `json:"cache_hit"`
	StorageWarning        string             `json:"storage_warning,omitempty"`
}

func (s *Service) Predict(ctx context.Context, request PredictRequest) (PredictionResult, error) {
	started := time.Now()
	if len(request.Features) == 0 {
		return PredictionResult{}, fmt.Errorf("features es obligatorio")
	}

	s.modelMu.RLock()
	configured := s.modelConfigured
	version := s.modelVersion
	artifact := s.artifact
	model := cloneModel(s.model)
	s.modelMu.RUnlock()
	if !configured {
		return PredictionResult{}, fmt.Errorf("modelo no configurado; inicie la API con -model")
	}
	artifact.Model = model
	if err := artifact.Validate(); err != nil {
		return PredictionResult{}, fmt.Errorf("modelo actual inválido: %w", err)
	}

	raw := make([]float64, len(artifact.FeatureNames))
	copiedInput := make(map[string]float64, len(artifact.FeatureNames))
	for i, name := range artifact.FeatureNames {
		value, exists := request.Features[name]
		if !exists {
			return PredictionResult{}, fmt.Errorf("feature faltante: %s", name)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return PredictionResult{}, fmt.Errorf("feature %s debe ser un número finito", name)
		}
		raw[i] = value
		copiedInput[name] = value
	}
	for name := range request.Features {
		if !containsFeature(artifact.FeatureNames, name) {
			return PredictionResult{}, fmt.Errorf("feature no reconocida: %s", name)
		}
	}

	cacheKey := storage.PredictionCacheKey(version, artifact.FeatureNames, copiedInput)
	if s.storage != nil && s.storage.RedisEnabled() {
		var cached PredictionResult
		found, err := s.storage.LoadCachedPrediction(ctx, cacheKey, &cached)
		if err == nil && found {
			cached.CacheHit = true
			cached.LatencyMS = time.Since(started).Milliseconds()
			if s.storage.MongoEnabled() {
				_ = s.storage.SavePrediction(ctx, version, copiedInput, cached, true)
			}
			return cached, nil
		}
	}

	probability, label, err := artifact.PredictRaw(raw)
	if err != nil {
		return PredictionResult{}, fmt.Errorf("predicción inválida: %w", err)
	}
	result := PredictionResult{ModelVersion: version, ProbabilityHighDemand: probability, PredictedHighDemand: label, DecisionBoundary: artifact.DecisionBoundary, FeatureOrder: append([]string(nil), artifact.FeatureNames...), Input: copiedInput, LatencyMS: time.Since(started).Milliseconds()}
	if s.storage != nil {
		if err := s.storage.CachePrediction(ctx, cacheKey, result); err != nil {
			result.StorageWarning = err.Error()
		}
		if err := s.storage.SavePrediction(ctx, version, copiedInput, result, false); err != nil && result.StorageWarning == "" {
			result.StorageWarning = err.Error()
		}
	}
	return result, nil
}

func containsFeature(names []string, candidate string) bool {
	for _, name := range names {
		if name == candidate {
			return true
		}
	}
	return false
}

var _ = ml.ModelArtifact{}
