package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/loader"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/metrics"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
)

type modelOutput struct {
	FeatureSummary features.Summary             `json:"feature_summary"`
	TrainReport    ml.TrainReport               `json:"train_report"`
	TestMetrics    metrics.ClassificationReport `json:"test_metrics"`
	TrainSamples   int                          `json:"train_samples"`
	TestSamples    int                          `json:"test_samples"`
	Model          ml.LogisticRegression        `json:"model"`
}

type benchmarkOutput struct {
	Performance metrics.PerformanceReport `json:"performance"`
	Notes       []string                  `json:"notes"`
}

type executionParameters struct {
	InputPath       string  `json:"input_path"`
	Workers         int     `json:"workers"`
	Limit           int     `json:"limit"`
	Iterations      int     `json:"iterations"`
	LearningRate    float64 `json:"learning_rate"`
	TestRatio       float64 `json:"test_ratio"`
	OutputDir       string  `json:"output_dir"`
	ProcessedOut    string  `json:"processed_out"`
	CreatedAt       string  `json:"created_at"`
	GoVersion       string  `json:"go_version"`
	OperatingSystem string  `json:"operating_system"`
	Architecture    string  `json:"architecture"`
}

func main() {
	input := flag.String("input", "data/raw/household_power_consumption.txt", "ruta del dataset original")
	workers := flag.Int("workers", runtime.NumCPU(), "numero de workers concurrentes")
	limit := flag.Int("limit", 0, "limite opcional de registros para pruebas; 0 procesa todo")
	out := flag.String("out", "results", "carpeta de salida de evidencias")
	processedOut := flag.String("processed-out", "data/processed", "carpeta para guardar dataset procesado")
	saveProcessed := flag.Bool("save-processed", true, "guarda las features procesadas en CSV")
	processedLimit := flag.Int("processed-limit", 0, "maximo de muestras procesadas a guardar; 0 guarda todas")
	iterations := flag.Int("iterations", 100, "iteraciones de entrenamiento de regresion logistica")
	learningRate := flag.Float64("lr", 0.8, "learning rate del modelo")
	testRatio := flag.Float64("test-ratio", 0.2, "proporcion para prueba, entre 0 y 0.5")
	flag.Parse()

	if *workers <= 0 {
		*workers = 1
	}
	if *testRatio <= 0 || *testRatio >= 0.5 {
		*testRatio = 0.2
	}

	if err := os.MkdirAll(*out, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*processedOut, 0755); err != nil {
		log.Fatal(err)
	}

	params := executionParameters{
		InputPath:       *input,
		Workers:         *workers,
		Limit:           *limit,
		Iterations:      *iterations,
		LearningRate:    *learningRate,
		TestRatio:       *testRatio,
		OutputDir:       *out,
		ProcessedOut:    *processedOut,
		CreatedAt:       time.Now().Format(time.RFC3339),
		GoVersion:       runtime.Version(),
		OperatingSystem: runtime.GOOS,
		Architecture:    runtime.GOARCH,
	}
	if err := writeJSON(filepath.Join(*out, "parametros_ejecucion.json"), params); err != nil {
		log.Fatal(err)
	}

	totalStart := time.Now()

	loadResult, err := loader.Run(loader.Config{
		InputPath: *input,
		Workers:   *workers,
		Limit:     *limit,
	})
	if err != nil {
		log.Fatal(err)
	}
	if err := writeJSON(filepath.Join(*out, "resumen_limpieza.json"), loadResult.Summary); err != nil {
		log.Fatal(err)
	}

	featureStart := time.Now()
	rawSamples, featureSummary := features.BuildSamples(loadResult.Records)
	if len(rawSamples) < 10 {
		log.Fatalf("registros validos insuficientes para entrenar: %d", len(rawSamples))
	}

	// El split ocurre antes de ajustar la normalizacion para evitar data leakage.
	rawTrainSamples, rawTestSamples, err := distributed.SplitTrainTest(rawSamples, distributed.SplitConfig{TestRatio: *testRatio, Seed: 42})
	if err != nil {
		log.Fatalf("no se pudo separar train/test: %v", err)
	}
	normalizer, err := features.FitMinMax(rawTrainSamples)
	if err != nil {
		log.Fatalf("no se pudo ajustar normalizacion con train: %v", err)
	}
	trainSamples, err := normalizer.TransformSamples(rawTrainSamples)
	if err != nil {
		log.Fatalf("no se pudo normalizar train: %v", err)
	}
	testSamples, err := normalizer.TransformSamples(rawTestSamples)
	if err != nil {
		log.Fatalf("no se pudo normalizar test: %v", err)
	}
	featureDuration := time.Since(featureStart)

	if *saveProcessed {
		// Se guarda el dataset completo usando parametros ajustados solo con train.
		normalizedAll, err := normalizer.TransformSamples(rawSamples)
		if err != nil {
			log.Fatalf("no se pudo normalizar dataset procesado: %v", err)
		}
		processedPath := filepath.Join(*processedOut, "features_high_demand.csv")
		if err := writeProcessedSamplesCSV(processedPath, normalizedAll, *processedLimit); err != nil {
			log.Fatal(err)
		}
	}

	trainStart := time.Now()
	model, trainReport := ml.TrainParallel(trainSamples, ml.TrainConfig{
		Iterations:   *iterations,
		LearningRate: *learningRate,
		Workers:      *workers,
	})
	trainDuration := time.Since(trainStart)

	predicted := model.PredictBatch(testSamples)
	actual := ml.ActualLabels(testSamples)
	testMetrics := metrics.ComputeClassification(actual, predicted)

	modelResult := modelOutput{
		FeatureSummary: featureSummary,
		TrainReport:    trainReport,
		TestMetrics:    testMetrics,
		TrainSamples:   len(trainSamples),
		TestSamples:    len(testSamples),
		Model:          *model,
	}
	if err := writeJSON(filepath.Join(*out, "metricas_modelo.json"), modelResult); err != nil {
		log.Fatal(err)
	}

	artifact := ml.ModelArtifact{
		ModelName:        "logistic_regression_high_demand",
		ProblemType:      "clasificacion_binaria",
		Target:           featureSummary.TargetDefinition,
		FeatureNames:     featureSummary.FeatureNames,
		ThresholdP75:     featureSummary.HighDemandThreshold,
		Normalization:    featureSummary.Normalization,
		Normalizer:       normalizer,
		DecisionBoundary: 0.5,
		Model:            *model,
		TrainReport:      trainReport,
		CreatedAt:        time.Now().Format(time.RFC3339),
		UsageNote:        "Las features recibidas por una API deben conservar el orden feature_names y normalizarse con normalizer (ajustado solo con train) antes de aplicar sigmoid(bias + sum(weights[i]*x[i])). Si probabilidad >= decision_boundary, high_demand=1.",
	}
	if err := ml.SaveArtifact(filepath.Join(*out, "modelo_entrenado.json"), artifact); err != nil {
		log.Fatal(err)
	}

	predPath := filepath.Join(*out, "predicciones_muestra.csv")
	if err := writePredictionsCSV(predPath, testSamples, predicted, 100); err != nil {
		log.Fatal(err)
	}

	totalDuration := time.Since(totalStart)
	performance := metrics.PerformanceReport{
		Workers:           *workers,
		RowsProcessed:     loadResult.Summary.TotalRows,
		LoadDurationMS:    loadResult.Summary.DurationMS,
		FeatureDurationMS: featureDuration.Milliseconds(),
		TrainDurationMS:   trainDuration.Milliseconds(),
		TotalDurationMS:   totalDuration.Milliseconds(),
	}
	if totalDuration.Seconds() > 0 {
		performance.RowsPerSecond = float64(loadResult.Summary.TotalRows) / totalDuration.Seconds()
	}

	bench := benchmarkOutput{
		Performance: performance,
		Notes: []string{
			"La carga y limpieza se ejecutan con patron productor/consumidor usando goroutines y channels.",
			"El entrenamiento calcula gradientes parciales en paralelo y luego los reduce en el coordinador.",
			"Para comparar rendimiento, ejecutar el mismo comando con -workers 1, 2, 4 y 8 manteniendo el mismo -limit, -iterations y -lr.",
			"El entrenamiento puede ser rapido porque es regresion logistica con 11 features, no una red neuronal profunda; la evidencia se valida con conteos, tiempos, loss inicial/final y archivo del modelo.",
		},
	}
	if err := writeJSON(filepath.Join(*out, "benchmark_concurrencia.json"), bench); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Ejecucion PC3 completada")
	fmt.Printf("Registros leidos: %d\n", loadResult.Summary.TotalRows)
	fmt.Printf("Registros validos: %d\n", loadResult.Summary.ValidRows)
	fmt.Printf("Registros descartados: %d\n", loadResult.Summary.InvalidRows)
	fmt.Printf("Workers usados: %d\n", *workers)
	fmt.Printf("Muestras entrenamiento: %d\n", len(trainSamples))
	fmt.Printf("Muestras prueba: %d\n", len(testSamples))
	fmt.Printf("Loss inicial: %.6f | Loss final: %.6f\n", trainReport.InitialLoss, trainReport.FinalLoss)
	fmt.Printf("Accuracy: %.4f | Precision: %.4f | Recall: %.4f | F1: %.4f\n", testMetrics.Accuracy, testMetrics.Precision, testMetrics.Recall, testMetrics.F1Score)
	fmt.Printf("Modelo guardado en: %s\n", filepath.Join(*out, "modelo_entrenado.json"))
	if *saveProcessed {
		fmt.Printf("Dataset procesado guardado en: %s\n", filepath.Join(*processedOut, "features_high_demand.csv"))
	}
	fmt.Printf("Resultados generados en: %s\n", *out)
}

func writeJSON(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

func writeProcessedSamplesCSV(path string, samples []features.Sample, limit int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := append([]string{}, features.FeatureNames...)
	header = append(header, "high_demand")
	if err := writer.Write(header); err != nil {
		return err
	}

	max := len(samples)
	if limit > 0 && limit < max {
		max = limit
	}
	for i := 0; i < max; i++ {
		row := make([]string, 0, len(samples[i].X)+1)
		for _, v := range samples[i].X {
			row = append(row, strconv.FormatFloat(v, 'f', 8, 64))
		}
		row = append(row, strconv.Itoa(samples[i].Y))
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writePredictionsCSV(path string, samples []features.Sample, predicted []int, limit int) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if err := writer.Write([]string{"index", "actual_high_demand", "predicted_high_demand"}); err != nil {
		return err
	}
	max := len(samples)
	if len(predicted) < max {
		max = len(predicted)
	}
	if limit > 0 && limit < max {
		max = limit
	}
	for i := 0; i < max; i++ {
		row := []string{strconv.Itoa(i), strconv.Itoa(samples[i].Y), strconv.Itoa(predicted[i])}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return writer.Error()
}
