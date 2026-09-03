package provider

import (
	"context"
	"sync"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

// CompositeProvider runs independent providers concurrently and keeps the
// strongest factual observation for each metric. Missing observations never
// overwrite an available observation.
type CompositeProvider struct {
	providers []AnalysisProvider
}

func NewCompositeProvider(providers ...AnalysisProvider) *CompositeProvider {
	return &CompositeProvider{providers: providers}
}

func (p *CompositeProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	type result struct {
		observations []Observation
		err          error
	}
	results := make(chan result, len(p.providers))
	var wait sync.WaitGroup
	for _, dataProvider := range p.providers {
		wait.Add(1)
		go func(current AnalysisProvider) {
			defer wait.Done()
			observations, err := current.Collect(ctx, site, radius)
			results <- result{observations: observations, err: err}
		}(dataProvider)
	}
	wait.Wait()
	close(results)

	merged, positions := unavailableObservations()
	competitionObservations := make([]Observation, 0, len(p.providers))
	for providerResult := range results {
		if providerResult.err != nil {
			continue
		}
		for _, observation := range providerResult.observations {
			position, exists := positions[observation.MetricType]
			if !exists {
				continue
			}
			if observation.MetricType == "competition" && observation.Status != domain.DataMissing {
				competitionObservations = append(competitionObservations, observation)
				continue
			}
			if observation.Status == domain.DataMissing {
				if merged[position].Status == domain.DataMissing && merged[position].Source.Type == "unavailable" {
					merged[position] = observation
				}
				continue
			}
			if dataStatusStrength(observation.Status) > dataStatusStrength(merged[position].Status) {
				merged[position] = observation
			}
		}
	}
	if len(competitionObservations) > 0 {
		merged[positions["competition"]] = mergeCompetitionObservations(competitionObservations, radius)
	}
	return merged, nil
}

func dataStatusStrength(status domain.DataStatus) int {
	switch status {
	case domain.DataVerified:
		return 3
	case domain.DataEstimated:
		return 2
	case domain.DataPreliminary:
		return 1
	default:
		return 0
	}
}
