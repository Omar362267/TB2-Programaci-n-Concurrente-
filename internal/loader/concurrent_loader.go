package loader

import (
	"bufio"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/preprocessing"
)

// Config define parametros de carga concurrente.
type Config struct {
	InputPath string
	Workers   int
	Limit     int
}

// Summary resume la limpieza y explica por que se descartan registros.
type Summary struct {
	TotalRows        int            `json:"total_rows"`
	ValidRows        int            `json:"valid_rows"`
	InvalidRows      int            `json:"invalid_rows"`
	Workers          int            `json:"workers"`
	DiscardReasons   map[string]int `json:"discard_reasons"`
	DurationMS       int64          `json:"duration_ms"`
	RowsPerSecond    float64        `json:"rows_per_second"`
	DiscardPolicy    string         `json:"discard_policy"`
	DatasetColumns   []string       `json:"dataset_columns"`
	ConcurrencyModel string         `json:"concurrency_model"`
}

// Result contiene el dataset limpio mas el resumen de ejecucion.
type Result struct {
	Records []preprocessing.PowerRecord
	Summary Summary
}

type job struct {
	lineNumber int
	raw        string
}

type partial struct {
	total          int
	valid          int
	invalid        int
	discardReasons map[string]int
	records        []preprocessing.PowerRecord
}

// Run lee el archivo original y procesa las filas en paralelo.
func Run(cfg Config) (Result, error) {
	start := time.Now()
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}

	file, err := os.Open(cfg.InputPath)
	if err != nil {
		return Result{}, err
	}
	defer file.Close()

	jobs := make(chan job, cfg.Workers*4)
	partials := make(chan partial, cfg.Workers)

	var wg sync.WaitGroup
	for i := 0; i < cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p := partial{
				discardReasons: make(map[string]int),
				records:        make([]preprocessing.PowerRecord, 0),
			}

			for j := range jobs {
				p.total++
				result := preprocessing.ParseRawRecord(j.lineNumber, j.raw)
				if result.Valid {
					p.valid++
					p.records = append(p.records, result.Record)
				} else {
					p.invalid++
					p.discardReasons[string(result.Reason)]++
				}
			}
			partials <- p
		}()
	}

	scanner := bufio.NewScanner(file)
	buffer := make([]byte, 1024*1024)
	scanner.Buffer(buffer, 10*1024*1024)

	lineNumber := 0
	sent := 0
	for scanner.Scan() {
		lineNumber++
		raw := scanner.Text()

		// Saltar cabecera del dataset original.
		if lineNumber == 1 && strings.Contains(raw, "Global_active_power") {
			continue
		}

		if cfg.Limit > 0 && sent >= cfg.Limit {
			break
		}

		sent++
		jobs <- job{lineNumber: lineNumber, raw: raw}
	}
	close(jobs)

	if err := scanner.Err(); err != nil {
		return Result{}, err
	}

	wg.Wait()
	close(partials)

	summary := Summary{
		Workers:        cfg.Workers,
		DiscardReasons: make(map[string]int),
		DiscardPolicy:  "Se descartan filas vacias, filas con '?' como valor faltante, filas con columnas incompletas, fechas invalidas, numeros invalidos o rangos fisicamente inconsistentes.",
		DatasetColumns: []string{
			"Date", "Time", "Global_active_power", "Global_reactive_power", "Voltage", "Global_intensity", "Sub_metering_1", "Sub_metering_2", "Sub_metering_3",
		},
		ConcurrencyModel: "Productor/consumidor: una goroutine lectora envia filas por channel; N workers parsean y validan; el coordinador reduce parciales sin estado compartido mutable.",
	}
	records := make([]preprocessing.PowerRecord, 0, sent)

	for p := range partials {
		summary.TotalRows += p.total
		summary.ValidRows += p.valid
		summary.InvalidRows += p.invalid
		for reason, count := range p.discardReasons {
			summary.DiscardReasons[reason] += count
		}
		records = append(records, p.records...)
	}

	if summary.TotalRows == 0 {
		return Result{Summary: summary}, errors.New("no se procesaron registros; verificar ruta o contenido del dataset")
	}

	// Reordenar por linea original para que el split train/test sea determinista.
	sort.Slice(records, func(i, j int) bool {
		return records[i].LineNumber < records[j].LineNumber
	})

	summary.DurationMS = time.Since(start).Milliseconds()
	seconds := time.Since(start).Seconds()
	if seconds > 0 {
		summary.RowsPerSecond = float64(summary.TotalRows) / seconds
	}

	return Result{Records: records, Summary: summary}, nil
}
