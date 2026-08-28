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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type OSMConfig struct {
	Endpoint  string
	UserAgent string
	CacheTTL  time.Duration
}

type OSMProvider struct {
	config OSMConfig
	client *http.Client
	cache  cache.Cache
}

type osmResponse struct {
	Version   float64      `json:"version"`
	Generator string       `json:"generator"`
	Elements  []osmElement `json:"elements"`
}

type osmElement struct {
	Type     string            `json:"type"`
	ID       int64             `json:"id"`
	Lat      float64           `json:"lat,omitempty"`
	Lon      float64           `json:"lon,omitempty"`
	Center   *osmCenter        `json:"center,omitempty"`
	Geometry []osmCenter       `json:"geometry,omitempty"`
	Tags     map[string]string `json:"tags"`
}

type osmCenter struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type osmPlace struct {
	OSMType   string  `json:"osmType"`
	OSMID     int64   `json:"osmId"`
	Name      string  `json:"name,omitempty"`
	Category  string  `json:"category"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type osmMetricValue struct {
	Count        int        `json:"count"`
	RadiusMeters int        `json:"radiusMeters"`
	Places       []osmPlace `json:"places"`
}

type osmRoadMetricValue struct {
	RadiusMeters           int            `json:"radiusMeters"`
	MappedMajorRoadCount   int            `json:"mappedMajorRoadCount"`
	NearestMajorRoadMeters *float64       `json:"nearestMajorRoadMeters,omitempty"`
	RoadClassCounts        map[string]int `json:"roadClassCounts"`
}

func NewOSMProvider(config OSMConfig, client *http.Client, externalCache cache.Cache) *OSMProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 15 * time.Minute
	}
	return &OSMProvider{config: config, client: client, cache: externalCache}
}

func (p *OSMProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		setMissingOSMObservation(observations, positions, "traffic", "Valid coordinates are required to query OpenStreetMap road-accessibility data.")
		setMissingOSMObservation(observations, positions, "poi", "Valid coordinates are required to query OpenStreetMap POI data.")
		setMissingOSMObservation(observations, positions, "competition", "Valid coordinates are required to query OpenStreetMap charging stations.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}

	query := buildOverpassQuery(*site.Latitude, *site.Longitude, radius)
	payload, err := p.fetch(ctx, query)
	if err != nil {
		setMissingOSMObservation(observations, positions, "poi", "OpenStreetMap provider was unavailable; no POI value or score was produced.")
		setMissingOSMObservation(observations, positions, "competition", "OpenStreetMap provider was unavailable; no competitor value or score was produced.")
		return observations, nil
	}

	var response osmResponse
	if err = json.Unmarshal(payload, &response); err != nil {
		setMissingOSMObservation(observations, positions, "poi", "OpenStreetMap returned an unreadable response; no POI value or score was produced.")
		setMissingOSMObservation(observations, positions, "competition", "OpenStreetMap returned an unreadable response; no competitor value or score was produced.")
		return observations, nil
	}

	poi, chargers := classifyOSMElements(response.Elements)
	roads := classifyOSMRoads(response.Elements)
	retrievedAt := time.Now().UTC()
	source := domain.DataSource{
		Name:         "OpenStreetMap via Overpass API",
		Type:         "open_data_api",
		ReferenceURI: "https://www.openstreetmap.org/copyright",
		RetrievedAt:  retrievedAt,
		Methodology:  "Count of currently returned OpenStreetMap elements within the requested radius, deduplicated by OSM type and ID.",
		License:      "Open Data Commons Open Database License (ODbL) 1.0",
	}

	observations[positions["poi"]] = osmObservation("poi", poi, radius, source, []string{
		"Coverage depends on voluntary OpenStreetMap contributions and may be incomplete.",
		"This is a factual count of returned tagged elements, not a normalized suitability score.",
	})
	observations[positions["competition"]] = osmObservation("competition", chargers, radius, source, []string{
		"Only charging stations mapped in OpenStreetMap are included; unmapped operators may be missing.",
		"Connector availability, power, pricing, and operational status are not verified by this query.",
		"No competition score is produced until a deterministic scoring rule is approved.",
	})
	roadSource := source
	roadSource.Methodology = "Count mapped motorway, trunk, primary, secondary and tertiary ways within the radius and approximate the nearest distance from returned way geometry vertices."
	observations[positions["traffic"]] = osmRoadObservation(roads, *site.Latitude, *site.Longitude, radius, roadSource)
	return observations, nil
}

func (p *OSMProvider) fetch(ctx context.Context, query string) ([]byte, error) {
	hash := sha256.Sum256([]byte(query))
	cacheKey := "osm:overpass:" + hex.EncodeToString(hash[:])
	if value, found, err := p.cache.Get(ctx, cacheKey); err == nil && found {
		return value, nil
	}

	form := url.Values{"data": {query}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)

	response, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("overpass returned status %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 10<<20))
	if err != nil {
		return nil, err
	}
	_ = p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL)
	return payload, nil
}

func buildOverpassQuery(latitude, longitude float64, radius int) string {
	lat := strconv.FormatFloat(latitude, 'f', 6, 64)
	lon := strconv.FormatFloat(longitude, 'f', 6, 64)
	r := strconv.Itoa(radius)
	return `[out:json][timeout:20];(` +
		`nwr(around:` + r + `,` + lat + `,` + lon + `)["amenity"="charging_station"];` +
		`nwr(around:` + r + `,` + lat + `,` + lon + `)["amenity"~"^(restaurant|cafe|fast_food|hospital|clinic|university|college|school|marketplace)$"];` +
		`nwr(around:` + r + `,` + lat + `,` + lon + `)["shop"];` +
		`nwr(around:` + r + `,` + lat + `,` + lon + `)["tourism"];` +
		`nwr(around:` + r + `,` + lat + `,` + lon + `)["leisure"];` +
		`);out center tags;` +
		`way(around:` + r + `,` + lat + `,` + lon + `)["highway"~"^(motorway|trunk|primary|secondary|tertiary)$"];out geom tags;`
}

func classifyOSMElements(elements []osmElement) (poi []osmPlace, chargers []osmPlace) {
	seen := make(map[string]struct{}, len(elements))
	for _, element := range elements {
		if element.Tags["highway"] != "" {
			continue
		}
		key := element.Type + ":" + strconv.FormatInt(element.ID, 10)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		category := osmCategory(element.Tags)
		place := osmPlace{OSMType: element.Type, OSMID: element.ID, Name: element.Tags["name"], Category: category, Latitude: element.Lat, Longitude: element.Lon}
		if element.Center != nil {
			place.Latitude, place.Longitude = element.Center.Lat, element.Center.Lon
		}
		if element.Tags["amenity"] == "charging_station" {
			chargers = append(chargers, place)
			continue
		}
		poi = append(poi, place)
	}
	return poi, chargers
}

func classifyOSMRoads(elements []osmElement) []osmElement {
	roads := make([]osmElement, 0)
	seen := make(map[int64]struct{})
	for _, element := range elements {
		if element.Type != "way" || element.Tags["highway"] == "" {
			continue
		}
		if _, exists := seen[element.ID]; exists {
			continue
		}
		seen[element.ID] = struct{}{}
		roads = append(roads, element)
	}
	return roads
}

func osmRoadObservation(roads []osmElement, latitude, longitude float64, radius int, source domain.DataSource) Observation {
	classCounts := make(map[string]int)
	var nearest *float64
	for _, road := range roads {
		classCounts[road.Tags["highway"]]++
		for _, point := range road.Geometry {
			distance := haversineMeters(latitude, longitude, point.Lat, point.Lon)
			if nearest == nil || distance < *nearest {
				value := distance
				nearest = &value
			}
		}
	}
	raw, _ := json.Marshal(osmRoadMetricValue{RadiusMeters: radius, MappedMajorRoadCount: len(roads), NearestMajorRoadMeters: nearest, RoadClassCounts: classCounts})
	return Observation{
		MetricType: "traffic", RawValue: raw, Status: domain.DataPreliminary, Source: source,
		Assumptions: []string{
			"This is a road accessibility proxy from mapped road classes, not a traffic count, speed, congestion, or AADT measurement.",
			"Nearest-road distance is approximated from the returned OpenStreetMap way geometry vertices.",
			"OpenStreetMap road classification and coverage may be incomplete or differ from official Thai classifications.",
			"No normalized traffic score is produced in this MVP.",
		},
	}
}

func haversineMeters(latitude1, longitude1, latitude2, longitude2 float64) float64 {
	const earthRadius = 6371000.0
	lat1 := latitude1 * math.Pi / 180
	lat2 := latitude2 * math.Pi / 180
	deltaLat := (latitude2 - latitude1) * math.Pi / 180
	deltaLon := (longitude2 - longitude1) * math.Pi / 180
	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	return earthRadius * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func osmCategory(tags map[string]string) string {
	for _, key := range []string{"amenity", "shop", "tourism", "leisure"} {
		if value := tags[key]; value != "" {
			return key + ":" + value
		}
	}
	return "other"
}

func osmObservation(metricType string, places []osmPlace, radius int, source domain.DataSource, assumptions []string) Observation {
	count := len(places)
	if len(places) > 100 {
		places = places[:100]
		assumptions = append(assumptions, "The response preserves at most 100 example places; count remains the full deduplicated total.")
	}
	raw, _ := json.Marshal(osmMetricValue{Count: count, RadiusMeters: radius, Places: places})
	return Observation{MetricType: metricType, RawValue: raw, Status: domain.DataVerified, Source: source, Assumptions: assumptions}
}

func unavailableObservations() ([]Observation, map[string]int) {
	base, _ := (UnavailableProvider{}).Collect(context.Background(), domain.Site{}, 0)
	positions := make(map[string]int, len(base))
	for index := range base {
		positions[base[index].MetricType] = index
	}
	return base, positions
}

func setMissingOSMObservation(observations []Observation, positions map[string]int, metricType, assumption string) {
	observations[positions[metricType]] = Observation{
		MetricType: metricType,
		Status:     domain.DataMissing,
		Source: domain.DataSource{
			Name:        "OpenStreetMap via Overpass API",
			Type:        "open_data_api",
			RetrievedAt: time.Now().UTC(),
			License:     "Open Data Commons Open Database License (ODbL) 1.0",
		},
		Assumptions: []string{assumption},
	}
}
