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
	for providerResult := range results {
		if providerResult.err != nil {
			continue
		}
		for _, observation := range providerResult.observations {
			position, exists := positions[observation.MetricType]
			if !exists || observation.Status == domain.DataMissing {
				continue
			}
			merged[position] = observation
		}
	}
	return merged, nil
}
