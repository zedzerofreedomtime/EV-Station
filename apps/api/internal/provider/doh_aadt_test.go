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

func TestDOHAADTProviderMatchesExactControlSection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/aadt.csv" {
			_, _ = writer.Write([]byte("ทางหลวงสาย,ตอนควบคุม,ชื่อสายทาง,จุดสำรวจ,a,b,c,d,e,f,g,h,i,j,รวม\n1,102,test,25+556,1,1,1,1,1,1,1,1,1,1,47413\n"))
			return
		}
		if request.URL.Path == "/roads" {
			_, _ = writer.Write([]byte(`{"features":[{"attributes":{"road_code":"0001","section_co":"0102","section_na":"test section","km_start":0,"km_end":1000},"geometry":{"paths":[[[100.5000,13.7000],[100.5100,13.7000]]]}}]}`))
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()
	latitude, longitude := 13.7005, 100.505
	provider := NewDOHAADTProvider(DOHAADTConfig{CSVURL: server.URL + "/aadt.csv", RoadLayerURL: server.URL + "/roads", DataYear: 2568, CacheTTL: time.Hour}, server.Client(), cache.Noop{})
	observations, err := provider.Collect(context.Background(), domain.Site{Latitude: &latitude, Longitude: &longitude}, 3000)
	if err != nil {
		t.Fatal(err)
	}
	for _, observation := range observations {
		if observation.MetricType == "traffic" {
			if observation.Status != domain.DataVerified || observation.NormalizedScore != nil || !strings.Contains(string(observation.RawValue), `"aadt":47413`) || !strings.Contains(string(observation.RawValue), `"controlSection":"0102"`) {
				t.Fatalf("unexpected traffic result: %+v", observation)
			}
			return
		}
	}
	t.Fatal("traffic observation missing")
}

func TestParseDOHAADTRejectsMalformedData(t *testing.T) {
	if _, err := parseDOHAADT([]byte("only,header\n")); err == nil {
		t.Fatal("expected invalid CSV error")
	}
}
