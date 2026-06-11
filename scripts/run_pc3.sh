#!/usr/bin/env bash
set -euo pipefail

go run ./cmd/pc3 \
  -input data/raw/household_power_consumption.txt \
  -workers 8 \
  -out results \
  -processed-out data/processed \
  -iterations 300 \
  -lr 1.0
