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

func TestGetLatestCompletedForSiteReturnsExistingRun(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	_, _ = repo.CreateSite(context.Background(), site)
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	first, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	second, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	latest, err := service.GetLatestCompletedForSite(context.Background(), site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if latest.ID != second.ID || latest.ID == first.ID {
		t.Fatalf("expected latest completed analysis %s, got %s", second.ID, latest.ID)
	}
}

func TestRecalculatePreliminaryUpgradesLegacyEvidenceWithoutReplacingIt(t *testing.T) {
	repo := repository.NewMemory()
	now := time.Now().UTC()
	site := domain.Site{ID: uuid.New(), Name: "Test site", Address: "Bangkok", LandSize: 100, LandSizeUnit: "sqm", CreatedAt: now, UpdatedAt: now}
	_, _ = repo.CreateSite(context.Background(), site)
	engine, _ := scoring.New(scoring.DefaultWeights)
	service := NewService(repo, provider.FixtureProvider{}, engine)
	run, err := service.Run(context.Background(), site.ID, 3000)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate an analysis created before preliminary scoring was introduced.
	run.OverallScore = nil
	run.Scoring = nil
	run.AssessmentStatus = domain.DataMissing
	for index := range run.Metrics {
		run.Metrics[index].NormalizedScore = nil
	}
	if err = repo.UpdateAnalysisScoring(context.Background(), run); err != nil {
		t.Fatal(err)
	}

	recalculated, err := service.RecalculatePreliminary(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recalculated.OverallScore == nil || recalculated.Scoring == nil {
		t.Fatalf("expected legacy evidence to receive a preliminary score: %+v", recalculated)
	}
	if recalculated.Scoring.Version != scoring.PreliminaryVersion {
		t.Fatalf("unexpected scoring version: %s", recalculated.Scoring.Version)
	}
	if recalculated.Metrics[0].Source.Type != "fixture" || recalculated.Metrics[0].Status != domain.DataPreliminary {
		t.Fatal("recalculation must preserve evidence provenance and status")
	}
}
