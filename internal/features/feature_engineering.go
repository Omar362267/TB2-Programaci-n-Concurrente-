package features

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/preprocessing"
)

var FeatureNames = []string{
	"hour",
	"day_of_week",
	"month",
	"is_weekend",
	"voltage",
	"global_reactive_power",
	"global_intensity",
	"sub_metering_1",
	"sub_metering_2",
	"sub_metering_3",
	"other_consumption",
}

type Sample struct {
	X []float64 `json:"x"`
	Y int       `json:"y"`
}

type Normalizer struct {
	Method       string    `json:"method"`
	FeatureNames []string  `json:"feature_names"`
	Mins         []float64 `json:"mins"`
	Maxs         []float64 `json:"maxs"`
	FittedOn     string    `json:"fitted_on"`
}

type Summary struct {
	FeatureNames        []string `json:"feature_names"`
	TotalSamples        int      `json:"total_samples"`
	HighDemandThreshold float64  `json:"high_demand_threshold_p75"`
	HighDemandCount     int      `json:"high_demand_count"`
	NormalDemandCount   int      `json:"normal_demand_count"`
	Normalization       string   `json:"normalization"`
	TargetDefinition    string   `json:"target_definition"`
}

func BuildSamples(records []preprocessing.PowerRecord) ([]Sample, Summary) {
	if len(records) == 0 {
		return nil, Summary{FeatureNames: append([]string(nil), FeatureNames...)}
	}

	threshold := percentile75(records)
	samples := make([]Sample, 0, len(records))
	high := 0

	for _, r := range records {
		other := OtherConsumption(r)
		y := 0
		if r.GlobalActivePower >= threshold {
			y = 1
			high++
		}

		x := []float64{
			float64(r.Timestamp.Hour()),
			float64(r.Timestamp.Weekday()),
			float64(r.Timestamp.Month()),
			boolToFloat(isWeekend(r.Timestamp)),
			r.Voltage,
			r.GlobalReactivePower,
			r.GlobalIntensity,
			r.SubMetering1,
			r.SubMetering2,
			r.SubMetering3,
			other,
		}
		samples = append(samples, Sample{X: x, Y: y})
	}

	summary := Summary{
		FeatureNames:        append([]string(nil), FeatureNames...),
		TotalSamples:        len(samples),
		HighDemandThreshold: threshold,
		HighDemandCount:     high,
		NormalDemandCount:   len(samples) - high,
		Normalization:       "min-max ajustada solo con el conjunto de entrenamiento; los mismos minimos y maximos se aplican a train, test y predicciones futuras",
		TargetDefinition:    "high_demand = 1 si Global_active_power >= percentil 75; 0 en caso contrario",
	}
	return samples, summary
}

func FitMinMax(samples []Sample) (Normalizer, error) {
	if len(samples) == 0 {
		return Normalizer{}, fmt.Errorf("no se puede ajustar normalizador con cero muestras")
	}
	featureCount := len(samples[0].X)
	if featureCount == 0 {
		return Normalizer{}, fmt.Errorf("las muestras no contienen features")
	}

	mins := make([]float64, featureCount)
	maxs := make([]float64, featureCount)
	for j := 0; j < featureCount; j++ {
		mins[j] = math.Inf(1)
		maxs[j] = math.Inf(-1)
	}

	for i, sample := range samples {
		if len(sample.X) != featureCount {
			return Normalizer{}, fmt.Errorf("muestra %d tiene %d features; se esperaban %d", i, len(sample.X), featureCount)
		}
		for j, value := range sample.X {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return Normalizer{}, fmt.Errorf("feature no finita en muestra %d, columna %d", i, j)
			}
			if value < mins[j] {
				mins[j] = value
			}
			if value > maxs[j] {
				maxs[j] = value
			}
		}
	}

	names := append([]string(nil), FeatureNames...)
	if len(names) != featureCount {
		names = make([]string, featureCount)
		for i := range names {
			names[i] = fmt.Sprintf("feature_%d", i)
		}
	}
	return Normalizer{
		Method:       "min-max",
		FeatureNames: names,
		Mins:         mins,
		Maxs:         maxs,
		FittedOn:     "training_set_only",
	}, nil
}

func (n Normalizer) TransformVector(x []float64) ([]float64, error) {
	if len(n.Mins) == 0 || len(n.Maxs) == 0 || len(n.Mins) != len(n.Maxs) {
		return nil, fmt.Errorf("normalizador invalido")
	}
	if len(x) != len(n.Mins) {
		return nil, fmt.Errorf("vector con %d features; se esperaban %d", len(x), len(n.Mins))
	}

	out := make([]float64, len(x))
	for j, value := range x {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("feature no finita en columna %d", j)
		}
		width := n.Maxs[j] - n.Mins[j]
		if width == 0 {
			out[j] = 0
			continue
		}
		out[j] = (value - n.Mins[j]) / width
	}
	return out, nil
}

func (n Normalizer) TransformSamples(samples []Sample) ([]Sample, error) {
	out := make([]Sample, len(samples))
	for i, sample := range samples {
		x, err := n.TransformVector(sample.X)
		if err != nil {
			return nil, fmt.Errorf("normalizando muestra %d: %w", i, err)
		}
		out[i] = Sample{X: x, Y: sample.Y}
	}
	return out, nil
}

func OtherConsumption(r preprocessing.PowerRecord) float64 {
	other := (r.GlobalActivePower * 1000.0 / 60.0) - r.SubMetering1 - r.SubMetering2 - r.SubMetering3
	if other < 0 && math.Abs(other) < 1e-9 {
		return 0
	}
	return other
}

func percentile75(records []preprocessing.PowerRecord) float64 {
	values := make([]float64, len(records))
	for i, r := range records {
		values[i] = r.GlobalActivePower
	}
	sort.Float64s(values)
	idx := int(math.Ceil(0.75*float64(len(values)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(values) {
		idx = len(values) - 1
	}
	return values[idx]
}

func isWeekend(t time.Time) bool {
	return t.Weekday() == time.Saturday || t.Weekday() == time.Sunday
}

func boolToFloat(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
