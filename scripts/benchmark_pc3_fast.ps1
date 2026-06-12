$ErrorActionPreference = "Stop"

Write-Host "== PC3: benchmark rapido para validacion tecnica =="
Write-Host "Usa menos iteraciones para probar que el flujo funciona. Para el informe final usar benchmark_pc3.ps1."

go run ./cmd/pc3-benchmark `
  -input data/raw/household_power_consumption.txt `
  -out results/benchmarks_fast `
  -figures results/figures `
  -workers 1,2,4,8 `
  -iterations 30 `
  -lr 1.0 `
  -run=true
