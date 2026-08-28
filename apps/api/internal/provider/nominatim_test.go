package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
)

func TestNominatimSearchPreservesProvenance(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") != "rbc-test" {
			t.Fatalf("unexpected user agent: %q", r.Header.Get("User-Agent"))
		}
		if r.URL.Query().Get("countrycodes") != "th" || r.URL.Query().Get("q") != "Bang Na" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"display_name":"Bang Na, Bangkok, Thailand","lat":"13.6682","lon":"100.6047","category":"place","type":"suburb"}]`))
	}))
	defer server.Close()

	geocoder := NewNominatimGeocoder(NominatimConfig{Endpoint: server.URL, UserAgent: "rbc-test", CountryCodes: "th", CacheTTL: time.Hour}, server.Client(), cache.Noop{})
	results, err := geocoder.Search(context.Background(), "Bang Na", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Latitude != 13.6682 || results[0].Longitude != 100.6047 {
		t.Fatalf("unexpected results: %#v", results)
	}
	if results[0].Status != "preliminary" || results[0].Source.Name != "OpenStreetMap Nominatim" || len(results[0].Assumptions) == 0 {
		t.Fatalf("provenance is incomplete: %#v", results[0])
	}
}

func TestNominatimRejectsShortQuery(t *testing.T) {
	t.Parallel()
	geocoder := NewNominatimGeocoder(NominatimConfig{}, nil, cache.Noop{})
	if _, err := geocoder.Search(context.Background(), "ab", 5); err != ErrInvalidGeocodingQuery {
		t.Fatalf("expected ErrInvalidGeocodingQuery, got %v", err)
	}
}
