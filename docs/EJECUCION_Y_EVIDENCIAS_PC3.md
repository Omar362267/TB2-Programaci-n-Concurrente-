# Guia de ejecucion y evidencias PC3

Este proyecto mantiene todas las etapas oficiales en Go: carga concurrente, limpieza, feature engineering, entrenamiento ML, evaluacion, analisis exploratorio, graficos SVG y benchmark.

## 1. Ejecucion final del pipeline

PowerShell:

```powershell
.\scripts\run_pc3.ps1
```

Genera:

- `results/final_run/consola_ejecucion_final.txt`
- `results/final_run/resumen_limpieza.json`
- `results/final_run/metricas_modelo.json`
- `results/final_run/modelo_entrenado.json`
- `results/final_run/predicciones_muestra.csv`
- `results/final_run/parametros_ejecucion.json`
- `data/processed/features_high_demand.csv`

## 2. Analisis exploratorio y graficos

PowerShell:

```powershell
.\scripts\analysis_pc3.ps1
```

Genera tablas en `results/analysis/` y graficos SVG en `results/figures/`:

- distribucion de consumo activo global
- consumo promedio por hora
- consumo promedio por dia de semana
- consumo promedio por mes
- matriz de correlacion
- matriz de confusion
- curva de perdida

## 3. Benchmark comparativo

PowerShell:

```powershell
.\scripts\benchmark_pc3.ps1
```

Genera:

- resultados por workers 1, 2, 4 y 8
- `benchmark_comparativo.json`
- `benchmark_comparativo.csv`
- `benchmark_resumen.md`
- `results/figures/benchmark_workers.svg`

## 4. Generacion completa

```powershell
.\scripts\generate_evidence.ps1
```

Ejecuta las tres fases anteriores.

## 5. Comandos Go directos

```powershell
go test ./...

go run ./cmd/pc3 -input data/raw/household_power_consumption.txt -workers 8 -out results/final_run -processed-out data/processed -iterations 300 -lr 1.0

go run ./cmd/pc3-analysis -input data/raw/household_power_consumption.txt -workers 8 -out results/analysis -figures results/figures -metrics results/final_run/metricas_modelo.json

go run ./cmd/pc3-benchmark -input data/raw/household_power_consumption.txt -out results/benchmarks -figures results/figures -workers 1,2,4,8 -iterations 300 -lr 1.0 -run=true
```
