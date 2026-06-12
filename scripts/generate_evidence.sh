#!/usr/bin/env bash
set -euo pipefail

bash scripts/run_pc3.sh
bash scripts/analysis_pc3.sh
bash scripts/benchmark_pc3.sh
