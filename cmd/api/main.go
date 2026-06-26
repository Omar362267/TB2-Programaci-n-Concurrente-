// api starts the HTTP coordinator for the PC4 distributed ML cluster.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/storage"

	clusterapi "github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/api"
)

func main() {
	host := flag.String("host", "127.0.0.1", "host/interface HTTP de escucha")
	port := flag.Int("port", 8080, "puerto HTTP de la API")
	nodesRaw := flag.String("nodes", "ml-node-1=127.0.0.1:9101,ml-node-2=127.0.0.1:9102,ml-node-3=127.0.0.1:9103,ml-node-4=127.0.0.1:9104", "nodos como id=host:puerto separados por coma")
	nodeTimeout := flag.Duration("node-timeout", 3*time.Second, "timeout TCP por nodo")
	modelPath := flag.String("model", "results/final_run_phase1/modelo_entrenado.json", "artefacto de Fase 1 con normalizador y metadata")
	trainedModelPath := flag.String("trained-model", "results/distributed/modelo_entrenado_distribuido.json", "salida del modelo actualizado por entrenamiento distribuido")
	testDataPath := flag.String("test-data", "data/distributed/test.csv", "shard de prueba hold-out normalizado de Fase 2")
	mongoURI := flag.String("mongo-uri", "", "URI MongoDB opcional, por ejemplo mongodb://127.0.0.1:27017")
	mongoDatabase := flag.String("mongo-db", "pc4_energy", "base de datos MongoDB")
	redisAddr := flag.String("redis-addr", "", "dirección Redis opcional, por ejemplo 127.0.0.1:6379")
	redisPassword := flag.String("redis-password", "", "contraseña Redis opcional")
	redisDB := flag.Int("redis-db", 0, "índice de base Redis")
	storageTimeout := flag.Duration("storage-timeout", 3*time.Second, "timeout para MongoDB y Redis")
	cacheTTL := flag.Duration("cache-ttl", 5*time.Minute, "vida útil de respuestas cacheadas en Redis")
	flag.Parse()
	if *port <= 0 || *port > 65535 {
		log.Fatal("-port debe estar entre 1 y 65535")
	}
	nodes, err := parseNodes(*nodesRaw)
	if err != nil {
		log.Fatal(err)
	}
	service, err := clusterapi.NewService(nodes, *nodeTimeout)
	if err != nil {
		log.Fatal(err)
	}
	artifact, err := ml.LoadArtifact(*modelPath)
	if err != nil {
		log.Fatalf("cargando -model: %v", err)
	}
	if err := service.ConfigureModel(artifact, filepath.Clean(*trainedModelPath)); err != nil {
		log.Fatal(err)
	}
	if err := service.ConfigureEvaluation(filepath.Clean(*testDataPath)); err != nil {
		log.Fatalf("configurando evaluación: %v", err)
	}
	repository, err := storage.NewRepository(context.Background(), storage.Config{
		MongoURI: *mongoURI, MongoDatabase: *mongoDatabase, RedisAddr: *redisAddr, RedisPassword: *redisPassword, RedisDB: *redisDB, Timeout: *storageTimeout, CacheTTL: *cacheTTL,
	})
	if err != nil {
		log.Fatalf("configurando almacenamiento: %v", err)
	}
	service.ConfigureStorage(repository)
	defer repository.Close(context.Background())
	server := clusterapi.NewServer(service)
	address := fmt.Sprintf("%s:%d", *host, *port)
	httpServer := &http.Server{Addr: address, Handler: server.Routes(), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		fmt.Printf("API coordinadora iniciada\nDirección HTTP: http://%s\nNodos configurados: %d\nEndpoints: GET /health | GET /v1/cluster/status | GET /v1/storage/status | POST /v1/train | POST /v1/predict | POST /v1/evaluate | GET /v1/model | GET /v1/metrics\n", address, len(nodes))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("API: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	fmt.Println("API detenida correctamente")
}

func parseNodes(raw string) ([]clusterapi.NodeEndpoint, error) {
	var nodes []clusterapi.NodeEndpoint
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
			return nil, fmt.Errorf("nodo invalido %q; use id=host:puerto", entry)
		}
		nodes = append(nodes, clusterapi.NodeEndpoint{ID: strings.TrimSpace(parts[0]), Address: strings.TrimSpace(parts[1])})
	}
	return nodes, nil
}
