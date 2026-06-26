// shard-data prepara el conjunto de entrenamiento para los nodos ML de PC4.
// Conserva la separación train/test y usa el mismo normalizador persistido en
// modelo_entrenado.json. No reemplaza el pipeline concurrente de PC3.
package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/distributed"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/loader"
	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"
)

func main() {
	input := flag.String("input", "data/raw/household_power_consumption.txt", "ruta del dataset original")
	modelPath := flag.String("model", "results/final_run_phase1/modelo_entrenado.json", "artefacto entrenado de Fase 1")
	out := flag.String("out", "data/distributed", "directorio de shards distribuidos")
	nodes := flag.Int("nodes", 4, "cantidad de nodos ML/shards")
	workers := flag.Int("workers", runtime.NumCPU(), "workers para carga y limpieza concurrente")
	limit := flag.Int("limit", 0, "limite opcional para smoke test; 0 procesa todo")
	testRatio := flag.Float64("test-ratio", 0.2, "proporcion de prueba; debe coincidir con Fase 1")
	seed := flag.Int64("seed", 42, "semilla de split; debe coincidir con Fase 1")
	overwrite := flag.Bool("overwrite", false, "permite sobrescribir archivos existentes")
	flag.Parse()

	if *nodes <= 0 {
		log.Fatal("-nodes debe ser mayor que cero")
	}
	if *workers <= 0 {
		*workers = 1
	}
	if *testRatio <= 0 || *testRatio >= 0.5 {
		log.Fatal("-test-ratio debe estar entre 0 y 0.5")
	}

	if err := ensureOutputDir(*out, *overwrite); err != nil {
		log.Fatal(err)
	}

	artifact, err := ml.LoadArtifact(*modelPath)
	if err != nil {
		log.Fatalf("no se pudo cargar artefacto de Fase 1: %v", err)
	}
	if artifact.Normalizer.FittedOn != "training_set_only" {
		log.Fatalf("artefacto invalido para distribución: normalizer.fitted_on=%q; se requiere training_set_only", artifact.Normalizer.FittedOn)
	}

	loaded, err := loader.Run(loader.Config{InputPath: *input, Workers: *workers, Limit: *limit})
	if err != nil {
		log.Fatal(err)
	}
	rawSamples, summary := features.BuildSamples(loaded.Records)
	if len(rawSamples) < *nodes+1 {
		log.Fatalf("muestras insuficientes (%d) para %d nodos", len(rawSamples), *nodes)
	}
	if math.Abs(summary.HighDemandThreshold-artifact.ThresholdP75) > 1e-9 {
		log.Fatalf("el umbral del dataset (%.6f) no coincide con el artefacto (%.6f). Use el mismo input y limit de Fase 1", summary.HighDemandThreshold, artifact.ThresholdP75)
	}

	splitCfg := distributed.SplitConfig{TestRatio: *testRatio, Seed: *seed}
	rawTrain, rawTest, err := distributed.SplitTrainTest(rawSamples, splitCfg)
	if err != nil {
		log.Fatal(err)
	}

	// Se usa el normalizador que quedó guardado con el modelo, no se vuelve a
	// ajustar sobre train/test: así los nodos y la API usan exactamente el mismo contrato.
	train, err := artifact.Normalizer.TransformSamples(rawTrain)
	if err != nil {
		log.Fatal(err)
	}
	test, err := artifact.Normalizer.TransformSamples(rawTest)
	if err != nil {
		log.Fatal(err)
	}
	shards, err := distributed.PartitionRoundRobin(train, *nodes)
	if err != nil {
		log.Fatal(err)
	}

	manifest := distributed.Manifest{
		SchemaVersion:       "pc4-phase2-v1",
		GeneratedAt:         time.Now().Format(time.RFC3339),
		SourceInput:         *input,
		SourceModelArtifact: *modelPath,
		ValidRecords:        len(rawSamples),
		TrainSamples:        len(train),
		TestSamples:         len(test),
		ShardCount:          *nodes,
		Split:               splitCfg,
		FeatureNames:        append([]string(nil), artifact.FeatureNames...),
		Normalizer:          artifact.Normalizer,
		TargetDefinition:    artifact.Target,
		HighDemandThreshold: artifact.ThresholdP75,
		TestFile:            "test.csv",
		Validation: distributed.ValidationSummary{
			TrainCoverageMatches:   true,
			TrainShardSamples:      len(train),
			DifferenceMaxMinShard:  distributed.MaxMinShardDifference(shards),
			NormalizerTrainingOnly: artifact.Normalizer.FittedOn == "training_set_only",
		},
	}

	for i, shard := range shards {
		name := fmt.Sprintf("node-%02d-train.csv", i+1)
		path := filepath.Join(*out, name)
		if err := distributed.WriteSamplesCSV(path, shard, artifact.FeatureNames); err != nil {
			log.Fatal(err)
		}
		hash, err := distributed.FileSHA256(path)
		if err != nil {
			log.Fatal(err)
		}
		pos, neg := distributed.ClassCounts(shard)
		manifest.Shards = append(manifest.Shards, distributed.ShardInfo{
			NodeID: fmt.Sprintf("ml-node-%d", i+1), File: name, Samples: len(shard),
			PositiveSamples: pos, NegativeSamples: neg, SHA256: hash,
		})
	}

	testPath := filepath.Join(*out, manifest.TestFile)
	if err := distributed.WriteSamplesCSV(testPath, test, artifact.FeatureNames); err != nil {
		log.Fatal(err)
	}
	testHash, err := distributed.FileSHA256(testPath)
	if err != nil {
		log.Fatal(err)
	}
	manifest.TestSHA256 = testHash
	if err := distributed.WriteManifest(filepath.Join(*out, "manifest.json"), manifest); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Particion distribuida completada")
	fmt.Printf("Registros validos reutilizados: %d\n", manifest.ValidRecords)
	fmt.Printf("Muestras train/test: %d/%d\n", manifest.TrainSamples, manifest.TestSamples)
	fmt.Printf("Nodos ML preparados: %d\n", manifest.ShardCount)
	for _, s := range manifest.Shards {
		fmt.Printf("- %s: %d muestras (%d alta demanda, %d normal) -> %s\n", s.NodeID, s.Samples, s.PositiveSamples, s.NegativeSamples, s.File)
	}
	fmt.Printf("Test global: %d muestras -> %s\n", manifest.TestSamples, manifest.TestFile)
	fmt.Printf("Manifest: %s\n", filepath.Join(*out, "manifest.json"))
}

func ensureOutputDir(out string, overwrite bool) error {
	if info, err := os.Stat(out); err == nil && info.IsDir() && !overwrite {
		entries, readErr := os.ReadDir(out)
		if readErr != nil {
			return readErr
		}
		if len(entries) > 0 {
			return fmt.Errorf("la carpeta %s ya contiene archivos; use -overwrite=true o una carpeta nueva", out)
		}
	}
	return os.MkdirAll(out, 0755)
}
