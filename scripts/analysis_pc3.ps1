$ErrorActionPreference = "Stop"

Write-Host "== PC3: analisis exploratorio y graficos 100% Go =="
New-Item -ItemType Directory -Force -Path results/analysis | Out-Null
New-Item -ItemType Directory -Force -Path results/figures | Out-Null

go run ./cmd/pc3-analysis `
  -input data/raw/household_power_consumption.txt `
  -workers 8 `
  -out results/analysis `
  -figures results/figures `
  -metrics results/final_run/metricas_modelo.json

Write-Host "Analisis generado. Tablas en results/analysis y graficos en results/figures"
