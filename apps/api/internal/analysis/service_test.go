package analysis

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

func TestRunWithExplicitFixtureProvider(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	if _, err := repo.CreateSite(context.Background(), site); err != nil {
		t.Fatal(err)
	}
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	run, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || run.OverallScore == nil {
		t.Fatalf("unexpected run: %+v", run)
	}
	for _, metric := range run.Metrics {
		if metric.Source.Type != "fixture" || metric.Status != domain.DataPreliminary {
			t.Fatalf("fixture provenance was not preserved: %+v", metric)
		}
	}
}

func TestUnavailableProviderDoesNotInventScore(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	_, _ = repo.CreateSite(context.Background(), site)
	engine, _ := scoring.New(scoring.DefaultWeights)
	run, err := NewService(repo, provider.UnavailableProvider{}, engine).Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	if run.OverallScore != nil {
		t.Fatal("overall score must be absent when required factual data is missing")
	}
}
