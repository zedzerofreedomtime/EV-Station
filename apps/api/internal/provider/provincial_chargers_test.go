package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestProvincialChargerProviderFiltersAndDeduplicatesStations(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/suphan.csv", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/csv")
		_, _ = writer.Write([]byte("ชื่อสถานี,ประเภทสถานี,ตำแหน่งที่ตั้ง\nStation A,PEA VOLTA,https://maps.google.com/?destination=14.500000%2C100.100000\nStation A,PEA VOLTA,https://maps.google.com/?destination=14.500000%2C100.100000\nFar station,Other,https://maps.google.com/?destination=15.500000%2C101.100000\n"))
	})
	mux.HandleFunc("/saraburi.json", func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"metadata":{"title":"Saraburi","license":"OGDL","last_updated":"2026-08-31T09:44:03.766Z"},"data":[{"id":"1","name":"Station B","latitude":14.501,"longitude":100.101,"energy_types":["EV"],"has_ev_charger":true,"brand":{"name":"Brand B"}},{"id":"2","name":"Fuel only","latitude":14.501,"longitude":100.101,"energy_types":["OIL"],"has_ev_charger":false}]}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	latitude, longitude := 14.5, 100.1
	dataProvider := NewProvincialChargerProvider(ProvincialChargerConfig{SuphanburiCSVURL: server.URL + "/suphan.csv", SaraburiJSONURL: server.URL + "/saraburi.json", CacheTTL: time.Minute}, server.Client(), cache.Noop{})
	observations, err := dataProvider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	competition := findObservation(t, observations, "competition")
	if competition.Status != domain.DataVerified || competition.NormalizedScore != nil {
		t.Fatalf("unexpected competition observation: %+v", competition)
	}
	var value competitionMetricValue
	if err = json.Unmarshal(competition.RawValue, &value); err != nil {
		t.Fatal(err)
	}
	if value.Count != 2 || len(value.Places) != 2 {
		t.Fatalf("expected two nearby deduplicated stations, got count=%d places=%d: %s", value.Count, len(value.Places), competition.RawValue)
	}
	if len(value.Sources) != 2 || value.Sources[0].Count+value.Sources[1].Count != 2 {
		t.Fatalf("expected two source breakdowns with two nearby records, got %+v", value.Sources)
	}
}

func TestMergeCompetitionObservationsPreservesSourcesAndDeduplicates(t *testing.T) {
	now := time.Now().UTC()
	osmRaw := json.RawMessage(`{"count":1,"radiusMeters":3000,"places":[{"osmType":"node","osmId":99,"name":"Station A","category":"amenity:charging_station","latitude":14.5,"longitude":100.1}]}`)
	provincialRaw := json.RawMessage(`{"count":1,"radiusMeters":3000,"places":[{"recordType":"provincial_csv","recordId":"a","name":"Station A","category":"charging_station","latitude":14.50001,"longitude":100.10001,"sourceNames":["Provincial source"],"sourceRecordCount":1}],"sources":[{"name":"Provincial source","referenceUri":"https://example.test/data","coverage":"one province","count":1,"retrievedAt":"2026-08-31T00:00:00Z"}]}`)
	merged := mergeCompetitionObservations([]Observation{
		{MetricType: "competition", RawValue: osmRaw, Status: domain.DataVerified, Source: domain.DataSource{Name: "OSM", ReferenceURI: "https://openstreetmap.org", RetrievedAt: now, License: "ODbL"}},
		{MetricType: "competition", RawValue: provincialRaw, Status: domain.DataVerified, Source: domain.DataSource{Name: "Provincial", RetrievedAt: now}},
	}, 3000)
	var value competitionMetricValue
	if err := json.Unmarshal(merged.RawValue, &value); err != nil {
		t.Fatal(err)
	}
	if value.Count != 1 || len(value.Sources) != 2 {
		t.Fatalf("expected one cross-source station and two preserved sources, got %s", merged.RawValue)
	}
	if len(value.Places) != 1 || len(value.Places[0].SourceNames) != 2 {
		t.Fatalf("expected merged place provenance, got %+v", value.Places)
	}
}

func findObservation(t *testing.T, observations []Observation, metricType string) Observation {
	t.Helper()
	for _, observation := range observations {
		if observation.MetricType == metricType {
			return observation
		}
	}
	t.Fatalf("observation %s not found", metricType)
	return Observation{}
}
