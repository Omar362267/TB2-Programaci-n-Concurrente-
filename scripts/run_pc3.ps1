$ErrorActionPreference = "Stop"

Write-Host "== PC3: ejecucion final del pipeline concurrente + ML =="
New-Item -ItemType Directory -Force -Path results/final_run | Out-Null
New-Item -ItemType Directory -Force -Path data/processed | Out-Null

go run ./cmd/pc3 `
  -input data/raw/household_power_consumption.txt `
  -workers 8 `
  -out results/final_run `
  -processed-out data/processed `
  -iterations 300 `
  -lr 1.0 2>&1 | Tee-Object -FilePath results/final_run/consola_ejecucion_final.txt

Write-Host "Ejecucion final terminada. Evidencias en results/final_run"
