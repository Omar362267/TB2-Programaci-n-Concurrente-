package storage

import (
	"context"
	"testing"
	"time"
)

func TestPredictionCacheKeyDeterministic(t *testing.T) {
	featureNames := []string{
		"hour",
		"day_of_week",
		"month",
		"voltage",
	}

	valuesA := map[string]float64{
		"hour":        20,
		"day_of_week": 6,
		"month":       12,
		"voltage":     240.8,
	}

	valuesB := map[string]float64{
		"voltage":     240.8,
		"month":       12,
		"hour":        20,
		"day_of_week": 6,
	}

	keyA := PredictionCacheKey(7, featureNames, valuesA)
	keyB := PredictionCacheKey(7, featureNames, valuesB)

	if keyA != keyB {
		t.Fatalf("misma versión y mismas features deben generar la misma clave:\nA=%s\nB=%s", keyA, keyB)
	}

	if keyA == "" {
		t.Fatal("la clave no debe quedar vacía")
	}
}

func TestPredictionCacheKeySeparatesModelVersions(t *testing.T) {
	featureNames := []string{"hour", "voltage"}
	values := map[string]float64{
		"hour":    20,
		"voltage": 240.8,
	}

	keyV1 := PredictionCacheKey(1, featureNames, values)
	keyV2 := PredictionCacheKey(2, featureNames, values)

	if keyV1 == keyV2 {
		t.Fatalf("versiones distintas del modelo no deben compartir clave Redis: %s", keyV1)
	}
}

func TestPredictionCacheKeySeparatesDifferentInputs(t *testing.T) {
	featureNames := []string{"hour", "voltage"}

	valuesA := map[string]float64{
		"hour":    20,
		"voltage": 240.8,
	}
	valuesB := map[string]float64{
		"hour":    21,
		"voltage": 240.8,
	}

	keyA := PredictionCacheKey(3, featureNames, valuesA)
	keyB := PredictionCacheKey(3, featureNames, valuesB)

	if keyA == keyB {
		t.Fatalf("entradas distintas no deben compartir clave Redis: %s", keyA)
	}
}

func TestPredictionCacheKeyUsesSortedNamesWhenFeatureOrderIsEmpty(t *testing.T) {
	valuesA := map[string]float64{
		"voltage": 240.8,
		"hour":    20,
		"month":   12,
	}
	valuesB := map[string]float64{
		"month":   12,
		"hour":    20,
		"voltage": 240.8,
	}

	keyA := PredictionCacheKey(4, nil, valuesA)
	keyB := PredictionCacheKey(4, nil, valuesB)

	if keyA != keyB {
		t.Fatalf("sin feature order explícito, las claves deben ser deterministas:\nA=%s\nB=%s", keyA, keyB)
	}
}

func TestNewRepositoryWithoutBackendsUsesDefaults(t *testing.T) {
	repo, err := NewRepository(context.Background(), Config{})
	if err != nil {
		t.Fatalf("NewRepository sin backends no debe fallar: %v", err)
	}
	defer func() {
		if closeErr := repo.Close(context.Background()); closeErr != nil {
			t.Fatalf("cerrando repositorio: %v", closeErr)
		}
	}()

	if repo.Enabled() {
		t.Fatal("un repositorio sin Mongo ni Redis no debe reportarse como enabled")
	}
	if repo.MongoEnabled() {
		t.Fatal("Mongo no debe estar habilitado")
	}
	if repo.RedisEnabled() {
		t.Fatal("Redis no debe estar habilitado")
	}

	health := repo.Health(context.Background())

	if health.MongoStatus != "disabled" {
		t.Fatalf("mongo_status esperado=disabled, recibido=%q", health.MongoStatus)
	}
	if health.RedisStatus != "disabled" {
		t.Fatalf("redis_status esperado=disabled, recibido=%q", health.RedisStatus)
	}
	if health.CacheTTLSecond != int64((5 * time.Minute).Seconds()) {
		t.Fatalf("TTL esperado=%d, recibido=%d", int64((5 * time.Minute).Seconds()), health.CacheTTLSecond)
	}
}
