package metrics

import "math"

// ClassificationReport contiene metricas basicas para clasificacion binaria.
type ClassificationReport struct {
	Accuracy  float64 `json:"accuracy"`
	Precision float64 `json:"precision"`
	Recall    float64 `json:"recall"`
	F1Score   float64 `json:"f1_score"`
	TP        int     `json:"true_positive"`
	TN        int     `json:"true_negative"`
	FP        int     `json:"false_positive"`
	FN        int     `json:"false_negative"`
	Samples   int     `json:"samples"`
}

// PerformanceReport resume tiempos y rendimiento de ejecucion.
type PerformanceReport struct {
	Workers           int     `json:"workers"`
	RowsProcessed     int     `json:"rows_processed"`
	LoadDurationMS    int64   `json:"load_duration_ms"`
	FeatureDurationMS int64   `json:"feature_duration_ms"`
	TrainDurationMS   int64   `json:"train_duration_ms"`
	TotalDurationMS   int64   `json:"total_duration_ms"`
	RowsPerSecond     float64 `json:"rows_per_second"`
}

// ComputeClassification calcula accuracy, precision, recall y F1.
func ComputeClassification(actual []int, predicted []int) ClassificationReport {
	limit := len(actual)
	if len(predicted) < limit {
		limit = len(predicted)
	}

	report := ClassificationReport{Samples: limit}
	for i := 0; i < limit; i++ {
		switch {
		case actual[i] == 1 && predicted[i] == 1:
			report.TP++
		case actual[i] == 0 && predicted[i] == 0:
			report.TN++
		case actual[i] == 0 && predicted[i] == 1:
			report.FP++
		case actual[i] == 1 && predicted[i] == 0:
			report.FN++
		}
	}

	if limit == 0 {
		return report
	}

	report.Accuracy = float64(report.TP+report.TN) / float64(limit)
	report.Precision = safeDiv(float64(report.TP), float64(report.TP+report.FP))
	report.Recall = safeDiv(float64(report.TP), float64(report.TP+report.FN))
	report.F1Score = safeDiv(2*report.Precision*report.Recall, report.Precision+report.Recall)
	return report
}

func safeDiv(a, b float64) float64 {
	if b == 0 || math.IsNaN(b) {
		return 0
	}
	return a / b
}
