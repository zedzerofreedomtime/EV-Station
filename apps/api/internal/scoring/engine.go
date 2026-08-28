package scoring

import (
	"errors"
	"math"
)

var (
	ErrInvalidWeights = errors.New("scoring weights must total 1.0")
	ErrMissingMetric  = errors.New("required metric score is missing")
)

var DefaultWeights = map[string]float64{
	"traffic":     0.25,
	"ev_demand":   0.20,
	"population":  0.15,
	"poi":         0.15,
	"competition": 0.15,
	"flood":       0.05,
	"electrical":  0.05,
}

type Engine struct{ Weights map[string]float64 }

func New(weights map[string]float64) (*Engine, error) {
	total := 0.0
	for _, weight := range weights {
		if weight < 0 {
			return nil, ErrInvalidWeights
		}
		total += weight
	}
	if math.Abs(total-1) > 0.000001 {
		return nil, ErrInvalidWeights
	}
	return &Engine{Weights: weights}, nil
}

func (e *Engine) Calculate(scores map[string]*float64) (float64, error) {
	total := 0.0
	for metric, weight := range e.Weights {
		score, ok := scores[metric]
		if !ok || score == nil {
			return 0, ErrMissingMetric
		}
		if *score < 0 || *score > 100 {
			return 0, errors.New("metric score must be between 0 and 100")
		}
		total += *score * weight
	}
	return math.Round(total*100) / 100, nil
}
