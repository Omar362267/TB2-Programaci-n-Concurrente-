package ml

import (
	"fmt"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

type GradientResult struct {
	Weights []float64 `json:"gradient_weights"`
	Bias    float64   `json:"gradient_bias"`
	LossSum float64   `json:"loss_sum"`
	Samples int       `json:"samples"`
}

func ComputeGradientPartial(samples []features.Sample, model LogisticRegression, workers int) (GradientResult, error) {
	if len(samples) == 0 {
		return GradientResult{}, fmt.Errorf("no se puede calcular gradiente con cero muestras")
	}
	if len(model.Weights) == 0 {
		return GradientResult{}, fmt.Errorf("modelo sin pesos")
	}
	if workers <= 0 {
		workers = 1
	}
	for i, sample := range samples {
		if len(sample.X) != len(model.Weights) {
			return GradientResult{}, fmt.Errorf("muestra %d tiene %d features; modelo espera %d", i, len(sample.X), len(model.Weights))
		}
	}

	partials := computeGradientParallel(samples, &model, workers)
	result := GradientResult{Weights: make([]float64, len(model.Weights))}
	for _, p := range partials {
		for j := range result.Weights {
			result.Weights[j] += p.weights[j]
		}
		result.Bias += p.bias
		result.LossSum += p.loss
		result.Samples += p.count
	}
	return result, nil
}
