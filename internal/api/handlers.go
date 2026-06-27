package api

import (
	"encoding/json"
	"net/http"
	"time"
)

type Server struct{ service *Service }

func NewServer(service *Service) *Server { return &Server{service: service} }

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleRoot)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/cluster/status", s.handleClusterStatus)
	mux.HandleFunc("POST /v1/train", s.handleTrain)
	mux.HandleFunc("GET /v1/model", s.handleModel)
	mux.HandleFunc("GET /v1/metrics", s.handleMetrics)
	mux.HandleFunc("POST /v1/evaluate", s.handleEvaluate)
	mux.HandleFunc("POST /v1/predict", s.handlePredict)
	mux.HandleFunc("GET /v1/storage/status", s.handleStorageStatus)
	return withJSONErrors(mux)
}
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"service": "pc4-api-coordinator", "status": "running", "endpoints": []string{"GET /health", "GET /v1/cluster/status", "GET /v1/storage/status", "POST /v1/train", "POST /v1/predict", "GET /v1/model", "GET /v1/metrics", "POST /v1/evaluate"}})
}
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	cluster := s.service.CheckCluster(r.Context())
	code := http.StatusOK
	if cluster.Status == "unavailable" {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"service": "pc4-api-coordinator", "status": cluster.Status, "cluster": cluster, "response_time_ms": time.Since(started).Milliseconds()})
}
func (s *Server) handleClusterStatus(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	cluster := s.service.CheckCluster(r.Context())
	code := http.StatusOK
	if cluster.Status == "unavailable" {
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{"cluster": cluster, "response_time_ms": time.Since(started).Milliseconds()})
}
func (s *Server) handleTrain(w http.ResponseWriter, r *http.Request) {
	var input TrainRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "JSON de entrenamiento inválido", "detail": err.Error()})
		return
	}
	report, err := s.service.TrainDistributed(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo entrenar el modelo distribuido", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "trained", "training": report, "model": s.service.ModelSnapshot()})
}

func (s *Server) handlePredict(w http.ResponseWriter, r *http.Request) {
	var input PredictRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "JSON de predicción inválido", "detail": err.Error()})
		return
	}
	result, err := s.service.Predict(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "no se pudo generar la predicción", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleStorageStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.service.StorageHealth(r.Context()))
}

func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.service.ModelSnapshot())
}
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	snapshot := s.service.ModelSnapshot()
	if snapshot.LastTraining == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "aún no existe entrenamiento distribuido"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"model_version": snapshot.Version, "training": snapshot.LastTraining})
}
func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	report, err := s.service.EvaluateCurrentModel(r.Context())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "no se pudo evaluar el modelo", "detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "evaluated", "evaluation": report})
}

func withJSONErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { next.ServeHTTP(w, r) })
}
func writeJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(payload)
}
