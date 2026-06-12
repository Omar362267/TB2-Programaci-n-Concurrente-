package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/visualization"
)

type benchFile struct {
	Performance struct {
		Workers           int     `json:"workers"`
		RowsProcessed     int     `json:"rows_processed"`
		LoadDurationMS    int64   `json:"load_duration_ms"`
		FeatureDurationMS int64   `json:"feature_duration_ms"`
		TrainDurationMS   int64   `json:"train_duration_ms"`
		TotalDurationMS   int64   `json:"total_duration_ms"`
		RowsPerSecond     float64 `json:"rows_per_second"`
	} `json:"performance"`
}

type row struct {
	Workers           int     `json:"workers"`
	RowsProcessed     int     `json:"rows_processed"`
	LoadDurationMS    int64   `json:"load_duration_ms"`
	FeatureDurationMS int64   `json:"feature_duration_ms"`
	TrainDurationMS   int64   `json:"train_duration_ms"`
	TotalDurationMS   int64   `json:"total_duration_ms"`
	RowsPerSecond     float64 `json:"rows_per_second"`
	Speedup           float64 `json:"speedup"`
	Efficiency        float64 `json:"efficiency"`
}

type benchmarkSummary struct {
	Rows  []row    `json:"rows"`
	Notes []string `json:"notes"`
}

func main() {
	input := flag.String("input", "data/raw/household_power_consumption.txt", "ruta del dataset original")
	out := flag.String("out", "results/benchmarks", "carpeta de salida")
	figures := flag.String("figures", "results/figures", "carpeta para graficos SVG")
	workersList := flag.String("workers", "1,2,4,8", "lista de workers separados por coma")
	iterations := flag.Int("iterations", 300, "iteraciones de entrenamiento")
	learningRate := flag.Float64("lr", 1.0, "learning rate")
	limit := flag.Int("limit", 0, "limite opcional; 0 procesa todo")
	run := flag.Bool("run", true, "si es true ejecuta pc3 para cada cantidad de workers; si es false consolida resultados existentes")
	flag.Parse()

	workers, err := parseWorkers(*workersList)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*figures, 0755); err != nil {
		log.Fatal(err)
	}

	if *run {
		for _, w := range workers {
			dir := filepath.Join(*out, fmt.Sprintf("benchmark_w%d", w))
			if err := os.MkdirAll(dir, 0755); err != nil {
				log.Fatal(err)
			}
			args := []string{"run", "./cmd/pc3", "-input", *input, "-workers", strconv.Itoa(w), "-out", dir, "-processed-out", "data/processed", "-save-processed=false", "-iterations", strconv.Itoa(*iterations), "-lr", fmt.Sprintf("%.8g", *learningRate)}
			if *limit > 0 {
				args = append(args, "-limit", strconv.Itoa(*limit))
			}
			cmd := exec.Command("go", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			fmt.Printf("\nEjecutando benchmark con %d workers...\n", w)
			if err := cmd.Run(); err != nil {
				log.Fatalf("benchmark con %d workers fallo: %v", w, err)
			}
		}
	}

	rows := make([]row, 0, len(workers))
	for _, w := range workers {
		path := filepath.Join(*out, fmt.Sprintf("benchmark_w%d", w), "benchmark_concurrencia.json")
		bf, err := readBench(path)
		if err != nil {
			log.Fatalf("no se pudo leer %s: %v", path, err)
		}
		rows = append(rows, row{Workers: bf.Performance.Workers, RowsProcessed: bf.Performance.RowsProcessed, LoadDurationMS: bf.Performance.LoadDurationMS, FeatureDurationMS: bf.Performance.FeatureDurationMS, TrainDurationMS: bf.Performance.TrainDurationMS, TotalDurationMS: bf.Performance.TotalDurationMS, RowsPerSecond: bf.Performance.RowsPerSecond})
	}
	baseline := rows[0].TotalDurationMS
	if baseline <= 0 {
		baseline = 1
	}
	baseWorkers := rows[0].Workers
	if baseWorkers <= 0 {
		baseWorkers = 1
	}
	for i := range rows {
		if rows[i].TotalDurationMS > 0 {
			rows[i].Speedup = float64(baseline) / float64(rows[i].TotalDurationMS)
		}
		rows[i].Efficiency = rows[i].Speedup / (float64(rows[i].Workers) / float64(baseWorkers))
	}
	summary := benchmarkSummary{Rows: rows, Notes: []string{"Benchmark generado en Go ejecutando el mismo pipeline con distintas cantidades de workers.", "El speedup usa como linea base la primera configuracion de la lista, normalmente 1 worker.", "Los resultados pueden variar segun CPU, disco, carga del sistema y temperatura del equipo."}}
	if err := writeJSON(filepath.Join(*out, "benchmark_comparativo.json"), summary); err != nil {
		log.Fatal(err)
	}
	if err := writeCSV(filepath.Join(*out, "benchmark_comparativo.csv"), rows); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "benchmark_resumen.md"), []byte(markdown(rows)), 0644); err != nil {
		log.Fatal(err)
	}
	labels := make([]string, 0, len(rows))
	values := make([]float64, 0, len(rows))
	for _, r := range rows {
		labels = append(labels, fmt.Sprintf("%d workers", r.Workers))
		values = append(values, r.RowsPerSecond)
	}
	_ = visualization.SimpleBars(filepath.Join(*figures, "benchmark_workers.svg"), "Rendimiento por cantidad de workers", "Workers", "Registros por segundo", labels, values)
	fmt.Println("Benchmark comparativo generado correctamente")
	fmt.Printf("Resultados: %s\n", *out)
}

func parseWorkers(s string) ([]int, error) {
	parts := strings.Split(s, ",")
	out := []int{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil || v <= 0 {
			return nil, fmt.Errorf("worker invalido: %s", p)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("lista de workers vacia")
	}
	return out, nil
}
func readBench(path string) (benchFile, error) {
	var b benchFile
	data, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	return b, json.Unmarshal(data, &b)
}
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}
func writeCSV(path string, rows []row) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"workers", "rows_processed", "load_ms", "features_ms", "train_ms", "total_ms", "rows_per_second", "speedup", "efficiency"})
	for _, r := range rows {
		w.Write([]string{itoa(r.Workers), itoa(r.RowsProcessed), i64(r.LoadDurationMS), i64(r.FeatureDurationMS), i64(r.TrainDurationMS), i64(r.TotalDurationMS), f64(r.RowsPerSecond), f64(r.Speedup), f64(r.Efficiency)})
	}
	return w.Error()
}
func markdown(rows []row) string {
	var b strings.Builder
	b.WriteString("# Benchmark comparativo de concurrencia\n\n")
	b.WriteString("| Workers | Registros | Tiempo total ms | Registros/s | Speedup | Eficiencia |\n|---:|---:|---:|---:|---:|---:|\n")
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("| %d | %d | %d | %.2f | %.2f | %.2f |\n", r.Workers, r.RowsProcessed, r.TotalDurationMS, r.RowsPerSecond, r.Speedup, r.Efficiency))
	}
	return b.String()
}
func itoa(v int) string    { return strconv.Itoa(v) }
func i64(v int64) string   { return strconv.FormatInt(v, 10) }
func f64(v float64) string { return strconv.FormatFloat(v, 'f', 6, 64) }
