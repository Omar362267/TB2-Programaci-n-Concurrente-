$ErrorActionPreference = "Stop"

Write-Host "== PC3: benchmark comparativo de concurrencia =="
Write-Host "Nota: esta prueba ejecuta el pipeline varias veces; puede tardar varios minutos."
New-Item -ItemType Directory -Force -Path results/benchmarks | Out-Null
New-Item -ItemType Directory -Force -Path results/figures | Out-Null

go run ./cmd/pc3-benchmark `
  -input data/raw/household_power_consumption.txt `
  -out results/benchmarks `
  -figures results/figures `
  -workers 1,2,4,8 `
  -iterations 300 `
  -lr 1.0 `
  -run=true

Write-Host "Benchmark terminado. Evidencias en results/benchmarks"
