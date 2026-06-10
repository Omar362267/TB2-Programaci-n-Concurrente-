package main

import (
    "flag"
    "fmt"
    "log"
    "runtime"

    "github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/loader"
)

func main() {
    input := flag.String("input", "data/raw/household_power_consumption.txt", "ruta del dataset original")
    workers := flag.Int("workers", runtime.NumCPU(), "numero de workers concurrentes")
    limit := flag.Int("limit", 0, "limite opcional de registros para pruebas")
    out := flag.String("out", "results", "carpeta de salida")
    flag.Parse()

    cfg := loader.Config{
        InputPath: *input,
        Workers:   *workers,
        Limit:     *limit,
        OutputDir: *out,
    }

    summary, err := loader.Run(cfg)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("Ejecucion PC3 completada")
    fmt.Printf("Registros leidos: %d
", summary.TotalRows)
    fmt.Printf("Registros validos: %d
", summary.ValidRows)
    fmt.Printf("Registros descartados: %d
", summary.InvalidRows)
}
