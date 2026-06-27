set -euo pipefail

mkdir -p results/final_run data/processed

go run ./cmd/pc3 \
  -input data/raw/household_power_consumption.txt \
  -workers 8 \
  -out results/final_run \
  -processed-out data/processed \
  -iterations 300 \
  -lr 1.0 | tee results/final_run/consola_ejecucion_final.txt
