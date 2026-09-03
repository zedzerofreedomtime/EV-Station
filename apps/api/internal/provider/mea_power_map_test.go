package provider

import (
	"encoding/json"
	"testing"
)

func TestFindMEALayerAndFeature(t *testing.T) {
	html := `"Station Area (2026 - 115kV)" : feature_group_abc, geo_json_def.addTo(feature_group_abc)`
	layer, err := findMEALayerID(html, 2026, 115)
	if err != nil || layer != "geo_json_def" {
		t.Fatalf("unexpected layer: %q %v", layer, err)
	}
	payload := []byte(`var MAP_DATA = {"geo_json_def":{"features":[{"properties":{"LayerType":"Buffer","Available Cap.":120,"Sub":"ABC"},"geometry":{"type":"Polygon","coordinates":[[[100,13],[101,13],[101,14],[100,14],[100,13]]]}}]}};`)
	feature, err := findMEAFeature(payload, layer, 13.5, 100.5)
	if err != nil {
		t.Fatal(err)
	}
	if value, ok := numberProperty(feature, "Available Cap."); !ok || value != 120 {
		t.Fatalf("unexpected capacity: %v %v", value, ok)
	}
	if _, err = json.Marshal(feature); err != nil {
		t.Fatal(err)
	}
}

func TestFindMEAFeatureDoesNotInferOutsidePolygon(t *testing.T) {
	payload := []byte(`var MAP_DATA = {"layer":{"features":[{"properties":{"LayerType":"Buffer","Available Cap.":120},"geometry":{"type":"Polygon","coordinates":[[[100,13],[101,13],[101,14],[100,14],[100,13]]]}}]}};`)
	if _, err := findMEAFeature(payload, "layer", 15, 102); err == nil {
		t.Fatal("expected no match outside polygon")
	}
}

func TestFindNearestMEAFeatureKeepsPublishedAreaDistinctFromSiteMatch(t *testing.T) {
	payload := []byte(`var MAP_DATA = {"layer":{"features":[{"properties":{"LayerType":"Buffer","Available Cap.":120,"Sub":"NEAR"},"geometry":{"type":"Polygon","coordinates":[[[100,13],[101,13],[101,14],[100,14],[100,13]]]}}]}};`)
	feature, distance, err := findNearestMEAFeature(payload, "layer", 14.02, 100.5)
	if err != nil {
		t.Fatal(err)
	}
	if feature["Sub"] != "NEAR" || distance <= 0 || distance > 3000 {
		t.Fatalf("unexpected nearest MEA area: %#v distance=%v", feature, distance)
	}
}
