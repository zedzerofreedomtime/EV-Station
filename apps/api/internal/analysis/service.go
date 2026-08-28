package analysis

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

type Service struct {
	repo     repository.Repository
	provider provider.AnalysisProvider
	scoring  *scoring.Engine
}

func NewService(repo repository.Repository, dataProvider provider.AnalysisProvider, scoringEngine *scoring.Engine) *Service {
	return &Service{repo: repo, provider: dataProvider, scoring: scoringEngine}
}

func (s *Service) Run(ctx context.Context, siteID uuid.UUID, radius int) (domain.AnalysisRun, error) {
	site, err := s.repo.GetSite(ctx, siteID)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	if radius == 0 {
		radius = 3000
	}
	now := time.Now().UTC()
	run := domain.AnalysisRun{
		ID: uuid.New(), SiteID: siteID, Status: "running", AnalysisRadiusMeters: radius,
		AssessmentStatus: domain.DataPreliminary, Recommendation: "Analysis in progress.", StartedAt: now, CreatedAt: now,
	}
	if _, err = s.repo.CreateAnalysis(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}

	observations, err := s.provider.Collect(ctx, site, radius)
	if err != nil {
		run.Status = "failed"
		run.Recommendation = "Analysis failed while collecting provider data."
		_ = s.repo.CompleteAnalysis(ctx, run)
		return run, err
	}

	scores := make(map[string]*float64, len(observations))
	hasMissing := false
	for _, observation := range observations {
		metric := domain.Metric{
			ID: uuid.New(), AnalysisRunID: run.ID, Type: observation.MetricType,
			RawValue: observation.RawValue, NormalizedScore: observation.NormalizedScore,
			Status: observation.Status, Source: observation.Source, Assumptions: observation.Assumptions, CreatedAt: time.Now().UTC(),
		}
		run.Metrics = append(run.Metrics, metric)
		scores[observation.MetricType] = observation.NormalizedScore
		if observation.NormalizedScore == nil || observation.Status == domain.DataMissing {
			hasMissing = true
		}
	}

	if !hasMissing {
		if score, scoreErr := s.scoring.Calculate(scores); scoreErr == nil {
			run.OverallScore = &score
			run.Recommendation = "Preliminary result only. Verify provider data and assumptions before making an investment decision."
		}
	} else {
		run.AssessmentStatus = domain.DataMissing
		run.Recommendation = "A recommendation is unavailable until the required factual data is supplied or verified."
	}
	run.Status = "completed"
	completed := time.Now().UTC()
	run.CompletedAt = &completed
	if err = s.repo.CompleteAnalysis(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}
	return run, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	return s.repo.GetAnalysis(ctx, id)
}
