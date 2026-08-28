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

func TestWorldPopProviderPreservesEstimatedStatusAndMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/population" {
			_, _ = writer.Write([]byte(`{"task_id":"task-1","status":"pending"}`))
			return
		}
		if request.Method == http.MethodGet && request.URL.Path == "/tasks/task-1" {
			_, _ = writer.Write([]byte(`{"task_id":"task-1","status":"success","result":{"total_population":12345.67,"area_km2":28.1,"data_year":2025,"data_source":"worldpop_R2025A_2025_100m","population_density":439.35}}`))
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()

	latitude, longitude := 13.7, 100.5
	dataProvider := NewWorldPopProvider(WorldPopConfig{Endpoint: server.URL, Year: 2025, Resolution: "100m", CacheTTL: time.Hour, UserAgent: "rbc-test"}, server.Client(), cache.Noop{})
	observations, err := dataProvider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	byType := make(map[string]Observation, len(observations))
	for _, observation := range observations {
		byType[observation.MetricType] = observation
	}
	population := byType["population"]
	if population.Status != domain.DataEstimated || population.NormalizedScore != nil {
		t.Fatalf("population must remain an unscored estimate: %+v", population)
	}
	if population.Source.DatasetVersion != "worldpop_R2025A_2025_100m" || !strings.Contains(string(population.RawValue), `"dataYear":2025`) {
		t.Fatalf("population provenance was not preserved: %+v", population)
	}
}
