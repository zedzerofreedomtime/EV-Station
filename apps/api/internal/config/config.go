package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment           string
	Port                  string
	DatabaseURL           string
	RedisURL              string
	RedisCacheTTL         time.Duration
	CORSAllowedOrigins    []string
	AnalysisProviderMode  string
	OverpassURL           string
	NominatimURL          string
	NominatimCountryCodes string
	GeocodingCacheTTL     time.Duration
	ExternalUserAgent     string
	ExternalHTTPTimeout   time.Duration
	WorldPopURL           string
	WorldPopYear          int
	WorldPopResolution    string
	WorldPopCacheTTL      time.Duration
	GISTDAAPIKey          string
}

func Load() Config {
	ttl, err := time.ParseDuration(getEnv("REDIS_CACHE_TTL", "15m"))
	if err != nil {
		ttl = 15 * time.Minute
	}

	origins := strings.Split(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://127.0.0.1:5173"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	httpTimeout, err := time.ParseDuration(getEnv("EXTERNAL_HTTP_TIMEOUT", "20s"))
	if err != nil {
		httpTimeout = 20 * time.Second
	}
	geocodingTTL, err := time.ParseDuration(getEnv("GEOCODING_CACHE_TTL", "720h"))
	if err != nil {
		geocodingTTL = 30 * 24 * time.Hour
	}
	worldPopTTL, err := time.ParseDuration(getEnv("WORLDPOP_CACHE_TTL", "720h"))
	if err != nil {
		worldPopTTL = 30 * 24 * time.Hour
	}
	worldPopYear := time.Now().Year()
	if parsed, parseErr := strconv.Atoi(getEnv("WORLDPOP_YEAR", strconv.Itoa(worldPopYear))); parseErr == nil {
		worldPopYear = parsed
	}

	return Config{
		Environment:           getEnv("APP_ENV", "development"),
		Port:                  getEnv("API_PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		RedisURL:              os.Getenv("REDIS_URL"),
		RedisCacheTTL:         ttl,
		CORSAllowedOrigins:    origins,
		AnalysisProviderMode:  getEnv("ANALYSIS_PROVIDER_MODE", "unavailable"),
		OverpassURL:           getEnv("OVERPASS_API_URL", "https://overpass-api.de/api/interpreter"),
		NominatimURL:          getEnv("NOMINATIM_API_URL", "https://nominatim.openstreetmap.org/search"),
		NominatimCountryCodes: getEnv("NOMINATIM_COUNTRY_CODES", "th"),
		GeocodingCacheTTL:     geocodingTTL,
		ExternalUserAgent:     getEnv("EXTERNAL_USER_AGENT", "RBC-EV-Station-MVP/0.1 (data-team@rbc.local)"),
		ExternalHTTPTimeout:   httpTimeout,
		WorldPopURL:           getEnv("WORLDPOP_API_URL", "https://api.worldpop.org/v2"),
		WorldPopYear:          worldPopYear,
		WorldPopResolution:    getEnv("WORLDPOP_RESOLUTION", "100m"),
		WorldPopCacheTTL:      worldPopTTL,
		GISTDAAPIKey:          os.Getenv("GISTDA_API_KEY"),
	}
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
