package scoring

import "testing"

func ptr(value float64) *float64 { return &value }

func TestCalculateWeightedScore(t *testing.T) {
	engine, err := New(DefaultWeights)
	if err != nil {
		t.Fatal(err)
	}
	score, err := engine.Calculate(map[string]*float64{
		"traffic": ptr(82), "ev_demand": ptr(76), "population": ptr(88),
		"poi": ptr(84), "competition": ptr(62), "flood": ptr(75), "electrical": ptr(55),
	})
	if err != nil {
		t.Fatal(err)
	}
	if score != 77.3 {
		t.Fatalf("expected 77.3, got %v", score)
	}
}

func TestCalculateRejectsMissingMetric(t *testing.T) {
	engine, _ := New(DefaultWeights)
	if _, err := engine.Calculate(map[string]*float64{}); err != ErrMissingMetric {
		t.Fatalf("expected ErrMissingMetric, got %v", err)
	}
}
