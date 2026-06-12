package features

import (
	"math"
	"sort"
	"time"

	"github.com/Axel-Pariona/pc3-consumo-electrico-go/internal/preprocessing"
)

// FeatureNames define el orden exacto de variables usadas por el modelo.
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

// Sample representa un registro transformado para Machine Learning.
type Sample struct {
	X []float64 `json:"x"`
	Y int       `json:"y"`
}

// Summary resume la generacion de features y la definicion de la variable objetivo.
type Summary struct {
	FeatureNames        []string `json:"feature_names"`
	TotalSamples        int      `json:"total_samples"`
	HighDemandThreshold float64  `json:"high_demand_threshold_p75"`
	HighDemandCount     int      `json:"high_demand_count"`
	NormalDemandCount   int      `json:"normal_demand_count"`
	Normalization       string   `json:"normalization"`
	TargetDefinition    string   `json:"target_definition"`
}

// BuildSamples genera variables predictoras y la etiqueta high_demand.
// high_demand = 1 cuando Global_active_power >= percentil 75.
func BuildSamples(records []preprocessing.PowerRecord) ([]Sample, Summary) {
	if len(records) == 0 {
		return nil, Summary{FeatureNames: FeatureNames}
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

	NormalizeMinMax(samples)

	summary := Summary{
		FeatureNames:        FeatureNames,
		TotalSamples:        len(samples),
		HighDemandThreshold: threshold,
		HighDemandCount:     high,
		NormalDemandCount:   len(samples) - high,
		Normalization:       "min-max por columna sobre el conjunto limpio",
		TargetDefinition:    "high_demand = 1 si Global_active_power >= percentil 75; 0 en caso contrario",
	}
	return samples, summary
}

// OtherConsumption estima el consumo que no corresponde a los tres submedidores.
// Global_active_power esta en kW; al multiplicar por 1000/60 se aproxima Wh por minuto.
func OtherConsumption(r preprocessing.PowerRecord) float64 {
	other := (r.GlobalActivePower * 1000.0 / 60.0) - r.SubMetering1 - r.SubMetering2 - r.SubMetering3
	if other < 0 && math.Abs(other) < 1e-9 {
		return 0
	}
	return other
}

// NormalizeMinMax escala cada columna al rango [0,1].
func NormalizeMinMax(samples []Sample) {
	if len(samples) == 0 || len(samples[0].X) == 0 {
		return
	}

	n := len(samples[0].X)
	mins := make([]float64, n)
	maxs := make([]float64, n)
	for j := 0; j < n; j++ {
		mins[j] = math.Inf(1)
		maxs[j] = math.Inf(-1)
	}

	for _, s := range samples {
		for j, v := range s.X {
			if v < mins[j] {
				mins[j] = v
			}
			if v > maxs[j] {
				maxs[j] = v
			}
		}
	}

	for i := range samples {
		for j, v := range samples[i].X {
			rangeValue := maxs[j] - mins[j]
			if rangeValue == 0 {
				samples[i].X[j] = 0
				continue
			}
			samples[i].X[j] = (v - mins[j]) / rangeValue
		}
	}
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
