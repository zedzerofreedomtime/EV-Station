package analysis

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
)

type Service struct {
	repo     repository.Repository
	provider provider.AnalysisProvider
	scoring  *scoring.Engine
	aiScorer *advisory.Service
}

func NewService(repo repository.Repository, dataProvider provider.AnalysisProvider, scoringEngine *scoring.Engine, aiScorer ...*advisory.Service) *Service {
	service := &Service{repo: repo, provider: dataProvider, scoring: scoringEngine}
	if len(aiScorer) > 0 {
		service.aiScorer = aiScorer[0]
	}
	return service
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

	for _, observation := range observations {
		metric := domain.Metric{
			ID: uuid.New(), AnalysisRunID: run.ID, Type: observation.MetricType,
			RawValue: observation.RawValue, NormalizedScore: observation.NormalizedScore,
			Status: observation.Status, Source: observation.Source, Assumptions: observation.Assumptions, CreatedAt: time.Now().UTC(),
		}
		run.Metrics = append(run.Metrics, metric)
	}

	result := s.scoring.EvaluatePreliminary(run.Metrics)
	s.applyDeterministicScore(&run, result)
	s.applyGeminiScore(ctx, &run)
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

func (s *Service) GetLatestCompletedForSite(ctx context.Context, siteID uuid.UUID) (domain.AnalysisRun, error) {
	return s.repo.GetLatestCompletedAnalysisForSite(ctx, siteID)
}

// RecalculatePreliminary applies the current deterministic screening rules to
// the evidence already stored on a completed run. This makes older runs usable
// without silently replacing their provider data.
func (s *Service) RecalculatePreliminary(ctx context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	run, err := s.repo.GetAnalysis(ctx, id)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	result := s.scoring.EvaluatePreliminary(run.Metrics)
	s.applyDeterministicScore(&run, result)
	s.applyGeminiScore(ctx, &run)
	if err := s.repo.UpdateAnalysisScoring(ctx, run); err != nil {
		return domain.AnalysisRun{}, err
	}
	return run, nil
}

func (s *Service) applyDeterministicScore(run *domain.AnalysisRun, result scoring.PreliminaryResult) {
	run.Scoring = &result.Summary
	for index := range run.Metrics {
		if score, exists := result.MetricScores[run.Metrics[index].Type]; exists {
			run.Metrics[index].NormalizedScore = &score
			run.Metrics[index].Assumptions = appendScoringRule(run.Metrics[index].Assumptions, scoring.ScoringRuleAssumption(result.Rules[run.Metrics[index].Type]))
		} else {
			run.Metrics[index].NormalizedScore = nil
		}
	}
	if result.Overall != nil {
		run.OverallScore = result.Overall
		run.AssessmentStatus = domain.DataPreliminary
		run.Recommendation = "Preliminary screening score is available. Review data coverage, sources and assumptions before any investment decision."
		return
	}
	run.OverallScore = nil
	run.AssessmentStatus = domain.DataMissing
	run.Recommendation = "A preliminary score requires at least 60% weighted data coverage."
}

func (s *Service) applyGeminiScore(ctx context.Context, run *domain.AnalysisRun) {
	if s.aiScorer == nil || run.OverallScore == nil {
		return
	}
	aiResult, err := s.aiScorer.Score(ctx, *run, "th")
	if err != nil {
		// A missing key, quota exhaustion or invalid model output must never hide
		// the evidence-backed screening score already available to staff.
		return
	}
	eligible := make(map[string]bool, len(run.Metrics))
	for index := range run.Metrics {
		if run.Metrics[index].NormalizedScore != nil {
			eligible[run.Metrics[index].Type] = true
		}
	}
	updated := 0
	for index := range run.Metrics {
		metric := &run.Metrics[index]
		score, ok := aiResult.MetricScores[metric.Type]
		if !ok || !eligible[metric.Type] || score < 0 || score > 100 {
			continue
		}
		metric.NormalizedScore = &score
		metric.Assumptions = appendScoringRule(metric.Assumptions, "Gemini assisted scoring: proposed from the collected evidence only; requires staff review.")
		updated++
	}
	if updated == 0 {
		return
	}
	result := s.scoring.EvaluatePreliminary(run.Metrics)
	if result.Overall == nil {
		return
	}
	run.OverallScore = result.Overall
	run.Scoring = &result.Summary
	run.Scoring.Version = "gemini-assisted-v1"
	run.Scoring.Limitations = append(run.Scoring.Limitations,
		"Gemini proposed the individual metric scores from the collected evidence; the backend validated the values and calculated the weighted total.",
		"Gemini model: "+aiResult.Model+". The recommendation remains preliminary and requires staff review.",
	)
	run.Recommendation = aiResult.Recommendation
	if run.Recommendation == "" {
		run.Recommendation = "AI-assisted preliminary screening score is available. Review data coverage, sources and assumptions before any investment decision."
	}
}

func appendScoringRule(assumptions []string, rule string) []string {
	for _, assumption := range assumptions {
		if assumption == rule {
			return assumptions
		}
	}
	return append(assumptions, rule)
}
