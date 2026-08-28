package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var MetricTypes = []string{"traffic", "ev_demand", "population", "poi", "competition", "flood", "electrical"}

type Observation struct {
	MetricType      string
	RawValue        json.RawMessage
	NormalizedScore *float64
	Status          domain.DataStatus
	Source          domain.DataSource
	Assumptions     []string
}

type AnalysisProvider interface {
	Collect(context.Context, domain.Site, int) ([]Observation, error)
}

// UnavailableProvider is the safe production default until factual providers are configured.
// It never fabricates a location fact or score.
type UnavailableProvider struct{}

func (UnavailableProvider) Collect(_ context.Context, _ domain.Site, _ int) ([]Observation, error) {
	now := time.Now().UTC()
	result := make([]Observation, 0, len(MetricTypes))
	for _, metricType := range MetricTypes {
		assumptions := []string{"No factual provider has been configured for this metric."}
		if metricType == "electrical" {
			assumptions = []string{"Utility capacity has not been verified with MEA or PEA."}
		}
		result = append(result, Observation{
			MetricType:  metricType,
			Status:      domain.DataMissing,
			Source:      domain.DataSource{Name: "Unavailable", Type: "unavailable", RetrievedAt: now},
			Assumptions: assumptions,
		})
	}
	return result, nil
}

// FixtureProvider exists only for automated tests and explicit demo environments.
// Values are deterministic fixtures, clearly labelled preliminary, and are not factual claims.
type FixtureProvider struct{}

func (FixtureProvider) Collect(_ context.Context, _ domain.Site, _ int) ([]Observation, error) {
	now := time.Now().UTC()
	values := map[string]float64{"traffic": 82, "ev_demand": 76, "population": 88, "poi": 84, "competition": 62, "flood": 75, "electrical": 55}
	result := make([]Observation, 0, len(MetricTypes))
	for _, metricType := range MetricTypes {
		score := values[metricType]
		result = append(result, Observation{
			MetricType:      metricType,
			RawValue:        json.RawMessage(`{"fixture":true}`),
			NormalizedScore: &score,
			Status:          domain.DataPreliminary,
			Source: domain.DataSource{
				Name:        "Deterministic development fixture",
				Type:        "fixture",
				RetrievedAt: now,
				Methodology: "Static values used only to verify application workflow.",
			},
			Assumptions: []string{"Not factual location data; never use for an investment decision."},
		})
	}
	return result, nil
}
