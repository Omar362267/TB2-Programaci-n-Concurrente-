package loader

import (
    "bufio"
    "encoding/json"
    "errors"
    "os"
    "path/filepath"
    "strings"
    "sync"
)

type Config struct {
    InputPath string
    Workers   int
    Limit     int
    OutputDir string
}

type Summary struct {
    TotalRows   int `json:"total_rows"`
    ValidRows   int `json:"valid_rows"`
    InvalidRows int `json:"invalid_rows"`
    Workers     int `json:"workers"`
}

type job struct {
    lineNumber int
    raw        string
}

type partial struct {
    total   int
    valid   int
    invalid int
}

func Run(cfg Config) (Summary, error) {
    if cfg.Workers <= 0 {
        cfg.Workers = 1
    }
    if cfg.OutputDir == "" {
        cfg.OutputDir = "results"
    }

    file, err := os.Open(cfg.InputPath)
    if err != nil {
        return Summary{}, err
    }
    defer file.Close()

    jobs := make(chan job, cfg.Workers*2)
    partials := make(chan partial, cfg.Workers)

    var wg sync.WaitGroup
    for i := 0; i < cfg.Workers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            p := partial{}
            for j := range jobs {
                p.total++
                if isValidRawRecord(j.raw) {
                    p.valid++
                } else {
                    p.invalid++
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

        // Saltar cabecera
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
        return Summary{}, err
    }

    wg.Wait()
    close(partials)

    summary := Summary{Workers: cfg.Workers}
    for p := range partials {
        summary.TotalRows += p.total
        summary.ValidRows += p.valid
        summary.InvalidRows += p.invalid
    }

    if summary.TotalRows == 0 {
        return summary, errors.New("no se procesaron registros; verificar ruta o contenido del dataset")
    }

    if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
        return summary, err
    }

    outPath := filepath.Join(cfg.OutputDir, "resumen_limpieza.json")
    b, err := json.MarshalIndent(summary, "", "  ")
    if err != nil {
        return summary, err
    }
    if err := os.WriteFile(outPath, b, 0644); err != nil {
        return summary, err
    }

    return summary, nil
}

func isValidRawRecord(raw string) bool {
    if strings.TrimSpace(raw) == "" {
        return false
    }
    if strings.Contains(raw, "?") {
        return false
    }
    parts := strings.Split(raw, ";")
    return len(parts) == 9
}
