$ErrorActionPreference = "Stop"
go run ./cmd/api `
  -host 127.0.0.1 `
  -port 8080 `
  -nodes "ml-node-1=127.0.0.1:9101,ml-node-2=127.0.0.1:9102,ml-node-3=127.0.0.1:9103,ml-node-4=127.0.0.1:9104"
