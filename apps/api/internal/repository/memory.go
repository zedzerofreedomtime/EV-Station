package repository

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type Memory struct {
	mu       sync.RWMutex
	sites    map[uuid.UUID]domain.Site
	analyses map[uuid.UUID]domain.AnalysisRun
}

func NewMemory() *Memory {
	return &Memory{sites: make(map[uuid.UUID]domain.Site), analyses: make(map[uuid.UUID]domain.AnalysisRun)}
}

func (m *Memory) CreateSite(_ context.Context, site domain.Site) (domain.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sites[site.ID] = site
	return site, nil
}

func (m *Memory) ListSites(_ context.Context) ([]domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]domain.Site, 0, len(m.sites))
	for _, site := range m.sites {
		result = append(result, site)
	}
	return result, nil
}

func (m *Memory) GetSite(_ context.Context, id uuid.UUID) (domain.Site, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	site, ok := m.sites[id]
	if !ok {
		return domain.Site{}, ErrNotFound
	}
	return site, nil
}

func (m *Memory) UpdateSite(_ context.Context, site domain.Site) (domain.Site, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[site.ID]; !ok {
		return domain.Site{}, ErrNotFound
	}
	m.sites[site.ID] = site
	return site, nil
}

func (m *Memory) DeleteSite(_ context.Context, id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.sites[id]; !ok {
		return ErrNotFound
	}
	delete(m.sites, id)
	for analysisID, run := range m.analyses {
		if run.SiteID == id {
			delete(m.analyses, analysisID)
		}
	}
	return nil
}

func (m *Memory) CreateAnalysis(_ context.Context, run domain.AnalysisRun) (domain.AnalysisRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyses[run.ID] = run
	return run, nil
}

func (m *Memory) CompleteAnalysis(_ context.Context, run domain.AnalysisRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.analyses[run.ID] = run
	return nil
}

func (m *Memory) UpdateAnalysisScoring(_ context.Context, run domain.AnalysisRun) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.analyses[run.ID]; !ok {
		return ErrNotFound
	}
	m.analyses[run.ID] = run
	return nil
}

func (m *Memory) GetAnalysis(_ context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	run, ok := m.analyses[id]
	if !ok {
		return domain.AnalysisRun{}, ErrNotFound
	}
	return run, nil
}

func (m *Memory) GetLatestCompletedAnalysisForSite(_ context.Context, siteID uuid.UUID) (domain.AnalysisRun, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var latest domain.AnalysisRun
	found := false
	for _, run := range m.analyses {
		if run.SiteID != siteID || run.Status != "completed" {
			continue
		}
		if !found || run.CreatedAt.After(latest.CreatedAt) {
			latest = run
			found = true
		}
	}
	if !found {
		return domain.AnalysisRun{}, ErrNotFound
	}
	return latest, nil
}
