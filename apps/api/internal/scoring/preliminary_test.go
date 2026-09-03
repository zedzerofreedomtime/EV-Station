package scoring

import (
	"encoding/json"
	"testing"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestPreliminaryScoreUsesAvailableEvidenceAndReportsCoverage(t *testing.T) {
	engine, _ := New(DefaultWeights)
	metrics := []domain.Metric{
		{Type: "traffic", Status: domain.DataVerified, RawValue: json.RawMessage(`{"aadt":80000}`)},
		{Type: "road_accessibility", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"mappedMajorRoadCount":4,"nearestMajorRoadMeters":100}`)},
		{Type: "ev_demand", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"registeredBev":150000}`)},
		{Type: "population", Status: domain.DataEstimated, RawValue: json.RawMessage(`{"populationDensityPerKm2":3000}`)},
		{Type: "poi", Status: domain.DataVerified, RawValue: json.RawMessage(`{"count":100,"radiusMeters":3000}`)},
		{Type: "competition", Status: domain.DataVerified, RawValue: json.RawMessage(`{"count":2,"coverageMatched":true}`)},
		{Type: "flood", Status: domain.DataVerified, RawValue: json.RawMessage(`{"mappedFloodRiskAreaCount":0}`)},
		{Type: "electrical", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"publishedCapacityMw":92}`)},
	}
	result := engine.EvaluatePreliminary(metrics)
	if result.Overall == nil {
		t.Fatal("expected a preliminary score")
	}
	if result.Summary.CoveragePercentage != 95 || result.Summary.ScoredMetricCount != 7 {
		t.Fatalf("unexpected coverage: %+v", result.Summary)
	}
	if _, exists := result.MetricScores["electrical"]; exists {
		t.Fatal("unverified electrical planning data must not be scored")
	}
}

func TestPreliminaryScoreExcludesCompetitionWithoutLocalCoverage(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "competition", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"count":0,"coverageMatched":false}`)}})
	if _, found := result.MetricScores["competition"]; found {
		t.Fatal("competition without local source coverage must not receive a score")
	}
}

func TestPreliminaryScoreRequiresMinimumCoverage(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "traffic", Status: domain.DataVerified, RawValue: json.RawMessage(`{"aadt":20000}`)}})
	if result.Overall != nil || result.Summary.CoveragePercentage != 20 {
		t.Fatalf("low-coverage evidence must not produce an overall score: %+v", result)
	}
}

func TestPreliminaryScoreSeparatesRoadAccessibilityFromTrafficVolume(t *testing.T) {
	engine, _ := New(DefaultWeights)
	result := engine.EvaluatePreliminary([]domain.Metric{{Type: "road_accessibility", Status: domain.DataPreliminary, RawValue: json.RawMessage(`{"mappedMajorRoadCount":3,"nearestMajorRoadMeters":80}`)}})
	if _, found := result.MetricScores["road_accessibility"]; !found {
		t.Fatal("expected road accessibility evidence to receive its own screening score")
	}
	if _, found := result.MetricScores["traffic"]; found {
		t.Fatal("road accessibility must not be recorded as traffic volume")
	}
}
