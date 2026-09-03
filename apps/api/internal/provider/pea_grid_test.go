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

func TestPEAGridProviderReturnsPublishedGridEvidenceWithoutCapacityClaim(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/stations" {
			_, _ = writer.Write([]byte(`{"features":[{"attributes":{"NAME_THAI":"สถานีตัวอย่าง","SECONDARYVOLTAGE":22},"geometry":{"x":100.01,"y":14.01}}]}`))
			return
		}
		_, _ = writer.Write([]byte(`{"features":[{"attributes":{"OP_VOLT":"H1","CONDUCTORSIZE":400,"PHASEDESIGNATION":7,"FEEDERID":"F-1"},"geometry":{"paths":[[[100.0,14.0],[100.02,14.02]]]}}]}`))
	}))
	defer server.Close()
	latitude, longitude := 14.0, 100.0
	provider := NewPEAGridProvider(PEAGridConfig{StationURL: server.URL + "/stations", ConductorURL: server.URL + "/lines", SearchRadiusMeters: 5000, CacheTTL: time.Minute}, server.Client(), cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	electrical := findObservation(t, observations, "electrical")
	if electrical.Status != domain.DataPreliminary {
		t.Fatalf("unexpected status: %s", electrical.Status)
	}
	var value peaGridValue
	if err := json.Unmarshal(electrical.RawValue, &value); err != nil {
		t.Fatal(err)
	}
	if value.HighVoltageLineCount != 1 || value.ConductorSize != "400" || value.PhaseCode != "7" || value.StationSecondaryVoltageKV == nil || *value.StationSecondaryVoltageKV != 22 {
		t.Fatalf("unexpected grid evidence: %+v", value)
	}
}
