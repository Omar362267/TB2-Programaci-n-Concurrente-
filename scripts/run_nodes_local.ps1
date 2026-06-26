# Ejecutar en cuatro terminales distintas desde la raiz del proyecto.
# Cada proceso representa un nodo ML TCP independiente.
go run ./cmd/ml-node -node-id ml-node-1 -port 9101 -shard data/distributed/node-01-train.csv -workers 4
