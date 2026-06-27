set -euo pipefail

mkdir -p results/analysis results/figures

go run ./cmd/pc3-analysis \
  -input data/raw/household_power_consumption.txt \
  -workers 8 \
  -out results/analysis \
  -figures results/figures \
  -metrics results/final_run/metricas_modelo.json
