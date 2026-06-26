package distributed

import "github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/ml"

const (
	MessageHealth         = "health"
	MessageHealthResponse = "health_response"
	MessageGradient       = "compute_gradient"
	MessageGradientResult = "gradient_response"
	MessageError          = "error"
)

// Request is sent by a coordinator/client to an ML node through TCP as one JSON line.
type Request struct {
	Type      string    `json:"type"`
	RequestID string    `json:"request_id"`
	Iteration int       `json:"iteration,omitempty"`
	Weights   []float64 `json:"weights,omitempty"`
	Bias      float64   `json:"bias,omitempty"`
}

// Response is returned by an ML node. A single schema keeps TCP troubleshooting simple.
type Response struct {
	Type         string    `json:"type"`
	RequestID    string    `json:"request_id,omitempty"`
	NodeID       string    `json:"node_id,omitempty"`
	Status       string    `json:"status,omitempty"`
	Samples      int       `json:"samples,omitempty"`
	FeatureCount int       `json:"feature_count,omitempty"`
	Iteration    int       `json:"iteration,omitempty"`
	Gradient     []float64 `json:"gradient_weights,omitempty"`
	GradientBias float64   `json:"gradient_bias,omitempty"`
	LossSum      float64   `json:"loss_sum,omitempty"`
	ProcessingMS int64     `json:"processing_ms,omitempty"`
	Error        string    `json:"error,omitempty"`
}

func NewGradientResponse(request Request, nodeID string, result ml.GradientResult, elapsedMS int64) Response {
	return Response{
		Type: MessageGradientResult, RequestID: request.RequestID, NodeID: nodeID,
		Samples: result.Samples, FeatureCount: len(result.Weights), Iteration: request.Iteration,
		Gradient: result.Weights, GradientBias: result.Bias, LossSum: result.LossSum,
		ProcessingMS: elapsedMS,
	}
}
