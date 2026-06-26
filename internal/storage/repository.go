// Package storage provides optional MongoDB persistence and Redis caching for PC4.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Config defines connection settings. Leaving MongoURI and RedisAddr empty disables the respective backend.
type Config struct {
	MongoURI      string
	MongoDatabase string
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	Timeout       time.Duration
	CacheTTL      time.Duration
}

// Health describes the availability of the optional storage backends.
type Health struct {
	MongoEnabled   bool   `json:"mongo_enabled"`
	MongoStatus    string `json:"mongo_status"`
	MongoError     string `json:"mongo_error,omitempty"`
	RedisEnabled   bool   `json:"redis_enabled"`
	RedisStatus    string `json:"redis_status"`
	RedisError     string `json:"redis_error,omitempty"`
	CacheTTLSecond int64  `json:"cache_ttl_seconds"`
}

// Repository manages persistence and cache. It is safe for concurrent use by the mongo and redis clients.
type Repository struct {
	mongoClient *mongo.Client
	mongoDB     *mongo.Database
	redisClient *redis.Client
	timeout     time.Duration
	cacheTTL    time.Duration
}

// NewRepository connects only to the configured backends. The API can keep running without storage when both are disabled.
func NewRepository(ctx context.Context, cfg Config) (*Repository, error) {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 3 * time.Second
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = 5 * time.Minute
	}
	repo := &Repository{timeout: cfg.Timeout, cacheTTL: cfg.CacheTTL}

	if strings.TrimSpace(cfg.MongoURI) != "" {
		connectCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
		client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(cfg.MongoURI))
		if err != nil {
			return nil, fmt.Errorf("conectando MongoDB: %w", err)
		}
		if err := client.Ping(connectCtx, nil); err != nil {
			_ = client.Disconnect(context.Background())
			return nil, fmt.Errorf("verificando MongoDB: %w", err)
		}
		dbName := strings.TrimSpace(cfg.MongoDatabase)
		if dbName == "" {
			dbName = "pc4_energy"
		}
		repo.mongoClient = client
		repo.mongoDB = client.Database(dbName)
	}

	if strings.TrimSpace(cfg.RedisAddr) != "" {
		client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB})
		pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
		if err := client.Ping(pingCtx).Err(); err != nil {
			_ = client.Close()
			if repo.mongoClient != nil {
				_ = repo.mongoClient.Disconnect(context.Background())
			}
			return nil, fmt.Errorf("verificando Redis: %w", err)
		}
		repo.redisClient = client
	}
	return repo, nil
}

func (r *Repository) Enabled() bool      { return r != nil && (r.mongoDB != nil || r.redisClient != nil) }
func (r *Repository) MongoEnabled() bool { return r != nil && r.mongoDB != nil }
func (r *Repository) RedisEnabled() bool { return r != nil && r.redisClient != nil }

func (r *Repository) Close(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var first error
	if r.redisClient != nil {
		if err := r.redisClient.Close(); err != nil && first == nil {
			first = err
		}
	}
	if r.mongoClient != nil {
		if err := r.mongoClient.Disconnect(ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (r *Repository) Health(ctx context.Context) Health {
	h := Health{MongoEnabled: r != nil && r.mongoDB != nil, RedisEnabled: r != nil && r.redisClient != nil}
	if r != nil {
		h.CacheTTLSecond = int64(r.cacheTTL.Seconds())
	}
	if !h.MongoEnabled {
		h.MongoStatus = "disabled"
	} else {
		checkCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		if err := r.mongoClient.Ping(checkCtx, nil); err != nil {
			h.MongoStatus = "unavailable"
			h.MongoError = err.Error()
		} else {
			h.MongoStatus = "ready"
		}
	}
	if !h.RedisEnabled {
		h.RedisStatus = "disabled"
	} else {
		checkCtx, cancel := context.WithTimeout(ctx, r.timeout)
		defer cancel()
		if err := r.redisClient.Ping(checkCtx).Err(); err != nil {
			h.RedisStatus = "unavailable"
			h.RedisError = err.Error()
		} else {
			h.RedisStatus = "ready"
		}
	}
	return h
}

// SaveTraining persists a complete immutable training report and model snapshot in MongoDB.
func (r *Repository) SaveTraining(ctx context.Context, version int, training any, model any) error {
	if !r.MongoEnabled() {
		return nil
	}
	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	_, err := r.mongoDB.Collection("training_runs").InsertOne(opCtx, bson.M{
		"model_version": version,
		"created_at":    time.Now().UTC(),
		"training":      training,
		"model":         model,
	})
	if err != nil {
		return fmt.Errorf("guardando training_runs: %w", err)
	}
	return nil
}

// SavePrediction persists every successfully evaluated prediction in MongoDB.
func (r *Repository) SavePrediction(ctx context.Context, version int, input any, result any, cacheHit bool) error {
	if !r.MongoEnabled() {
		return nil
	}
	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	_, err := r.mongoDB.Collection("prediction_logs").InsertOne(opCtx, bson.M{
		"model_version": version,
		"created_at":    time.Now().UTC(),
		"input":         input,
		"result":        result,
		"cache_hit":     cacheHit,
	})
	if err != nil {
		return fmt.Errorf("guardando prediction_logs: %w", err)
	}
	return nil
}

// CachePrediction stores the response serialized as JSON; disabled Redis is a no-op.
func (r *Repository) CachePrediction(ctx context.Context, key string, result any) error {
	if !r.RedisEnabled() {
		return nil
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("serializando cache: %w", err)
	}
	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	if err := r.redisClient.Set(opCtx, key, payload, r.cacheTTL).Err(); err != nil {
		return fmt.Errorf("guardando cache Redis: %w", err)
	}
	return nil
}

// LoadCachedPrediction deserializes a cached JSON response. found=false is not an error.
func (r *Repository) LoadCachedPrediction(ctx context.Context, key string, output any) (bool, error) {
	if !r.RedisEnabled() {
		return false, nil
	}
	opCtx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	payload, err := r.redisClient.Get(opCtx, key).Bytes()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("leyendo cache Redis: %w", err)
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return false, fmt.Errorf("leyendo JSON de cache: %w", err)
	}
	return true, nil
}

// PredictionCacheKey is deterministic: the same model version and same raw feature vector map to one Redis key.
func PredictionCacheKey(modelVersion int, featureNames []string, values map[string]float64) string {
	names := append([]string(nil), featureNames...)
	// Caller normally supplies model order, but sort protects deterministic behavior if input is custom.
	if len(names) == 0 {
		for name := range values {
			names = append(names, name)
		}
		sort.Strings(names)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "v=%d|", modelVersion)
	for _, name := range names {
		fmt.Fprintf(&builder, "%s=%.12g|", name, values[name])
	}
	hash := sha256.Sum256([]byte(builder.String()))
	return "pc4:prediction:" + hex.EncodeToString(hash[:])
}
