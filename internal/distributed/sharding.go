// Package distributed contiene utilidades preparatorias para ejecutar el
// entrenamiento de forma distribuida. En Fase 2 prepara shards deterministas;
// las fases posteriores incorporarán nodos TCP y un coordinador.
package distributed

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/features"
)

// SplitConfig fija los parámetros que deben repetirse exactamente entre el
// entrenamiento local y la creación de shards distribuidos.
type SplitConfig struct {
	TestRatio float64 `json:"test_ratio"`
	Seed      int64   `json:"seed"`
}

// ShardInfo describe un archivo asignado a un nodo ML.
type ShardInfo struct {
	NodeID          string `json:"node_id"`
	File            string `json:"file"`
	Samples         int    `json:"samples"`
	PositiveSamples int    `json:"positive_samples"`
	NegativeSamples int    `json:"negative_samples"`
	SHA256          string `json:"sha256"`
}

// Manifest documenta la partición distribuida y permite verificar que ningún
// registro de entrenamiento se perdió o fue duplicado.
type Manifest struct {
	SchemaVersion       string              `json:"schema_version"`
	GeneratedAt         string              `json:"generated_at"`
	SourceInput         string              `json:"source_input"`
	SourceModelArtifact string              `json:"source_model_artifact"`
	ValidRecords        int                 `json:"valid_records"`
	TrainSamples        int                 `json:"train_samples"`
	TestSamples         int                 `json:"test_samples"`
	ShardCount          int                 `json:"shard_count"`
	Split               SplitConfig         `json:"split"`
	FeatureNames        []string            `json:"feature_names"`
	Normalizer          features.Normalizer `json:"normalizer"`
	TargetDefinition    string              `json:"target_definition"`
	HighDemandThreshold float64             `json:"high_demand_threshold_p75"`
	TestFile            string              `json:"test_file"`
	TestSHA256          string              `json:"test_sha256"`
	Shards              []ShardInfo         `json:"shards"`
	Validation          ValidationSummary   `json:"validation"`
}

// ValidationSummary resume las comprobaciones automáticas de la partición.
type ValidationSummary struct {
	TrainShardSamples      int  `json:"train_shard_samples"`
	TrainCoverageMatches   bool `json:"train_coverage_matches"`
	DifferenceMaxMinShard  int  `json:"difference_max_min_shard"`
	NormalizerTrainingOnly bool `json:"normalizer_training_set_only"`
}

// SplitTrainTest mezcla determinísticamente antes de separar train/test. La
// misma semilla hace que PC3 y el preparador de shards produzcan el mismo split.
func SplitTrainTest(samples []features.Sample, cfg SplitConfig) ([]features.Sample, []features.Sample, error) {
	if len(samples) < 2 {
		return nil, nil, fmt.Errorf("se requieren al menos 2 muestras para split; se recibieron %d", len(samples))
	}
	if cfg.TestRatio <= 0 || cfg.TestRatio >= 0.5 {
		return nil, nil, fmt.Errorf("test_ratio debe estar entre 0 y 0.5 (exclusivo); se recibio %v", cfg.TestRatio)
	}

	shuffled := make([]features.Sample, len(samples))
	copy(shuffled, samples)
	rng := rand.New(rand.NewSource(cfg.Seed))
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	testSize := int(float64(len(shuffled)) * cfg.TestRatio)
	if testSize < 1 {
		testSize = 1
	}
	trainSize := len(shuffled) - testSize
	if trainSize < 1 {
		trainSize = len(shuffled) - 1
	}
	return shuffled[:trainSize], shuffled[trainSize:], nil
}

// PartitionRoundRobin asigna las muestras de train de forma balanceada a los
// nodos. Como train ya está mezclado, cada shard conserva una distribución
// comparable y la diferencia de tamaño entre shards es como máximo 1.
func PartitionRoundRobin(samples []features.Sample, shardCount int) ([][]features.Sample, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("no se pueden particionar cero muestras")
	}
	if shardCount <= 0 {
		return nil, fmt.Errorf("shard_count debe ser mayor que cero")
	}
	if shardCount > len(samples) {
		return nil, fmt.Errorf("shard_count (%d) no puede superar las muestras de train (%d)", shardCount, len(samples))
	}
	shards := make([][]features.Sample, shardCount)
	for i, s := range samples {
		idx := i % shardCount
		shards[idx] = append(shards[idx], s)
	}
	return shards, nil
}

// WriteSamplesCSV persiste muestras ya normalizadas en un formato que los
// futuros nodos ML podrán cargar sin recalcular el normalizador.
func WriteSamplesCSV(path string, samples []features.Sample, featureNames []string) error {
	if len(featureNames) == 0 {
		return fmt.Errorf("feature_names vacio")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	header := append([]string(nil), featureNames...)
	header = append(header, "high_demand")
	if err := writer.Write(header); err != nil {
		return err
	}
	for i, sample := range samples {
		if len(sample.X) != len(featureNames) {
			return fmt.Errorf("muestra %d: %d features; se esperaban %d", i, len(sample.X), len(featureNames))
		}
		row := make([]string, 0, len(sample.X)+1)
		for _, value := range sample.X {
			row = append(row, strconv.FormatFloat(value, 'f', 8, 64))
		}
		row = append(row, strconv.Itoa(sample.Y))
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	if err := writer.Error(); err != nil {
		return err
	}
	return nil
}

// WriteManifest guarda la evidencia legible y versionada de la distribución.
func WriteManifest(path string, manifest Manifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// FileSHA256 permite probar la integridad de cada shard antes de montarlo en un
// contenedor/nodo distribuido.
func FileSHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// ClassCounts devuelve conteos por clase para documentar balance de shards.
func ClassCounts(samples []features.Sample) (positive, negative int) {
	for _, sample := range samples {
		if sample.Y == 1 {
			positive++
		} else {
			negative++
		}
	}
	return positive, negative
}

// MaxMinShardDifference mide el desequilibrio de tamaño; en round-robin debe
// ser 0 o 1, salvo que se usen datos inválidos.
func MaxMinShardDifference(shards [][]features.Sample) int {
	if len(shards) == 0 {
		return 0
	}
	min, max := len(shards[0]), len(shards[0])
	for _, shard := range shards[1:] {
		if len(shard) < min {
			min = len(shard)
		}
		if len(shard) > max {
			max = len(shard)
		}
	}
	return max - min
}
