// cluster-smoke verifica conectividad TCP y calcula gradientes parciales en nodos ML.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
)

func main() {
	nodesFlag := flag.String("nodes", "127.0.0.1:9101,127.0.0.1:9102,127.0.0.1:9103,127.0.0.1:9104", "direcciones TCP separadas por coma")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout total por solicitud")
	flag.Parse()
	nodes := splitNodes(*nodesFlag)
	if len(nodes) == 0 {
		log.Fatal("se requiere al menos un nodo")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	health := parallelRequests(ctx, nodes, func(i int) distributed.Request {
		return distributed.Request{Type: distributed.MessageHealth, RequestID: fmt.Sprintf("health-%02d", i+1)}
	})
	fmt.Println("Health check del cluster")
	featureCount := -1
	for _, item := range health {
		if item.Err != nil {
			log.Fatalf("%s: %v", item.Address, item.Err)
		}
		fmt.Printf("- %s -> %s | %d muestras | %d features\n", item.Response.NodeID, item.Response.Status, item.Response.Samples, item.Response.FeatureCount)
		if featureCount == -1 {
			featureCount = item.Response.FeatureCount
		} else if featureCount != item.Response.FeatureCount {
			log.Fatal("los nodos no tienen el mismo número de features")
		}
	}

	weights := make([]float64, featureCount)
	gradients := parallelRequests(ctx, nodes, func(i int) distributed.Request {
		return distributed.Request{Type: distributed.MessageGradient, RequestID: fmt.Sprintf("gradient-%02d", i+1), Iteration: 1, Weights: weights, Bias: 0}
	})
	total, lossSum, biasSum := 0, 0.0, 0.0
	gradientSum := make([]float64, featureCount)
	fmt.Println("\nGradientes parciales del cluster")
	for _, item := range gradients {
		if item.Err != nil {
			log.Fatalf("%s: %v", item.Address, item.Err)
		}
		r := item.Response
		if len(r.Gradient) != featureCount {
			log.Fatalf("%s devolvió gradiente incompatible", r.NodeID)
		}
		fmt.Printf("- %s -> %d muestras | loss_sum=%.6f | %d ms\n", r.NodeID, r.Samples, r.LossSum, r.ProcessingMS)
		total += r.Samples
		lossSum += r.LossSum
		biasSum += r.GradientBias
		for j := range gradientSum {
			gradientSum[j] += r.Gradient[j]
		}
	}
	fmt.Printf("\nReducción global\nMuestras procesadas: %d\nLoss promedio global: %.6f\nGradiente bias promedio: %.6f\n", total, lossSum/float64(total), biasSum/float64(total))
	fmt.Println("Resultado: los nodos TCP calcularon gradientes parciales y el coordinador de prueba los redujo sin actualizar pesos globales.")
}

type result struct {
	Address  string
	Response distributed.Response
	Err      error
}

func parallelRequests(ctx context.Context, nodes []string, build func(int) distributed.Request) []result {
	out := make([]result, len(nodes))
	var wg sync.WaitGroup
	for i, address := range nodes {
		wg.Add(1)
		go func(i int, address string) {
			defer wg.Done()
			r, err := distributed.SendRequest(ctx, address, build(i))
			out[i] = result{Address: address, Response: r, Err: err}
		}(i, address)
	}
	wg.Wait()
	return out
}
func splitNodes(raw string) []string {
	var out []string
	for _, n := range strings.Split(raw, ",") {
		if n = strings.TrimSpace(n); n != "" {
			out = append(out, n)
		}
	}
	return out
}
