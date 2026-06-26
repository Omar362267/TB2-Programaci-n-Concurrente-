# Requires four local ml-node processes already running on ports 9101..9104.
go run ./cmd/api `
  -host 127.0.0.1 `
  -port 8080 `
  -model results/final_run_phase1/modelo_entrenado.json `
  -trained-model results/distributed/modelo_entrenado_distribuido.json `
  -nodes "ml-node-1=127.0.0.1:9101,ml-node-2=127.0.0.1:9102,ml-node-3=127.0.0.1:9103,ml-node-4=127.0.0.1:9104" `
  -mongo-uri "mongodb://127.0.0.1:27017" `
  -mongo-db "pc4_energy" `
  -redis-addr "127.0.0.1:6379" `
  -cache-ttl 5m
