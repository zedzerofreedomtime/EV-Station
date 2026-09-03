package scoring

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const (
	PreliminaryVersion        = "preliminary-v1"
	MinimumCoveragePercentage = 60.0
)

type PreliminaryResult struct {
	Overall      *float64
	MetricScores map[string]float64
	Rules        map[string]string
	Summary      domain.ScoringSummary
}

// EvaluatePreliminary converts provider evidence into transparent screening
// scores. It never changes the evidence status and excludes metrics that cannot
// be interpreted safely (notably unverified electrical capacity).
func (e *Engine) EvaluatePreliminary(metrics []domain.Metric) PreliminaryResult {
	result := PreliminaryResult{
		MetricScores: make(map[string]float64),
		Rules:        make(map[string]string),
		Summary: domain.ScoringSummary{
			Version: PreliminaryVersion, RequiredMetricCount: len(e.Weights),
			MinimumCoveragePercent: MinimumCoveragePercentage,
			Limitations: []string{
				"This score is a deterministic preliminary screening indicator, not an investment approval.",
				"Missing metrics are excluded and the available weights are renormalized; review coverage before comparing sites.",
				"Electrical capacity is excluded unless a utility-confirmed site value becomes available.",
			},
		},
	}

	availableWeight := 0.0
	weightedTotal := 0.0
	seen := make(map[string]bool, len(metrics))
	for _, metric := range metrics {
		weight, required := e.Weights[metric.Type]
		if !required {
			continue
		}
		seen[metric.Type] = true
		score, rule, ok := preliminaryMetricScore(metric)
		if !ok {
			result.Summary.ExcludedMetrics = append(result.Summary.ExcludedMetrics, metric.Type)
			continue
		}
		score = roundTwo(clamp(score, 0, 100))
		result.MetricScores[metric.Type] = score
		result.Rules[metric.Type] = rule
		availableWeight += weight
		weightedTotal += score * weight
	}
	for metricType := range e.Weights {
		if !seen[metricType] {
			result.Summary.ExcludedMetrics = append(result.Summary.ExcludedMetrics, metricType)
		}
	}
	sort.Strings(result.Summary.ExcludedMetrics)
	result.Summary.ScoredMetricCount = len(result.MetricScores)
	result.Summary.CoveragePercentage = roundTwo(availableWeight * 100)
	if result.Summary.CoveragePercentage >= MinimumCoveragePercentage && availableWeight > 0 {
		overall := roundTwo(weightedTotal / availableWeight)
		result.Overall = &overall
	}
	return result
}

func preliminaryMetricScore(metric domain.Metric) (float64, string, bool) {
	if metric.NormalizedScore != nil {
		return *metric.NormalizedScore, "Provider-supplied deterministic normalized score.", true
	}
	if metric.Status == domain.DataMissing || len(metric.RawValue) == 0 {
		return 0, "", false
	}
	switch metric.Type {
	case "traffic":
		var value struct {
			AADT float64 `json:"aadt"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.AADT >= 0 {
			return 20 + 80*(value.AADT/80000), "AADT screening rule: 20 points at zero, increasing linearly to 100 at 80,000 vehicles/day.", true
		}
	case "road_accessibility":
		var road struct {
			MappedMajorRoadCount int      `json:"mappedMajorRoadCount"`
			NearestMeters        *float64 `json:"nearestMajorRoadMeters"`
		}
		if json.Unmarshal(metric.RawValue, &road) == nil && road.NearestMeters != nil {
			proximity := 100 - (*road.NearestMeters / 20)
			count := math.Min(float64(road.MappedMajorRoadCount)*2, 100)
			return proximity*0.7 + count*0.3, "Road-accessibility rule: 70% nearest-major-road proximity and 30% mapped major-road count; this does not verify the actual entrance, turning movements, road width or on-site obstructions.", true
		}
	case "ev_demand":
		var value struct {
			RegisteredBEV float64 `json:"registeredBev"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.RegisteredBEV >= 0 {
			return 20 + 80*(value.RegisteredBEV/300000), "Provincial BEV screening rule: 20 points at zero, increasing linearly to 100 at 300,000 registered BEVs; not local demand.", true
		}
	case "population":
		var value struct {
			Density float64 `json:"populationDensityPerKm2"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Density >= 0 {
			return 10 + 90*(value.Density/6000), "Population rule: modelled density increases linearly from 10 to 100 points at 6,000 people/km².", true
		}
	case "poi":
		var value struct {
			Count  float64 `json:"count"`
			Radius float64 `json:"radiusMeters"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Count >= 0 && value.Radius > 0 {
			areaKM2 := math.Pi * math.Pow(value.Radius/1000, 2)
			density := value.Count / areaKM2
			return 10 + 90*(density/8), "POI rule: mapped POI density increases linearly from 10 to 100 points at 8 POIs/km².", true
		}
	case "competition":
		var value struct {
			Count           float64 `json:"count"`
			CoverageMatched bool    `json:"coverageMatched"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Count >= 0 && value.CoverageMatched {
			return 80 - value.Count*8, "Competition rule: starts at 80 with no mapped competitors and subtracts 8 points per deduplicated station, with a 0–100 clamp.", true
		}
	case "flood":
		var value struct {
			Count float64 `json:"mappedFloodRiskAreaCount"`
		}
		if json.Unmarshal(metric.RawValue, &value) == nil && value.Count >= 0 {
			if value.Count == 0 {
				return 80, "Flood-layer rule: 80 when no published risk polygon overlaps the radius; this does not mean zero flood risk.", true
			}
			return 40 - (value.Count-1)*5, "Flood-layer rule: 40 for one overlapping published risk polygon, minus 5 for each additional polygon.", true
		}
	case "electrical":
		// Public planning maps do not verify remaining capacity for a plot.
		return 0, "", false
	}
	return 0, "", false
}

func ScoringRuleAssumption(rule string) string {
	return fmt.Sprintf("Deterministic %s scoring rule: %s", PreliminaryVersion, rule)
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}

func roundTwo(value float64) float64 { return math.Round(value*100) / 100 }
