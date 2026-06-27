package features

import (
	"math"
	"testing"
)

func TestFitMinMaxUsesTrainingOnly(t *testing.T) {
	train := []Sample{
		{X: []float64{0, 10}, Y: 0},
		{X: []float64{10, 20}, Y: 1},
	}
	test := []Sample{{X: []float64{100, 15}, Y: 1}}

	normalizer, err := FitMinMax(train)
	if err != nil {
		t.Fatalf("FitMinMax() error = %v", err)
	}
	if normalizer.Mins[0] != 0 || normalizer.Maxs[0] != 10 {
		t.Fatalf("normalizer used unexpected bounds: mins=%v maxs=%v", normalizer.Mins, normalizer.Maxs)
	}

	normalizedTest, err := normalizer.TransformSamples(test)
	if err != nil {
		t.Fatalf("TransformSamples() error = %v", err)
	}
	if got, want := normalizedTest[0].X[0], 10.0; got != want {
		t.Fatalf("test value was fitted with leakage: got %v, want %v", got, want)
	}
	if got, want := normalizedTest[0].X[1], 0.5; math.Abs(got-want) > 1e-12 {
		t.Fatalf("second feature = %v, want %v", got, want)
	}
}

func TestConstantFeatureNormalizesToZero(t *testing.T) {
	normalizer, err := FitMinMax([]Sample{{X: []float64{5}}, {X: []float64{5}}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := normalizer.TransformVector([]float64{5})
	if err != nil {
		t.Fatal(err)
	}
	if got[0] != 0 {
		t.Fatalf("constant feature = %v, want 0", got[0])
	}
}
