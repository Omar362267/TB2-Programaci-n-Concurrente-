#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/shard-data \
  -input data/raw/household_power_consumption.txt \
  -model results/final_run_phase1/modelo_entrenado.json \
  -workers 8 \
  -nodes 4 \
  -out data/distributed
