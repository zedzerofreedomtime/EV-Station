package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestGISTDAFloodProviderPreservesPublishedOverlapWithoutScore(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Fatalf("expected GET, got %s", request.Method)
		}
		if request.URL.Query().Get("returnCountOnly") != "true" || request.URL.Query().Get("distance") != "3000" {
			t.Fatalf("unexpected spatial query: %s", request.URL.RawQuery)
		}
		if request.URL.Query().Get("geometry") != "100.500000,13.700000" {
			t.Fatalf("unexpected point geometry: %s", request.URL.Query().Get("geometry"))
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"count":2}`))
	}))
	defer server.Close()

	latitude, longitude := 13.7, 100.5
	dataProvider := NewGISTDAFloodProvider(GISTDAFloodConfig{Endpoint: server.URL, CacheTTL: time.Hour, UserAgent: "rbc-test"}, server.Client(), cache.Noop{})
	observations, err := dataProvider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	byType := make(map[string]Observation, len(observations))
	for _, observation := range observations {
		byType[observation.MetricType] = observation
	}
	flood := byType["flood"]
	if flood.Status != domain.DataVerified || flood.NormalizedScore != nil {
		t.Fatalf("flood must be a verified published-layer overlap without a score: %+v", flood)
	}
	if flood.Source.Name != "GISTDA Flood Risk Layer (ArcGIS REST)" || !strings.Contains(string(flood.RawValue), `"mappedFloodRiskAreaCount":2`) {
		t.Fatalf("flood provenance was not preserved: %+v", flood)
	}
}

func TestGISTDAFloodProviderDoesNotInventResultWhenUnavailable(t *testing.T) {
	latitude, longitude := 13.7, 100.5
	dataProvider := NewGISTDAFloodProvider(GISTDAFloodConfig{Endpoint: "http://127.0.0.1:1", CacheTTL: time.Hour}, &http.Client{Timeout: 20 * time.Millisecond}, cache.Noop{})
	observations, err := dataProvider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range observations {
		if observation.MetricType == "flood" && (observation.Status != domain.DataMissing || observation.NormalizedScore != nil) {
			t.Fatalf("unavailable flood provider must not invent a result: %+v", observation)
		}
	}
}
