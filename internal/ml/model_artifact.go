package ml

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

// ModelArtifact es el contrato persistible del modelo para predicciones locales o remotas.
type ModelArtifact struct {
	ModelName        string              `json:"model_name"`
	ProblemType      string              `json:"problem_type"`
	Target           string              `json:"target"`
	FeatureNames     []string            `json:"feature_names"`
	ThresholdP75     float64             `json:"high_demand_threshold_p75"`
	Normalization    string              `json:"normalization"`
	Normalizer       features.Normalizer `json:"normalizer"`
	DecisionBoundary float64             `json:"decision_boundary"`
	Model            LogisticRegression  `json:"model"`
	TrainReport      TrainReport         `json:"train_report"`
	CreatedAt        string              `json:"created_at"`
	UsageNote        string              `json:"usage_note"`
}

// SaveArtifact guarda un modelo y los parametros necesarios para usarlo sin reentrenar.
func SaveArtifact(path string, artifact ModelArtifact) error {
	if err := artifact.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return fmt.Errorf("serializando artefacto: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// LoadArtifact recupera un modelo entrenado y valida su contrato.
func LoadArtifact(path string) (ModelArtifact, error) {
	var artifact ModelArtifact
	data, err := os.ReadFile(path)
	if err != nil {
		return artifact, err
	}
	if err := json.Unmarshal(data, &artifact); err != nil {
		return artifact, fmt.Errorf("leyendo artefacto JSON: %w", err)
	}
	if err := artifact.Validate(); err != nil {
		return artifact, err
	}
	return artifact, nil
}

// Validate comprueba que el modelo y normalizador usen el mismo orden de features.
func (a ModelArtifact) Validate() error {
	if len(a.Model.Weights) == 0 {
		return fmt.Errorf("artefacto sin pesos")
	}
	if len(a.Normalizer.Mins) != len(a.Model.Weights) || len(a.Normalizer.Maxs) != len(a.Model.Weights) {
		return fmt.Errorf("normalizador incompatible con pesos del modelo")
	}
	if len(a.FeatureNames) != 0 && len(a.FeatureNames) != len(a.Model.Weights) {
		return fmt.Errorf("feature_names incompatible con pesos del modelo")
	}
	if a.DecisionBoundary <= 0 || a.DecisionBoundary >= 1 {
		return fmt.Errorf("decision boundary invalido")
	}
	return nil
}

// PredictRaw normaliza una observacion cruda con los parametros del entrenamiento
// y devuelve probabilidad y etiqueta sin requerir reentrenar el modelo.
func (a ModelArtifact) PredictRaw(rawFeatures []float64) (float64, int, error) {
	if err := a.Validate(); err != nil {
		return 0, 0, err
	}
	normalized, err := a.Normalizer.TransformVector(rawFeatures)
	if err != nil {
		return 0, 0, err
	}
	probability := a.Model.PredictProbability(normalized)
	label := 0
	if probability >= a.DecisionBoundary {
		label = 1
	}
	return probability, label, nil
}
