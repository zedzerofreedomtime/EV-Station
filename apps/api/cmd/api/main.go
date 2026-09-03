package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/rbc/ev-station/apps/api/internal/advisory"
	"github.com/rbc/ev-station/apps/api/internal/analysis"
	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/config"
	"github.com/rbc/ev-station/apps/api/internal/httpapi"
	"github.com/rbc/ev-station/apps/api/internal/provider"
	"github.com/rbc/ev-station/apps/api/internal/repository"
	"github.com/rbc/ev-station/apps/api/internal/scoring"
	"github.com/rbc/ev-station/apps/api/internal/site"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	var externalCache cache.Cache = cache.Noop{}
	if strings.TrimSpace(cfg.RedisURL) != "" {
		redisCache, cacheErr := cache.NewRedis(cfg.RedisURL)
		if cacheErr != nil {
			logger.Warn("redis configuration is invalid; cache disabled", "error", cacheErr)
		} else {
			defer redisCache.Close()
			if !redisCache.Available(ctx) {
				logger.Warn("redis unavailable; cache disabled for this run")
			} else {
				externalCache = redisCache
			}
		}
	}

	var repo repository.Repository
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		logger.Warn("DATABASE_URL is not set; using non-persistent in-memory repository")
		repo = repository.NewMemory()
	} else {
		postgresRepo, err := repository.NewPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			logger.Error("database connection failed", "error", err)
			os.Exit(1)
		}
		defer postgresRepo.Close()
		repo = postgresRepo
	}

	var dataProvider provider.AnalysisProvider = provider.UnavailableProvider{}
	if cfg.AnalysisProviderMode == "fixture" && cfg.Environment != "production" {
		logger.Warn("using deterministic development fixture provider; results are not factual")
		dataProvider = provider.FixtureProvider{}
	} else if cfg.AnalysisProviderMode == "osm" {
		logger.Info("using free OpenStreetMap, provincial charger, WorldPop, GISTDA, Department of Highways, DLT, MEA and PEA Power Map providers")
		osmProvider := provider.NewOSMProvider(provider.OSMConfig{
			Endpoint: cfg.OverpassURL, FallbackEndpoints: cfg.OverpassFallbackURLs,
			UserAgent: cfg.ExternalUserAgent, CacheTTL: cfg.RedisCacheTTL,
		}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
		worldPopProvider := provider.NewWorldPopProvider(provider.WorldPopConfig{
			Endpoint: cfg.WorldPopURL, Year: cfg.WorldPopYear, Resolution: cfg.WorldPopResolution,
			CacheTTL: cfg.WorldPopCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
		gistdaFloodProvider := provider.NewGISTDAFloodProvider(provider.GISTDAFloodConfig{
			Endpoint: cfg.GISTDAFloodRiskURL, CacheTTL: cfg.GISTDAFloodCacheTTL,
			UserAgent: cfg.ExternalUserAgent,
		}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
		dohAADTProvider := provider.NewDOHAADTProvider(provider.DOHAADTConfig{
			CSVURL: cfg.DOHAADTCSVURL, RoadLayerURL: cfg.DOHAADTRoadLayerURL, DataYear: cfg.DOHAADTYear,
			CacheTTL: cfg.DOHAADTCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
		dltEVProvider := provider.NewDLTEVRegistrationProvider(provider.DLTEVRegistrationConfig{
			CSVURL: cfg.DLTEVRegistrationCSVURL, DatasetDate: cfg.DLTEVRegistrationDatasetDate,
			PreviousCSVURL: cfg.DLTEVRegistrationPreviousCSVURL, PreviousDatasetDate: cfg.DLTEVRegistrationPreviousDatasetDate,
			CacheTTL: cfg.DLTEVRegistrationCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, nil, externalCache)
		provincialChargerProvider := provider.NewProvincialChargerProvider(provider.ProvincialChargerConfig{
			SuphanburiCSVURL: cfg.ProvincialChargerSuphanburiCSVURL,
			SaraburiJSONURL:  cfg.ProvincialChargerSaraburiJSONURL,
			CacheTTL:         cfg.ProvincialChargerCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
		meaPowerMapProvider := provider.NewMEAPowerMapProvider(provider.MEAPowerMapConfig{
			PageURL: cfg.MEAPowerMapPageURL, DataURL: cfg.MEAPowerMapDataURL,
			Year: cfg.MEAPowerMapYear, VoltageKV: cfg.MEAPowerMapVoltageKV,
			CacheTTL: cfg.MEAPowerMapCacheTTL, UserAgent: cfg.ExternalUserAgent,
		}, nil, externalCache)
		peaGridProvider := provider.NewPEAGridProvider(provider.PEAGridConfig{
			StationURL: cfg.PEAGridStationURL, ConductorURL: cfg.PEAGridConductorURL,
			SearchRadiusMeters: cfg.PEAGridSearchRadiusMeters, CacheTTL: cfg.PEAGridCacheTTL,
			UserAgent: cfg.ExternalUserAgent,
		}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
		dataProvider = provider.NewCompositeProvider(osmProvider, provincialChargerProvider, worldPopProvider, gistdaFloodProvider, dohAADTProvider, dltEVProvider, meaPowerMapProvider, peaGridProvider)
	}

	scoringEngine, err := scoring.New(scoring.DefaultWeights)
	if err != nil {
		logger.Error("invalid scoring configuration", "error", err)
		os.Exit(1)
	}
	siteService := site.NewService(repo)
	geminiAdvisory := advisory.NewGeminiService(advisory.GeminiConfig{APIKey: cfg.GeminiAPIKey, Model: cfg.GeminiModel, BaseURL: cfg.GeminiBaseURL, Timeout: cfg.GeminiTimeout}, nil)
	analysisService := analysis.NewService(repo, dataProvider, scoringEngine, geminiAdvisory)
	geocoder := provider.NewNominatimGeocoder(provider.NominatimConfig{
		Endpoint: cfg.NominatimURL, UserAgent: cfg.ExternalUserAgent,
		CountryCodes: cfg.NominatimCountryCodes, CacheTTL: cfg.GeocodingCacheTTL,
	}, &http.Client{Timeout: cfg.ExternalHTTPTimeout}, externalCache)
	handler := httpapi.NewHandler(siteService, analysisService, geocoder, geminiAdvisory, scoring.DefaultWeights)
	router := httpapi.NewRouter(cfg, handler)
	logger.Info("api listening", "port", cfg.Port, "environment", cfg.Environment)
	if err = router.Run(":" + cfg.Port); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}
