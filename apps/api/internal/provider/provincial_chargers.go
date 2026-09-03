package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const (
	suphanburiChargerReference = "https://data.go.th/dataset/energy_ev"
	saraburiChargerReference   = "https://data.go.th/dataset/ev-station"
)

type ProvincialChargerConfig struct {
	SuphanburiCSVURL string
	SaraburiJSONURL  string
	CacheTTL         time.Duration
	UserAgent        string
}

type ProvincialChargerProvider struct {
	config ProvincialChargerConfig
	client *http.Client
	cache  cache.Cache
}

type competitionPlace struct {
	RecordType        string   `json:"recordType"`
	RecordID          string   `json:"recordId"`
	Name              string   `json:"name,omitempty"`
	Category          string   `json:"category"`
	Latitude          float64  `json:"latitude"`
	Longitude         float64  `json:"longitude"`
	Province          string   `json:"province,omitempty"`
	Operator          string   `json:"operator,omitempty"`
	SourceNames       []string `json:"sourceNames"`
	SourceRecordCount int      `json:"sourceRecordCount"`
}

type competitionSourceBreakdown struct {
	Name           string     `json:"name"`
	ReferenceURI   string     `json:"referenceUri"`
	Coverage       string     `json:"coverage"`
	Count          int        `json:"count"`
	RetrievedAt    time.Time  `json:"retrievedAt"`
	ObservedAt     *time.Time `json:"observedAt,omitempty"`
	DatasetVersion string     `json:"datasetVersion,omitempty"`
	License        string     `json:"license,omitempty"`
}

type competitionMetricValue struct {
	Count                     int                          `json:"count"`
	RadiusMeters              int                          `json:"radiusMeters"`
	CoverageMatched           bool                         `json:"coverageMatched"`
	Places                    []competitionPlace           `json:"places"`
	Sources                   []competitionSourceBreakdown `json:"sources"`
	UnmaterializedRecordCount int                          `json:"unmaterializedRecordCount,omitempty"`
}

type provincialDatasetResult struct {
	Places []competitionPlace
	Source competitionSourceBreakdown
}

type saraburiChargerResponse struct {
	Metadata struct {
		Title       string `json:"title"`
		License     string `json:"license"`
		LastUpdated string `json:"last_updated"`
	} `json:"metadata"`
	Data []struct {
		ID           string   `json:"id"`
		Name         string   `json:"name"`
		Latitude     float64  `json:"latitude"`
		Longitude    float64  `json:"longitude"`
		EnergyTypes  []string `json:"energy_types"`
		HasEVCharger bool     `json:"has_ev_charger"`
		Brand        struct {
			Name string `json:"name"`
		} `json:"brand"`
	} `json:"data"`
}

var destinationCoordinatesPattern = regexp.MustCompile(`(?i)destination=([+-]?\d+(?:\.\d+)?)\s*,\s*([+-]?\d+(?:\.\d+)?)`)

func NewProvincialChargerProvider(config ProvincialChargerConfig, client *http.Client, externalCache cache.Cache) *ProvincialChargerProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &ProvincialChargerProvider{config: config, client: client, cache: externalCache}
}

func (p *ProvincialChargerProvider) Collect(ctx context.Context, site domain.Site, radius int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["competition"]] = p.missing("Valid coordinates are required to query provincial charging-station datasets.")
		return observations, nil
	}
	if radius <= 0 {
		radius = 3000
	}

	results := make([]provincialDatasetResult, 0, 2)
	if strings.TrimSpace(p.config.SuphanburiCSVURL) != "" {
		if result, err := p.loadSuphanburi(ctx); err == nil {
			results = append(results, result)
		}
	}
	if strings.TrimSpace(p.config.SaraburiJSONURL) != "" {
		if result, err := p.loadSaraburi(ctx); err == nil {
			results = append(results, result)
		}
	}
	if len(results) == 0 {
		observations[positions["competition"]] = p.missing("Configured provincial charging-station datasets were unavailable; no provincial value was inferred.")
		return observations, nil
	}

	value := competitionMetricValue{RadiusMeters: radius, Places: make([]competitionPlace, 0), Sources: make([]competitionSourceBreakdown, 0, len(results))}
	for _, result := range results {
		nearby := make([]competitionPlace, 0)
		for _, place := range result.Places {
			if haversineMeters(*site.Latitude, *site.Longitude, place.Latitude, place.Longitude) <= float64(radius) {
				nearby = append(nearby, place)
			}
		}
		result.Source.Count = len(nearby)
		// The currently connected provincial datasets cannot prove a zero
		// result outside their own geographic coverage. A nearby returned
		// record is usable evidence; otherwise keep the result preliminary.
		if len(nearby) > 0 {
			value.CoverageMatched = true
		}
		value.Sources = append(value.Sources, result.Source)
		value.Places = append(value.Places, nearby...)
	}
	value.Places = deduplicateCompetitionPlaces(value.Places)
	value.Count = len(value.Places)
	if len(value.Places) > 100 {
		value.Places = value.Places[:100]
	}
	sort.Slice(value.Sources, func(i, j int) bool { return value.Sources[i].Name < value.Sources[j].Name })
	raw, _ := json.Marshal(value)
	now := time.Now().UTC()
	observations[positions["competition"]] = Observation{
		MetricType: "competition",
		RawValue:   raw,
		Status:     competitionStatus(value.CoverageMatched),
		Source: domain.DataSource{
			Name:         "Provincial government charging-station datasets",
			Type:         "official_open_data_multi_source",
			ReferenceURI: "https://data.go.th/",
			RetrievedAt:  now,
			Methodology:  "Filter connected provincial source records by great-circle distance from the submitted coordinates and deduplicate repeated station records before counting.",
			License:      "Dataset-specific open-data licences; see source breakdown.",
		},
		Assumptions: []string{
			"Coverage currently includes only the connected Suphan Buri and Saraburi provincial datasets; zero records do not prove that no charging station exists.",
			"A source record confirms only that the publisher returned that record; current operation, connector availability, power and access were not independently verified.",
			"Repeated rows and cross-source records are deterministically deduplicated by coordinates and normalized station name.",
			"The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic.",
		},
	}
	return observations, nil
}

func competitionStatus(coverageMatched bool) domain.DataStatus {
	if coverageMatched {
		return domain.DataVerified
	}
	return domain.DataPreliminary
}

func (p *ProvincialChargerProvider) loadSuphanburi(ctx context.Context) (provincialDatasetResult, error) {
	payload, retrievedAt, err := p.fetch(ctx, p.config.SuphanburiCSVURL)
	if err != nil {
		return provincialDatasetResult{}, err
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(payload, []byte{0xef, 0xbb, 0xbf})))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil || len(records) < 1 {
		return provincialDatasetResult{}, fmt.Errorf("invalid Suphan Buri charger CSV")
	}
	headings := make(map[string]int)
	for index, heading := range records[0] {
		headings[strings.TrimSpace(heading)] = index
	}
	nameIndex, nameOK := headings["ชื่อสถานี"]
	locationIndex, locationOK := headings["ตำแหน่งที่ตั้ง"]
	operatorIndex := headings["ประเภทสถานี"]
	if !nameOK || !locationOK {
		return provincialDatasetResult{}, fmt.Errorf("Suphan Buri charger CSV schema changed")
	}

	places := make([]competitionPlace, 0, len(records)-1)
	for rowIndex, row := range records[1:] {
		if nameIndex >= len(row) || locationIndex >= len(row) {
			continue
		}
		latitude, longitude, ok := parseDestinationCoordinates(row[locationIndex])
		if !ok {
			continue
		}
		operator := ""
		if operatorIndex < len(row) {
			operator = strings.TrimSpace(row[operatorIndex])
		}
		places = append(places, competitionPlace{RecordType: "provincial_csv", RecordID: "suphanburi:" + strconv.Itoa(rowIndex+2), Name: strings.TrimSpace(row[nameIndex]), Category: "charging_station", Latitude: latitude, Longitude: longitude, Province: "Suphan Buri", Operator: operator, SourceNames: []string{"Suphan Buri Provincial Office EV charging points"}, SourceRecordCount: 1})
	}
	places = deduplicateCompetitionPlaces(places)
	return provincialDatasetResult{Places: places, Source: competitionSourceBreakdown{Name: "Suphan Buri Provincial Office EV charging points", ReferenceURI: suphanburiChargerReference, Coverage: "Suphan Buri province only", RetrievedAt: retrievedAt, DatasetVersion: "Dataset updated 2 July 2024", License: "Open Data Common"}}, nil
}

func (p *ProvincialChargerProvider) loadSaraburi(ctx context.Context) (provincialDatasetResult, error) {
	payload, retrievedAt, err := p.fetch(ctx, p.config.SaraburiJSONURL)
	if err != nil {
		return provincialDatasetResult{}, err
	}
	var response saraburiChargerResponse
	if err = json.Unmarshal(payload, &response); err != nil {
		return provincialDatasetResult{}, fmt.Errorf("invalid Saraburi charger JSON: %w", err)
	}
	places := make([]competitionPlace, 0)
	for _, record := range response.Data {
		isEV := record.HasEVCharger
		for _, energyType := range record.EnergyTypes {
			if strings.EqualFold(strings.TrimSpace(energyType), "EV") {
				isEV = true
			}
		}
		if !isEV || record.Latitude == 0 || record.Longitude == 0 {
			continue
		}
		places = append(places, competitionPlace{RecordType: "provincial_api", RecordID: "saraburi:" + record.ID, Name: strings.TrimSpace(record.Name), Category: "charging_station", Latitude: record.Latitude, Longitude: record.Longitude, Province: "Saraburi", Operator: strings.TrimSpace(record.Brand.Name), SourceNames: []string{"Saraburi Provincial Energy Office stations API"}, SourceRecordCount: 1})
	}
	var observedAt *time.Time
	if parsed, parseErr := time.Parse(time.RFC3339Nano, response.Metadata.LastUpdated); parseErr == nil {
		observedAt = &parsed
	}
	version := response.Metadata.LastUpdated
	if version == "" {
		version = "Publisher metadata did not provide a last-updated value"
	}
	license := response.Metadata.License
	if license == "" {
		license = "Open Data Common (data.go.th metadata)"
	}
	return provincialDatasetResult{Places: deduplicateCompetitionPlaces(places), Source: competitionSourceBreakdown{Name: "Saraburi Provincial Energy Office stations API", ReferenceURI: saraburiChargerReference, Coverage: "Saraburi province only", RetrievedAt: retrievedAt, ObservedAt: observedAt, DatasetVersion: version, License: license}}, nil
}

func (p *ProvincialChargerProvider) fetch(ctx context.Context, endpoint string) ([]byte, time.Time, error) {
	hash := sha256.Sum256([]byte(endpoint))
	key := "provincial-chargers:" + hex.EncodeToString(hash[:])
	if payload, found, err := p.cache.Get(ctx, key); err == nil && found {
		return payload, time.Now().UTC(), nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, time.Time{}, err
	}
	req.Header.Set("Accept", "application/json,text/csv,*/*")
	req.Header.Set("User-Agent", p.config.UserAgent)
	response, err := p.client.Do(req)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, time.Time{}, fmt.Errorf("provincial charger source returned %d", response.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(response.Body, 20<<20))
	if err != nil {
		return nil, time.Time{}, err
	}
	_ = p.cache.Set(ctx, key, payload, p.config.CacheTTL)
	return payload, time.Now().UTC(), nil
}

func parseDestinationCoordinates(raw string) (float64, float64, bool) {
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		decoded = raw
	}
	match := destinationCoordinatesPattern.FindStringSubmatch(decoded)
	if len(match) != 3 {
		return 0, 0, false
	}
	latitude, latErr := strconv.ParseFloat(match[1], 64)
	longitude, lonErr := strconv.ParseFloat(match[2], 64)
	if latErr != nil || lonErr != nil || latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return 0, 0, false
	}
	return latitude, longitude, true
}

func deduplicateCompetitionPlaces(places []competitionPlace) []competitionPlace {
	sort.SliceStable(places, func(i, j int) bool {
		if places[i].Name != places[j].Name {
			return places[i].Name < places[j].Name
		}
		if places[i].Latitude != places[j].Latitude {
			return places[i].Latitude < places[j].Latitude
		}
		return places[i].Longitude < places[j].Longitude
	})
	result := make([]competitionPlace, 0, len(places))
	for _, candidate := range places {
		matched := -1
		candidateName := normalizeCompetitionName(candidate.Name)
		for index := range result {
			distance := haversineMeters(candidate.Latitude, candidate.Longitude, result[index].Latitude, result[index].Longitude)
			sameName := candidateName != "" && candidateName == normalizeCompetitionName(result[index].Name)
			if distance <= 20 || (sameName && distance <= 150) {
				matched = index
				break
			}
		}
		if matched < 0 {
			if candidate.SourceRecordCount <= 0 {
				candidate.SourceRecordCount = 1
			}
			candidate.SourceNames = uniqueSortedStrings(candidate.SourceNames)
			result = append(result, candidate)
			continue
		}
		result[matched].SourceRecordCount += maxInt(candidate.SourceRecordCount, 1)
		result[matched].SourceNames = uniqueSortedStrings(append(result[matched].SourceNames, candidate.SourceNames...))
		if result[matched].Operator == "" {
			result[matched].Operator = candidate.Operator
		}
	}
	return result
}

func normalizeCompetitionName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.NewReplacer("-", " ", "_", " ", ".", "", ",", "").Replace(value)), ""))
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func maxInt(first, second int) int {
	if first > second {
		return first
	}
	return second
}

func mergeCompetitionObservations(observations []Observation, fallbackRadius int) Observation {
	type rawCompetitionPlace struct {
		OSMType           string   `json:"osmType"`
		OSMID             int64    `json:"osmId"`
		RecordType        string   `json:"recordType"`
		RecordID          string   `json:"recordId"`
		Name              string   `json:"name"`
		Category          string   `json:"category"`
		Latitude          float64  `json:"latitude"`
		Longitude         float64  `json:"longitude"`
		Province          string   `json:"province"`
		Operator          string   `json:"operator"`
		SourceNames       []string `json:"sourceNames"`
		SourceRecordCount int      `json:"sourceRecordCount"`
	}
	type rawCompetitionValue struct {
		Count           int                          `json:"count"`
		RadiusMeters    int                          `json:"radiusMeters"`
		CoverageMatched bool                         `json:"coverageMatched"`
		Places          []rawCompetitionPlace        `json:"places"`
		Sources         []competitionSourceBreakdown `json:"sources"`
	}

	value := competitionMetricValue{RadiusMeters: fallbackRadius, Places: make([]competitionPlace, 0), Sources: make([]competitionSourceBreakdown, 0)}
	assumptions := make([]string, 0)
	strongestStatus := domain.DataPreliminary
	latestRetrieved := time.Time{}
	for _, observation := range observations {
		var raw rawCompetitionValue
		if err := json.Unmarshal(observation.RawValue, &raw); err != nil {
			continue
		}
		if raw.RadiusMeters > 0 {
			value.RadiusMeters = raw.RadiusMeters
		}
		value.CoverageMatched = value.CoverageMatched || raw.CoverageMatched || raw.Count > 0
		if dataStatusStrength(observation.Status) > dataStatusStrength(strongestStatus) {
			strongestStatus = observation.Status
		}
		if observation.Source.RetrievedAt.After(latestRetrieved) {
			latestRetrieved = observation.Source.RetrievedAt
		}
		assumptions = append(assumptions, observation.Assumptions...)
		if len(raw.Sources) == 0 {
			raw.Sources = []competitionSourceBreakdown{{Name: observation.Source.Name, ReferenceURI: observation.Source.ReferenceURI, Coverage: "OpenStreetMap mapped coverage within the requested radius", Count: raw.Count, RetrievedAt: observation.Source.RetrievedAt, ObservedAt: observation.Source.ObservedAt, DatasetVersion: observation.Source.DatasetVersion, License: observation.Source.License}}
		}
		value.Sources = append(value.Sources, raw.Sources...)
		for _, place := range raw.Places {
			recordType, recordID := place.RecordType, place.RecordID
			if place.OSMType != "" {
				recordType = "osm_" + place.OSMType
				recordID = place.OSMType + ":" + strconv.FormatInt(place.OSMID, 10)
			}
			sourceNames := place.SourceNames
			if len(sourceNames) == 0 {
				sourceNames = []string{observation.Source.Name}
			}
			value.Places = append(value.Places, competitionPlace{RecordType: recordType, RecordID: recordID, Name: place.Name, Category: place.Category, Latitude: place.Latitude, Longitude: place.Longitude, Province: place.Province, Operator: place.Operator, SourceNames: sourceNames, SourceRecordCount: maxInt(place.SourceRecordCount, 1)})
		}
		if raw.Count > len(raw.Places) {
			value.UnmaterializedRecordCount += raw.Count - len(raw.Places)
		}
	}

	value.Places = deduplicateCompetitionPlaces(value.Places)
	value.Count = len(value.Places) + value.UnmaterializedRecordCount
	if len(value.Places) > 100 {
		value.UnmaterializedRecordCount += len(value.Places) - 100
		value.Places = value.Places[:100]
	}
	sort.Slice(value.Sources, func(i, j int) bool {
		if value.Sources[i].Name != value.Sources[j].Name {
			return value.Sources[i].Name < value.Sources[j].Name
		}
		return value.Sources[i].ReferenceURI < value.Sources[j].ReferenceURI
	})
	value.Sources = deduplicateCompetitionSources(value.Sources)
	strongestStatus = competitionStatus(value.CoverageMatched)
	assumptions = uniqueSortedStrings(append(assumptions,
		"The combined count is the deterministic union of connected sources after coordinate-and-name deduplication; source coverage remains unequal across Thailand.",
		"A zero count is not shown as complete coverage unless a connected source specifically covers this location; it never proves absence of competitors.",
	))
	if latestRetrieved.IsZero() {
		latestRetrieved = time.Now().UTC()
	}
	raw, _ := json.Marshal(value)
	return Observation{MetricType: "competition", RawValue: raw, Status: strongestStatus, Source: domain.DataSource{Name: "Combined charging-station evidence", Type: "multi_source_open_data", ReferenceURI: "https://data.go.th/", RetrievedAt: latestRetrieved, Methodology: "Combine OpenStreetMap and connected provincial charging-station records, then deduplicate records within 20 metres or matching normalized names within 150 metres.", License: "Source-specific licences; see source breakdown."}, Assumptions: assumptions}
}

func deduplicateCompetitionSources(sources []competitionSourceBreakdown) []competitionSourceBreakdown {
	result := make([]competitionSourceBreakdown, 0, len(sources))
	seen := make(map[string]struct{}, len(sources))
	for _, source := range sources {
		key := source.Name + "\x00" + source.ReferenceURI
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, source)
	}
	return result
}

func (p *ProvincialChargerProvider) missing(assumption string) Observation {
	return Observation{MetricType: "competition", Status: domain.DataMissing, Source: domain.DataSource{Name: "Provincial government charging-station datasets", Type: "official_open_data_multi_source", ReferenceURI: "https://data.go.th/", RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}
