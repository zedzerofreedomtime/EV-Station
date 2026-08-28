package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type WorldPopConfig struct {
	Endpoint   string
	Year       int
	Resolution string
	CacheTTL   time.Duration
	UserAgent  string
}

type WorldPopProvider struct {
	config WorldPopConfig
	client *http.Client
	cache  cache.Cache
}

type worldPopTaskResponse struct {
	TaskID string          `json:"task_id"`
	Status string          `json:"status"`
	Result *worldPopResult `json:"result"`
	Error  any             `json:"error"`
}

type worldPopResult struct {
	TotalPopulation   float64 `json:"total_population"`
	AreaKM2           float64 `json:"area_km2"`
	DataYear          int     `json:"data_year"`
	DataSource        string  `json:"data_source"`
	PopulationDensity float64 `json:"population_density"`
}

type worldPopMetricValue struct {
	Population        float64 `json:"population"`
	PopulationDensity float64 `json:"populationDensityPerKm2"`
	AreaKM2           float64 `json:"areaKm2"`
	RadiusMeters      int     `json:"radiusMeters"`
	DataYear          int     `json:"dataYear"`
	Resolution        string  `json:"resolution"`
}

func NewWorldPopProvider(config WorldPopConfig, client *http.Client, externalCache cache.Cache) *WorldPopProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.Endpoint == "" {
		config.Endpoint = "https://api.worldpop.org/v2"
	}
	if config.Year == 0 {
		config.Year = time.Now().Year()
	}
	if config.Resolution == "" {
		config.Resolution = "100m"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 30 * 24 * time.Hour
	}
	return &WorldPopProvider{config: config, client: client, cache: externalCache}
}

func (p *WorldPopProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["population"]] = p.missing("Valid coordinates are required to query WorldPop population data.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}
	result, err := p.fetch(ctx, *site.Latitude, *site.Longitude, radius)
	if err != nil {
		observations[positions["population"]] = p.missing("WorldPop was unavailable or did not complete in time; no population value was produced.")
		return observations, nil
	}
	raw, _ := json.Marshal(worldPopMetricValue{Population: result.TotalPopulation, PopulationDensity: result.PopulationDensity, AreaKM2: result.AreaKM2, RadiusMeters: radius, DataYear: result.DataYear, Resolution: p.config.Resolution})
	observations[positions["population"]] = Observation{
		MetricType: "population", RawValue: raw, Status: domain.DataEstimated,
		Source: domain.DataSource{
			Name: "WorldPop Global 2 Population Data", Type: "modelled_open_data_api",
			ReferenceURI: "https://api.worldpop.org/v2/", DatasetVersion: result.DataSource,
			RetrievedAt: time.Now().UTC(), Methodology: "WorldPop population cells aggregated by its API within a generated circular polygon around the site.",
			License: "WorldPop data terms and dataset citation apply",
		},
		Assumptions: []string{
			"WorldPop is a modelled population estimate, not an official census count for the site.",
			"The query uses a 32-segment approximation of the requested radius.",
			"Population is not converted into a suitability score in this MVP.",
		},
	}
	return observations, nil
}

func (p *WorldPopProvider) fetch(ctx context.Context, latitude, longitude float64, radius int) (worldPopResult, error) {
	payload := map[string]any{"geojson": circlePolygon(latitude, longitude, radius, 32), "year": p.config.Year, "resolution": p.config.Resolution}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return worldPopResult{}, err
	}
	hash := sha256.Sum256(encoded)
	cacheKey := "worldpop:population:" + hex.EncodeToString(hash[:])
	if cached, found, cacheErr := p.cache.Get(ctx, cacheKey); cacheErr == nil && found {
		var result worldPopResult
		if json.Unmarshal(cached, &result) == nil {
			return result, nil
		}
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(p.config.Endpoint, "/")+"/population", bytes.NewReader(encoded))
	if err != nil {
		return worldPopResult{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(request)
	if err != nil {
		return worldPopResult{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return worldPopResult{}, fmt.Errorf("worldpop submit returned status %d", response.StatusCode)
	}
	var task worldPopTaskResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&task); err != nil || task.TaskID == "" {
		return worldPopResult{}, fmt.Errorf("invalid worldpop task response")
	}

	for attempt := 0; attempt < 20; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return worldPopResult{}, ctx.Err()
			case <-time.After(300 * time.Millisecond):
			}
		}
		result, done, pollErr := p.poll(ctx, task.TaskID)
		if pollErr != nil {
			return worldPopResult{}, pollErr
		}
		if done {
			cached, _ := json.Marshal(result)
			_ = p.cache.Set(ctx, cacheKey, cached, p.config.CacheTTL)
			return result, nil
		}
	}
	return worldPopResult{}, fmt.Errorf("worldpop task timed out")
}

func (p *WorldPopProvider) poll(ctx context.Context, taskID string) (worldPopResult, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(p.config.Endpoint, "/")+"/tasks/"+taskID, nil)
	if err != nil {
		return worldPopResult{}, false, err
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(request)
	if err != nil {
		return worldPopResult{}, false, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return worldPopResult{}, false, fmt.Errorf("worldpop task returned status %d", response.StatusCode)
	}
	var task worldPopTaskResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&task); err != nil {
		return worldPopResult{}, false, err
	}
	if task.Status == "failure" {
		return worldPopResult{}, false, fmt.Errorf("worldpop task failed")
	}
	if task.Status == "success" && task.Result != nil {
		return *task.Result, true, nil
	}
	return worldPopResult{}, false, nil
}

func (p *WorldPopProvider) missing(assumption string) Observation {
	return Observation{MetricType: "population", Status: domain.DataMissing, Source: domain.DataSource{Name: "WorldPop Global 2 Population Data", Type: "modelled_open_data_api", ReferenceURI: "https://api.worldpop.org/v2/", RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}

func circlePolygon(latitude, longitude float64, radius, segments int) map[string]any {
	coordinates := make([][]float64, 0, segments+1)
	latRadians := latitude * math.Pi / 180
	for index := 0; index <= segments; index++ {
		angle := 2 * math.Pi * float64(index) / float64(segments)
		latitudeOffset := float64(radius) / 111320 * math.Sin(angle)
		longitudeOffset := float64(radius) / (111320 * math.Cos(latRadians)) * math.Cos(angle)
		coordinates = append(coordinates, []float64{longitude + longitudeOffset, latitude + latitudeOffset})
	}
	return map[string]any{"type": "Polygon", "coordinates": []any{coordinates}}
}
