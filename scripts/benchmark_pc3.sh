#!/usr/bin/env bash
set -euo pipefail

mkdir -p results/benchmarks results/figures

go run ./cmd/pc3-benchmark \
  -input data/raw/household_power_consumption.txt \
  -out results/benchmarks \
  -figures results/figures \
  -workers 1,2,4,8 \
  -iterations 300 \
  -lr 1.0 \
  -run=true
