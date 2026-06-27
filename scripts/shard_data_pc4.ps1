param(
  [int]$Nodes = 4,
  [int]$Workers = 8,
  [string]$Input = "data/raw/household_power_consumption.txt",
  [string]$Model = "results/final_run_phase1/modelo_entrenado.json",
  [string]$Out = "data/distributed"
)

go run ./cmd/shard-data `
  -input $Input `
  -model $Model `
  -workers $Workers `
  -nodes $Nodes `
  -out $Out
