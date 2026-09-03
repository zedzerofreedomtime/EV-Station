package site

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rbc/ev-station/apps/api/internal/domain"
	"github.com/rbc/ev-station/apps/api/internal/repository"
)

var ErrInvalidLocation = errors.New("provide an address or both latitude and longitude")

type Service struct{ repo repository.Repository }

func NewService(repo repository.Repository) *Service { return &Service{repo: repo} }

func (s *Service) Create(ctx context.Context, input domain.CreateSiteInput) (domain.Site, error) {
	return s.save(ctx, uuid.New(), input, time.Now().UTC(), true)
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, input domain.CreateSiteInput) (domain.Site, error) {
	existing, err := s.repo.GetSite(ctx, id)
	if err != nil {
		return domain.Site{}, err
	}
	return s.save(ctx, id, input, existing.CreatedAt, false)
}

func (s *Service) save(ctx context.Context, id uuid.UUID, input domain.CreateSiteInput, createdAt time.Time, isNew bool) (domain.Site, error) {
	if err := ValidateLocation(input.Address, input.Latitude, input.Longitude); err != nil {
		return domain.Site{}, err
	}
	now := time.Now().UTC()
	site := domain.Site{
		ID: id, Name: strings.TrimSpace(input.Name), Address: strings.TrimSpace(input.Address),
		Latitude: input.Latitude, Longitude: input.Longitude, LandSize: input.LandSize,
		LandSizeUnit: input.LandSizeUnit, GoogleMapsURL: input.GoogleMapsURL, Notes: input.Notes,
		InputStatus: domain.DataPreliminary, CreatedAt: createdAt, UpdatedAt: now,
	}
	if !isNew {
		return s.repo.UpdateSite(ctx, site)
	}
	return s.repo.CreateSite(ctx, site)
}

func ValidateLocation(address string, latitude, longitude *float64) error {
	hasAddress := strings.TrimSpace(address) != ""
	hasCoordinates := latitude != nil && longitude != nil
	if !hasAddress && !hasCoordinates {
		return ErrInvalidLocation
	}
	if (latitude == nil) != (longitude == nil) {
		return ErrInvalidLocation
	}
	if hasCoordinates && (*latitude < -90 || *latitude > 90 || *longitude < -180 || *longitude > 180) {
		return ErrInvalidLocation
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]domain.Site, error) { return s.repo.ListSites(ctx) }
func (s *Service) Get(ctx context.Context, id uuid.UUID) (domain.Site, error) {
	return s.repo.GetSite(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error { return s.repo.DeleteSite(ctx, id) }
