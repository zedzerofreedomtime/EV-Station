package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const meaPowerMapReference = "https://measervice.mea.or.th/powermap/load/index.html"
const thailandPowerMapReference = "https://erc.or.th/th/thailand-power-map"
const maxMEANearestAreaDistanceMeters = 15000.0

type MEAPowerMapConfig struct {
	PageURL   string
	DataURL   string
	Year      int
	VoltageKV int
	CacheTTL  time.Duration
	UserAgent string
}

type MEAPowerMapProvider struct {
	config MEAPowerMapConfig
	client *http.Client
	cache  cache.Cache
}

type meaMapData map[string]struct {
	Features []struct {
		Properties map[string]any `json:"properties"`
		Geometry   struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

type meaPowerMapValue struct {
	AssessmentType       string  `json:"assessmentType"`
	MatchingMethod       string  `json:"matchingMethod"`
	DistanceToAreaMeters float64 `json:"distanceToAreaMeters"`
	MapYear              int     `json:"mapYear"`
	VoltageKV            int     `json:"voltageKv"`
	StationCode          string  `json:"stationCode,omitempty"`
	StationName          string  `json:"stationName,omitempty"`
	PublishedCapacityMW  float64 `json:"publishedCapacityMw"`
}

func NewMEAPowerMapProvider(config MEAPowerMapConfig, client *http.Client, externalCache cache.Cache) *MEAPowerMapProvider {
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.PageURL == "" {
		config.PageURL = meaPowerMapReference
	}
	if config.DataURL == "" {
		config.DataURL = "https://measervice.mea.or.th/powermap/load/powermap_data.js"
	}
	if config.Year == 0 {
		config.Year = time.Now().Year()
	}
	if config.VoltageKV == 0 {
		config.VoltageKV = 115
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &MEAPowerMapProvider{config: config, client: client, cache: externalCache}
}

func (p *MEAPowerMapProvider) Collect(ctx context.Context, site domain.Site, _ int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["electrical"]] = p.missing("Valid coordinates are required to query the published MEA Power Map area.")
		return observations, nil
	}

	html, err := p.fetch(ctx, "page", p.config.PageURL, 6<<20)
	if err != nil {
		observations[positions["electrical"]] = p.missing("The official MEA Power Map page was unavailable; no electrical preliminary assessment was produced.")
		return observations, nil
	}
	payload, err := p.fetch(ctx, "data", p.config.DataURL, 24<<20)
	if err != nil {
		observations[positions["electrical"]] = p.missing("The official MEA Power Map spatial dataset was unavailable; no electrical preliminary assessment was produced.")
		return observations, nil
	}
	selectedVoltage := p.config.VoltageKV
	var feature map[string]any
	matchedNearestArea := false
	var nearestFeature map[string]any
	nearestDistance := math.Inf(1)
	nearestVoltage := selectedVoltage
	for _, voltage := range preferredMEAVoltages(p.config.VoltageKV) {
		layerID, layerErr := findMEALayerID(string(html), p.config.Year, voltage)
		if layerErr != nil {
			continue
		}
		candidate, featureErr := findMEAFeature(payload, layerID, *site.Latitude, *site.Longitude)
		if featureErr == nil {
			feature = candidate
			selectedVoltage = voltage
			break
		}
		candidate, distance, nearestErr := findNearestMEAFeature(payload, layerID, *site.Latitude, *site.Longitude)
		if nearestErr == nil && distance < nearestDistance {
			nearestFeature = candidate
			nearestDistance = distance
			nearestVoltage = voltage
		}
	}
	if feature == nil {
		if nearestFeature == nil || nearestDistance > maxMEANearestAreaDistanceMeters {
			observations[positions["electrical"]] = p.missing("The coordinates did not intersect a published MEA station-area polygon or fall near a published MEA area. This may mean the site is outside the MEA service area or the layer has no coverage.")
			return observations, nil
		}
		feature = nearestFeature
		selectedVoltage = nearestVoltage
		matchedNearestArea = true
	}
	capacity, ok := numberProperty(feature, "Available Cap.")
	if !ok {
		observations[positions["electrical"]] = p.missing("The matching MEA polygon did not publish a numeric capacity value; no value was inferred.")
		return observations, nil
	}
	raw, _ := json.Marshal(meaPowerMapValue{
		AssessmentType:      "published_station_area_guideline",
		MatchingMethod:      "point_in_published_area",
		MapYear:             p.config.Year,
		VoltageKV:           selectedVoltage,
		StationCode:         stringProperty(feature, "Sub_Station", "Sub"),
		StationName:         stringProperty(feature, "Name_TH", "Name_EN"),
		PublishedCapacityMW: capacity,
	})
	if matchedNearestArea {
		raw, _ = json.Marshal(meaPowerMapValue{
			AssessmentType: "published_station_area_guideline", MatchingMethod: "nearest_published_area",
			DistanceToAreaMeters: nearestDistance, MapYear: p.config.Year, VoltageKV: selectedVoltage,
			StationCode: stringProperty(feature, "Sub_Station", "Sub"), StationName: stringProperty(feature, "Name_TH", "Name_EN"), PublishedCapacityMW: capacity,
		})
	}
	methodology := "Point-in-polygon lookup against the station-area layer published by MEA for the selected year and voltage."
	assumptions := []string{
		"This is a preliminary planning indicator from the published MEA Power Map, not verified remaining grid capacity for the submitted plot.",
		"MEA states that map values may change and do not confirm supply capability, connection feasibility, construction, reinforcement, or network-extension requirements.",
		"A formal MEA technical review and connection request are required before an investment decision.",
		"This public planning value is excluded from scoring until utility-confirmed site capacity is available.",
	}
	if matchedNearestArea {
		methodology = "Nearest published MEA station-area boundary lookup; the site does not intersect that area. Distance is calculated from the submitted point to the published polygon boundary."
		assumptions = append(assumptions, "The displayed capacity belongs to the nearest published MEA station area, not to the submitted plot. It must not be treated as the plot's available capacity.")
	}
	observations[positions["electrical"]] = Observation{
		MetricType: "electrical", RawValue: raw, Status: domain.DataPreliminary,
		Source: domain.DataSource{
			Name: "MEA Power Map for electricity users", Type: "official_public_power_map", Authority: "official", GeographicScope: "published_station_area", SiteVerification: "preliminary_map_lookup",
			ReferenceURI:   meaPowerMapReference,
			DatasetVersion: fmt.Sprintf("Load map %d, %d kV", p.config.Year, selectedVoltage),
			RetrievedAt:    time.Now().UTC(),
			Methodology:    methodology,
		},
		Assumptions: assumptions,
	}
	return observations, nil
}

func preferredMEAVoltages(configured int) []int {
	if configured == 69 {
		return []int{69, 115}
	}
	return []int{configured, 69}
}

func (p *MEAPowerMapProvider) fetch(ctx context.Context, kind, endpoint string, limit int64) ([]byte, error) {
	hash := sha256.Sum256([]byte(endpoint))
	key := "mea:power-map:" + kind + ":" + hex.EncodeToString(hash[:])
	if payload, found, err := p.cache.Get(ctx, key); err == nil && found {
		return payload, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/html,application/javascript,application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("MEA Power Map returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, limit))
	if err != nil {
		return nil, err
	}
	_ = p.cache.Set(ctx, key, payload, p.config.CacheTTL)
	return payload, nil
}

func findMEALayerID(html string, year, voltageKV int) (string, error) {
	label := fmt.Sprintf("Station Area (%d - %dkV)", year, voltageKV)
	groupPattern := regexp.MustCompile(regexp.QuoteMeta(`"`+label+`"`) + `\s*:\s*(feature_group_[a-f0-9]+)`)
	groupMatch := groupPattern.FindStringSubmatch(html)
	if len(groupMatch) != 2 {
		return "", fmt.Errorf("MEA layer group not found")
	}
	layerPattern := regexp.MustCompile(`(geo_json_[a-f0-9]+)\.addTo\(` + regexp.QuoteMeta(groupMatch[1]) + `\)`)
	layerMatch := layerPattern.FindStringSubmatch(html)
	if len(layerMatch) != 2 {
		return "", fmt.Errorf("MEA data layer not found")
	}
	return layerMatch[1], nil
}

func findMEAFeature(payload []byte, layerID string, latitude, longitude float64) (map[string]any, error) {
	text := strings.TrimSpace(string(payload))
	text = strings.TrimPrefix(text, "var MAP_DATA =")
	text = strings.TrimSuffix(strings.TrimSpace(text), ";")
	var data meaMapData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, fmt.Errorf("decode MEA Power Map: %w", err)
	}
	collection, found := data[layerID]
	if !found {
		return nil, fmt.Errorf("MEA layer data missing")
	}
	for _, feature := range collection.Features {
		if stringProperty(feature.Properties, "LayerType") != "Buffer" {
			continue
		}
		if geometryContainsPoint(feature.Geometry.Type, feature.Geometry.Coordinates, longitude, latitude) {
			return feature.Properties, nil
		}
	}
	return nil, fmt.Errorf("no MEA area contains point")
}

func findNearestMEAFeature(payload []byte, layerID string, latitude, longitude float64) (map[string]any, float64, error) {
	text := strings.TrimSpace(string(payload))
	text = strings.TrimPrefix(text, "var MAP_DATA =")
	text = strings.TrimSuffix(strings.TrimSpace(text), ";")
	var data meaMapData
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return nil, 0, fmt.Errorf("decode MEA Power Map: %w", err)
	}
	collection, found := data[layerID]
	if !found {
		return nil, 0, fmt.Errorf("MEA layer data missing")
	}
	var nearest map[string]any
	nearestDistance := math.Inf(1)
	for _, feature := range collection.Features {
		if stringProperty(feature.Properties, "LayerType") != "Buffer" {
			continue
		}
		distance := geometryDistanceMeters(feature.Geometry.Type, feature.Geometry.Coordinates, longitude, latitude)
		if distance < nearestDistance {
			nearest, nearestDistance = feature.Properties, distance
		}
	}
	if nearest == nil || math.IsInf(nearestDistance, 1) {
		return nil, 0, fmt.Errorf("no MEA areas available")
	}
	return nearest, nearestDistance, nil
}

func geometryContainsPoint(geometryType string, raw json.RawMessage, longitude, latitude float64) bool {
	switch geometryType {
	case "Polygon":
		var polygon [][][]float64
		return json.Unmarshal(raw, &polygon) == nil && polygonContainsPoint(polygon, longitude, latitude)
	case "MultiPolygon":
		var polygons [][][][]float64
		if json.Unmarshal(raw, &polygons) != nil {
			return false
		}
		for _, polygon := range polygons {
			if polygonContainsPoint(polygon, longitude, latitude) {
				return true
			}
		}
	}
	return false
}

func polygonContainsPoint(polygon [][][]float64, longitude, latitude float64) bool {
	if len(polygon) == 0 || !ringContainsPoint(polygon[0], longitude, latitude) {
		return false
	}
	for _, hole := range polygon[1:] {
		if ringContainsPoint(hole, longitude, latitude) {
			return false
		}
	}
	return true
}

func geometryDistanceMeters(geometryType string, raw json.RawMessage, longitude, latitude float64) float64 {
	switch geometryType {
	case "Polygon":
		var polygon [][][]float64
		if json.Unmarshal(raw, &polygon) == nil {
			return polygonDistanceMeters(polygon, longitude, latitude)
		}
	case "MultiPolygon":
		var polygons [][][][]float64
		if json.Unmarshal(raw, &polygons) == nil {
			nearest := math.Inf(1)
			for _, polygon := range polygons {
				nearest = math.Min(nearest, polygonDistanceMeters(polygon, longitude, latitude))
			}
			return nearest
		}
	}
	return math.Inf(1)
}

func polygonDistanceMeters(polygon [][][]float64, longitude, latitude float64) float64 {
	if len(polygon) == 0 {
		return math.Inf(1)
	}
	if polygonContainsPoint(polygon, longitude, latitude) {
		return 0
	}
	nearest := math.Inf(1)
	for _, ring := range polygon {
		for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
			if len(ring[i]) < 2 || len(ring[j]) < 2 {
				continue
			}
			nearest = math.Min(nearest, pointToSegmentMeters(latitude, longitude, ring[j][1], ring[j][0], ring[i][1], ring[i][0]))
		}
	}
	return nearest
}

func ringContainsPoint(ring [][]float64, longitude, latitude float64) bool {
	inside := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		if len(ring[i]) < 2 || len(ring[j]) < 2 {
			continue
		}
		xi, yi, xj, yj := ring[i][0], ring[i][1], ring[j][0], ring[j][1]
		intersects := (yi > latitude) != (yj > latitude) && longitude < (xj-xi)*(latitude-yi)/(yj-yi)+xi
		if intersects {
			inside = !inside
		}
	}
	return inside
}

func numberProperty(properties map[string]any, key string) (float64, bool) {
	value, exists := properties[key]
	if !exists {
		return 0, false
	}
	switch typed := value.(type) {
	case float64:
		return typed, true
	case string:
		number, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(typed), ",", ""), 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func stringProperty(properties map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := properties[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (p *MEAPowerMapProvider) missing(assumption string) Observation {
	return Observation{MetricType: "electrical", Status: domain.DataMissing, Source: domain.DataSource{
		Name: "Thailand Power Map / MEA Power Map", Type: "official_public_power_map", Authority: "official", GeographicScope: "published_station_area",
		ReferenceURI: thailandPowerMapReference, RetrievedAt: time.Now().UTC(),
	}, Assumptions: []string{assumption, "No MEA or PEA supply capability has been verified for the submitted plot."}}
}
