#!/usr/bin/env bash
set -euo pipefail

INPUT="${1:-data/raw/household_power_consumption.txt}"
LIMIT="${2:-0}"
ITERATIONS="${3:-100}"
LR="${4:-0.8}"

for WORKERS in 1 2 4 8; do
  echo "Ejecutando benchmark con ${WORKERS} workers..."
  go run ./cmd/pc3 \
    -input "$INPUT" \
    -workers "$WORKERS" \
    -limit "$LIMIT" \
    -out "results/benchmark_w${WORKERS}" \
    -processed-out "data/processed/benchmark_w${WORKERS}" \
    -save-processed=false \
    -iterations "$ITERATIONS" \
    -lr "$LR"
done
