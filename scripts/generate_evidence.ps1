$ErrorActionPreference = "Stop"

Write-Host "== PC3: generacion completa de evidencias =="
Write-Host "1/3 Ejecucion final"
& "$PSScriptRoot/run_pc3.ps1"

Write-Host "2/3 Analisis exploratorio y graficos"
& "$PSScriptRoot/analysis_pc3.ps1"

Write-Host "3/3 Benchmark comparativo"
& "$PSScriptRoot/benchmark_pc3.ps1"

Write-Host "Proceso completo finalizado. Revisar carpetas results/final_run, results/analysis, results/figures y results/benchmarks."
