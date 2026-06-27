package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/analysis"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/loader"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/visualization"
)

type modelMetricsJSON struct {
	TrainReport struct {
		TrainingHistory []struct {
			Iteration int     `json:"iteration"`
			Loss      float64 `json:"loss"`
		} `json:"training_history"`
	} `json:"train_report"`
	TestMetrics struct {
		TP int `json:"true_positive"`
		TN int `json:"true_negative"`
		FP int `json:"false_positive"`
		FN int `json:"false_negative"`
	} `json:"test_metrics"`
}

func main() {
	input := flag.String("input", "data/raw/household_power_consumption.txt", "ruta del dataset original")
	workers := flag.Int("workers", 8, "workers para lectura/limpieza concurrente")
	limit := flag.Int("limit", 0, "limite opcional; 0 procesa todo")
	out := flag.String("out", "results/analysis", "carpeta de salida de tablas de analisis")
	figures := flag.String("figures", "results/figures", "carpeta de salida de graficos SVG")
	metricsPath := flag.String("metrics", "results/final_run/metricas_modelo.json", "metricas del modelo para matriz de confusion y curva loss")
	flag.Parse()

	if err := os.MkdirAll(*out, 0755); err != nil {
		log.Fatal(err)
	}
	if err := visualization.EnsureDir(*figures); err != nil {
		log.Fatal(err)
	}

	result, err := loader.Run(loader.Config{InputPath: *input, Workers: *workers, Limit: *limit})
	if err != nil {
		log.Fatal(err)
	}

	a := analysis.AnalyzeRecords(result.Records)
	if err := analysis.SaveAnalysis(*out, a); err != nil {
		log.Fatal(err)
	}

	if err := visualization.Histogram(filepath.Join(*figures, "distribucion_global_active_power.svg"), "Distribucion de Global_active_power", a.Histogram); err != nil {
		log.Fatal(err)
	}
	if err := visualization.BarChart(filepath.Join(*figures, "consumo_promedio_por_hora.svg"), "Consumo promedio por hora", "Hora", "Promedio Global_active_power (kW)", a.Hourly); err != nil {
		log.Fatal(err)
	}
	if err := visualization.BarChart(filepath.Join(*figures, "consumo_por_dia_semana.svg"), "Consumo promedio por dia de semana", "Dia", "Promedio Global_active_power (kW)", a.Weekday); err != nil {
		log.Fatal(err)
	}
	if err := visualization.BarChart(filepath.Join(*figures, "consumo_por_mes.svg"), "Consumo promedio por mes", "Mes", "Promedio Global_active_power (kW)", a.Monthly); err != nil {
		log.Fatal(err)
	}
	if err := visualization.Heatmap(filepath.Join(*figures, "matriz_correlacion.svg"), "Matriz de correlacion", a.CorrelationVariables, a.Correlation); err != nil {
		log.Fatal(err)
	}

	if m, err := readMetrics(*metricsPath); err == nil {
		_ = visualization.ConfusionMatrix(filepath.Join(*figures, "matriz_confusion.svg"), m.TestMetrics.TP, m.TestMetrics.TN, m.TestMetrics.FP, m.TestMetrics.FN)
		labels := make([]string, 0, len(m.TrainReport.TrainingHistory))
		values := make([]float64, 0, len(m.TrainReport.TrainingHistory))
		for _, row := range m.TrainReport.TrainingHistory {
			labels = append(labels, fmt.Sprintf("%d", row.Iteration))
			values = append(values, row.Loss)
		}
		_ = visualization.LineChart(filepath.Join(*figures, "curva_loss.svg"), "Curva de perdida durante entrenamiento", "Iteracion", "Loss", labels, values)
	} else {
		fmt.Printf("Advertencia: no se pudo leer %s: %v\n", *metricsPath, err)
	}

	fmt.Println("Analisis del dataset generado correctamente")
	fmt.Printf("Tablas: %s\n", *out)
	fmt.Printf("Graficos SVG: %s\n", *figures)
}

func readMetrics(path string) (modelMetricsJSON, error) {
	var m modelMetricsJSON
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(b, &m)
}
