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
	if err := ValidateLocation(input.Address, input.Latitude, input.Longitude); err != nil {
		return domain.Site{}, err
	}
	now := time.Now().UTC()
	site := domain.Site{
		ID: uuid.New(), Name: strings.TrimSpace(input.Name), Address: strings.TrimSpace(input.Address),
		Latitude: input.Latitude, Longitude: input.Longitude, LandSize: input.LandSize,
		LandSizeUnit: input.LandSizeUnit, GoogleMapsURL: input.GoogleMapsURL, Notes: input.Notes,
		InputStatus: domain.DataPreliminary, CreatedAt: now, UpdatedAt: now,
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
