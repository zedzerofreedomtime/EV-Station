package provider

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
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

const dohAADTReference = "https://datagov.mot.go.th/th/dataset/traf62"
const dohRoadLayerReference = "https://giportal.mot.go.th/portal/home/item.html?id=7dd9dca0bd7948de9351b29302cd4c2c"

// DOHAADTConfig combines the annual AADT tabular release with the official DOH
// control-section geometry layer. A value is emitted only when road number and
// control section match in both sources and the mapped section is near the site.
type DOHAADTConfig struct {
	CSVURL       string
	RoadLayerURL string
	DataYear     int
	CacheTTL     time.Duration
	UserAgent    string
}

type DOHAADTProvider struct {
	config DOHAADTConfig
	client *http.Client
	cache  cache.Cache
}

type dohAADTRecord struct {
	RoadCode       string
	ControlSection string
	SurveyKM       string
	TotalAADT      int
}

type dohRoadFeatureResponse struct {
	Features []dohRoadFeature `json:"features"`
}

type dohRoadFeature struct {
	Attributes struct {
		RoadCode       string `json:"road_code"`
		ControlSection string `json:"section_co"`
		SectionName    string `json:"section_na"`
		KMStart        int    `json:"km_start"`
		KMEnd          int    `json:"km_end"`
	} `json:"attributes"`
	Geometry struct {
		Paths [][][]float64 `json:"paths"`
	} `json:"geometry"`
}

type dohAADTMetricValue struct {
	AADT               int     `json:"aadt"`
	DataYear           int     `json:"dataYear"`
	RoadCode           string  `json:"roadCode"`
	ControlSection     string  `json:"controlSection"`
	ControlSectionName string  `json:"controlSectionName"`
	SurveyKM           string  `json:"surveyKm"`
	DistanceToSectionM float64 `json:"distanceToSectionMeters"`
	MatchRadiusMeters  int     `json:"matchRadiusMeters"`
}

func NewDOHAADTProvider(config DOHAADTConfig, client *http.Client, externalCache cache.Cache) *DOHAADTProvider {
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.CSVURL == "" {
		config.CSVURL = "https://opendata.doh.go.th/dataset/ed101df4-f0a1-4d7f-b9d9-76859b2ca73e/resource/f88f9c4e-b32e-4cd1-bf51-4e1f84ba7b16/download/aadt-68.csv"
	}
	if config.RoadLayerURL == "" {
		config.RoadLayerURL = "https://giportal.mot.go.th/arcgis/rest/services/Hosted/%E0%B9%80%E0%B8%AA%E0%B9%89%E0%B8%99%E0%B8%97%E0%B8%B2%E0%B8%87%E0%B8%AB%E0%B8%A5%E0%B8%A7%E0%B8%87%E0%B9%81%E0%B8%9C%E0%B9%88%E0%B8%99%E0%B8%94%E0%B8%B4%E0%B8%99_%E0%B8%97%E0%B8%A5/FeatureServer/0/query"
	}
	if config.DataYear == 0 {
		config.DataYear = 2568
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &DOHAADTProvider{config: config, client: client, cache: externalCache}
}

func (p *DOHAADTProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["traffic"]] = p.missing("Valid coordinates are required to match an official DOH AADT control section.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}
	records, err := p.fetchAADT(ctx)
	if err != nil {
		observations[positions["traffic"]] = p.missing("The official DOH AADT dataset could not be retrieved; no traffic value was produced.")
		return observations, nil
	}
	roads, err := p.fetchNearbyRoads(ctx, *site.Latitude, *site.Longitude, radius)
	if err != nil {
		observations[positions["traffic"]] = p.missing("The official DOH road control-section layer could not be queried; no AADT value was produced.")
		return observations, nil
	}

	var matched *dohAADTMetricValue
	for _, road := range roads {
		record, found := records[dohKey(road.Attributes.RoadCode, road.Attributes.ControlSection)]
		if !found {
			continue
		}
		distance := distanceToRoadMeters(*site.Latitude, *site.Longitude, road.Geometry.Paths)
		if math.IsInf(distance, 1) || distance > float64(radius) {
			continue
		}
		candidate := dohAADTMetricValue{AADT: record.TotalAADT, DataYear: p.config.DataYear, RoadCode: road.Attributes.RoadCode, ControlSection: road.Attributes.ControlSection, ControlSectionName: road.Attributes.SectionName, SurveyKM: record.SurveyKM, DistanceToSectionM: distance, MatchRadiusMeters: radius}
		if matched == nil || candidate.DistanceToSectionM < matched.DistanceToSectionM {
			matched = &candidate
		}
	}
	if matched == nil {
		observations[positions["traffic"]] = p.missing("No nearby official road geometry could be matched to an AADT road number and control section; the system did not infer a traffic count.")
		return observations, nil
	}
	raw, _ := json.Marshal(matched)
	observations[positions["traffic"]] = Observation{MetricType: "traffic", RawValue: raw, Status: domain.DataVerified, Source: domain.DataSource{
		Name: "Department of Highways AADT 2568 + road control sections", Type: "official_open_data_and_gis", ReferenceURI: dohAADTReference, DatasetVersion: fmt.Sprintf("AADT %d; DOH road geometry published Dec 2022", p.config.DataYear), RetrievedAt: time.Now().UTC(), Methodology: "Nearest official road geometry within the requested radius, matched exactly to the annual AADT dataset by road number and control section. Geometry reference: " + dohRoadLayerReference,
	}, Assumptions: []string{
		"AADT is the Department of Highways annual average daily traffic measurement for the matched road control section, not a live count at the site entrance.",
		"The current public road-geometry layer is described by its publisher as a December 2022 snapshot; its route/control-section identifiers must match the annual AADT release exactly.",
		"Only the closest exact road-number and control-section match within the requested radius is shown; unmatched roads produce no AADT value.",
		"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
	}}
	return observations, nil
}

func (p *DOHAADTProvider) fetchAADT(ctx context.Context) (map[string]dohAADTRecord, error) {
	key := "doh:aadt:" + hashText(p.config.CSVURL)
	if value, found, err := p.cache.Get(ctx, key); err == nil && found {
		return parseDOHAADT(value)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.config.CSVURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", p.config.UserAgent)
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("DOH AADT returned status %d", res.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if _, err = parseDOHAADT(payload); err != nil {
		return nil, err
	}
	_ = p.cache.Set(ctx, key, payload, p.config.CacheTTL)
	return parseDOHAADT(payload)
}

func (p *DOHAADTProvider) fetchNearbyRoads(ctx context.Context, latitude, longitude float64, radius int) ([]dohRoadFeature, error) {
	params := url.Values{"f": {"json"}, "where": {"1=1"}, "geometry": {fmt.Sprintf("%.7f,%.7f", longitude, latitude)}, "geometryType": {"esriGeometryPoint"}, "inSR": {"4326"}, "spatialRel": {"esriSpatialRelIntersects"}, "distance": {strconv.Itoa(radius)}, "units": {"esriSRUnit_Meter"}, "outFields": {"road_code,section_co,section_na,km_start,km_end"}, "returnGeometry": {"true"}, "outSR": {"4326"}}
	requestURL := strings.TrimRight(p.config.RoadLayerURL, "?") + "?" + params.Encode()
	key := "doh:roads:" + hashText(requestURL)
	if value, found, err := p.cache.Get(ctx, key); err == nil && found {
		var result dohRoadFeatureResponse
		if json.Unmarshal(value, &result) == nil {
			return result.Features, nil
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.config.UserAgent)
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("DOH road layer returned status %d", res.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	var result dohRoadFeatureResponse
	if err = json.Unmarshal(payload, &result); err != nil {
		return nil, err
	}
	_ = p.cache.Set(ctx, key, payload, p.config.CacheTTL)
	return result.Features, nil
}

func parseDOHAADT(payload []byte) (map[string]dohAADTRecord, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(payload), "\ufeff")))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("invalid DOH AADT CSV")
	}
	result := make(map[string]dohAADTRecord, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 15 {
			continue
		}
		road, section := normalizeDOHCode(row[0], 4), normalizeDOHCode(row[1], 4)
		total, err := parseDOHNumber(row[14])
		if err != nil || road == "" || section == "" {
			continue
		}
		result[dohKey(road, section)] = dohAADTRecord{RoadCode: road, ControlSection: section, SurveyKM: strings.TrimSpace(row[3]), TotalAADT: total}
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("DOH AADT CSV contains no usable records")
	}
	return result, nil
}

func normalizeDOHCode(value string, width int) string {
	value = strings.TrimSpace(value)
	number, err := strconv.Atoi(value)
	if err != nil {
		return strings.TrimLeft(value, "0")
	}
	return fmt.Sprintf("%0*d", width, number)
}
func dohKey(road, section string) string {
	return normalizeDOHCode(road, 4) + ":" + normalizeDOHCode(section, 4)
}
func parseDOHNumber(value string) (int, error) {
	return strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(value), ",", ""))
}
func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func distanceToRoadMeters(latitude, longitude float64, paths [][][]float64) float64 {
	minimum := math.Inf(1)
	for _, path := range paths {
		for index := 1; index < len(path); index++ {
			if len(path[index-1]) < 2 || len(path[index]) < 2 {
				continue
			}
			distance := pointToSegmentMeters(latitude, longitude, path[index-1][1], path[index-1][0], path[index][1], path[index][0])
			if distance < minimum {
				minimum = distance
			}
		}
	}
	return minimum
}

func pointToSegmentMeters(latitude, longitude, latitude1, longitude1, latitude2, longitude2 float64) float64 {
	const metersPerDegree = 111320.0
	cosLatitude := math.Cos(latitude * math.Pi / 180)
	x, y := 0.0, 0.0
	x1, y1 := (longitude1-longitude)*metersPerDegree*cosLatitude, (latitude1-latitude)*metersPerDegree
	x2, y2 := (longitude2-longitude)*metersPerDegree*cosLatitude, (latitude2-latitude)*metersPerDegree
	dx, dy := x2-x1, y2-y1
	denominator := dx*dx + dy*dy
	if denominator == 0 {
		return math.Hypot(x1-x, y1-y)
	}
	t := math.Max(0, math.Min(1, -((x1-x)*dx+(y1-y)*dy)/denominator))
	return math.Hypot(x1+t*dx-x, y1+t*dy-y)
}

func (p *DOHAADTProvider) missing(assumption string) Observation {
	return Observation{MetricType: "traffic", Status: domain.DataMissing, Source: domain.DataSource{Name: "Department of Highways AADT and road control sections", Type: "official_open_data_and_gis", ReferenceURI: dohAADTReference, RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}
