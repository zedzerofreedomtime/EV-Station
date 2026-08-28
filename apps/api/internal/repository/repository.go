package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var ErrNotFound = errorString("not found")

type errorString string

func (e errorString) Error() string { return string(e) }

type Repository interface {
	CreateSite(context.Context, domain.Site) (domain.Site, error)
	ListSites(context.Context) ([]domain.Site, error)
	GetSite(context.Context, uuid.UUID) (domain.Site, error)
	CreateAnalysis(context.Context, domain.AnalysisRun) (domain.AnalysisRun, error)
	CompleteAnalysis(context.Context, domain.AnalysisRun) error
	GetAnalysis(context.Context, uuid.UUID) (domain.AnalysisRun, error)
}
