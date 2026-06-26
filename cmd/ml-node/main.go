// ml-node inicia un nodo ML distribuido. Mantiene el cálculo concurrente local
// de PC3 y expone un protocolo TCP JSON para el coordinador de PC4.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
)

func main() {
	nodeID := flag.String("node-id", "ml-node-1", "identificador único del nodo")
	port := flag.Int("port", 9101, "puerto TCP local")
	host := flag.String("host", "127.0.0.1", "host/interface de escucha")
	shard := flag.String("shard", "data/distributed/node-01-train.csv", "CSV de entrenamiento asignado al nodo")
	workers := flag.Int("workers", runtime.NumCPU(), "goroutines locales para cálculo de gradiente")
	flag.Parse()
	if *port <= 0 || *port > 65535 {
		log.Fatal("-port debe estar entre 1 y 65535")
	}

	server, err := distributed.NewNodeServer(distributed.NodeConfig{NodeID: *nodeID, Address: fmt.Sprintf("%s:%d", *host, *port), ShardPath: *shard, Workers: *workers})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Nodo ML iniciado\nID: %s\nDirección TCP: %s\nShard cargado: %d muestras\nWorkers locales: %d\nEstado: ready\n", server.NodeID(), server.Addr(), server.Samples(), *workers)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := server.Serve(ctx); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Nodo %s detenido correctamente\n", server.NodeID())
}
