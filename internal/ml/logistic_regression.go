package ml

import (
	"math"
	"sync"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

type LogisticRegression struct {
	Weights []float64 `json:"weights"`
	Bias    float64   `json:"bias"`
}

type TrainConfig struct {
	Iterations   int     `json:"iterations"`
	LearningRate float64 `json:"learning_rate"`
	Workers      int     `json:"workers"`
}

type IterationMetric struct {
	Iteration int     `json:"iteration"`
	Loss      float64 `json:"loss"`
}

type TrainReport struct {
	Config          TrainConfig       `json:"config"`
	InitialLoss     float64           `json:"initial_loss"`
	FinalLoss       float64           `json:"final_loss"`
	TrainSamples    int               `json:"train_samples"`
	FeatureCount    int               `json:"feature_count"`
	TrainingHistory []IterationMetric `json:"training_history"`
	Parallelism     string            `json:"parallelism"`
}

type gradientPartial struct {
	weights []float64
	bias    float64
	loss    float64
	count   int
}

func TrainParallel(samples []features.Sample, cfg TrainConfig) (*LogisticRegression, TrainReport) {
	if len(samples) == 0 {
		return &LogisticRegression{}, TrainReport{Config: cfg}
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.Iterations <= 0 {
		cfg.Iterations = 30
	}
	if cfg.LearningRate <= 0 {
		cfg.LearningRate = 0.1
	}

	featureCount := len(samples[0].X)
	model := &LogisticRegression{
		Weights: make([]float64, featureCount),
		Bias:    0,
	}

	initialLoss := computeLossParallel(samples, model, cfg.Workers)
	finalLoss := initialLoss
	history := make([]IterationMetric, 0, cfg.Iterations+1)
	history = append(history, IterationMetric{Iteration: 0, Loss: initialLoss})

	for iter := 1; iter <= cfg.Iterations; iter++ {
		partials := computeGradientParallel(samples, model, cfg.Workers)
		gradW := make([]float64, featureCount)
		gradB := 0.0
		total := 0
		loss := 0.0

		for _, p := range partials {
			for j := 0; j < featureCount; j++ {
				gradW[j] += p.weights[j]
			}
			gradB += p.bias
			loss += p.loss
			total += p.count
		}

		if total == 0 {
			break
		}

		for j := 0; j < featureCount; j++ {
			model.Weights[j] -= cfg.LearningRate * (gradW[j] / float64(total))
		}
		model.Bias -= cfg.LearningRate * (gradB / float64(total))
		finalLoss = loss / float64(total)
		history = append(history, IterationMetric{Iteration: iter, Loss: finalLoss})
	}

	report := TrainReport{
		Config:          cfg,
		InitialLoss:     initialLoss,
		FinalLoss:       finalLoss,
		TrainSamples:    len(samples),
		FeatureCount:    featureCount,
		TrainingHistory: history,
		Parallelism:     "gradientes parciales calculados con goroutines y reducidos por el coordinador sin estado compartido mutable",
	}
	return model, report
}

func (m *LogisticRegression) PredictProbability(x []float64) float64 {
	z := m.Bias
	for i, v := range x {
		if i < len(m.Weights) {
			z += m.Weights[i] * v
		}
	}
	return sigmoid(z)
}

func (m *LogisticRegression) PredictLabel(x []float64) int {
	if m.PredictProbability(x) >= 0.5 {
		return 1
	}
	return 0
}

func (m *LogisticRegression) PredictBatch(samples []features.Sample) []int {
	pred := make([]int, len(samples))
	for i, s := range samples {
		pred[i] = m.PredictLabel(s.X)
	}
	return pred
}

func ActualLabels(samples []features.Sample) []int {
	y := make([]int, len(samples))
	for i, s := range samples {
		y[i] = s.Y
	}
	return y
}

func computeGradientParallel(samples []features.Sample, model *LogisticRegression, workers int) []gradientPartial {
	if workers > len(samples) {
		workers = len(samples)
	}
	if workers <= 0 {
		workers = 1
	}

	partials := make([]gradientPartial, workers)
	chunk := int(math.Ceil(float64(len(samples)) / float64(workers)))
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		start := w * chunk
		end := start + chunk
		if end > len(samples) {
			end = len(samples)
		}
		if start >= len(samples) {
			partials[w] = gradientPartial{weights: make([]float64, len(model.Weights))}
			continue
		}

		wg.Add(1)
		go func(idx, start, end int) {
			defer wg.Done()
			p := gradientPartial{weights: make([]float64, len(model.Weights))}
			for _, s := range samples[start:end] {
				prob := model.PredictProbability(s.X)
				errorValue := prob - float64(s.Y)
				for j, x := range s.X {
					p.weights[j] += errorValue * x
				}
				p.bias += errorValue
				p.loss += logLoss(float64(s.Y), prob)
				p.count++
			}
			partials[idx] = p
		}(w, start, end)
	}

	wg.Wait()
	return partials
}

func computeLossParallel(samples []features.Sample, model *LogisticRegression, workers int) float64 {
	partials := computeGradientParallel(samples, model, workers)
	loss := 0.0
	count := 0
	for _, p := range partials {
		loss += p.loss
		count += p.count
	}
	if count == 0 {
		return 0
	}
	return loss / float64(count)
}

func sigmoid(z float64) float64 {
	if z >= 0 {
		e := math.Exp(-z)
		return 1 / (1 + e)
	}
	e := math.Exp(z)
	return e / (1 + e)
}

func logLoss(y, p float64) float64 {
	eps := 1e-15
	if p < eps {
		p = eps
	}
	if p > 1-eps {
		p = 1 - eps
	}
	return -(y*math.Log(p) + (1-y)*math.Log(1-p))
}
