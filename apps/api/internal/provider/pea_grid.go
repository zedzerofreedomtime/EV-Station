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

const peaGridReference = "https://gisportal.pea.co.th/arcgis/rest/services/PEA_PAPD/PAPD_T_Station/FeatureServer"

type PEAGridConfig struct {
	StationURL         string
	ConductorURL       string
	SearchRadiusMeters int
	CacheTTL           time.Duration
	UserAgent          string
}

type PEAGridProvider struct {
	config PEAGridConfig
	client *http.Client
	cache  cache.Cache
}

type peaGridValue struct {
	AssessmentType               string   `json:"assessmentType"`
	SearchRadiusMeters           int      `json:"searchRadiusMeters"`
	HighVoltageLineCount         int      `json:"highVoltageLineCount"`
	NearestHighVoltageLineMeters *float64 `json:"nearestHighVoltageLineMeters,omitempty"`
	VoltageCode                  string   `json:"voltageCode,omitempty"`
	ConductorSize                string   `json:"conductorSize,omitempty"`
	PhaseCode                    string   `json:"phaseCode,omitempty"`
	FeederID                     string   `json:"feederId,omitempty"`
	NearestStationMeters         *float64 `json:"nearestStationMeters,omitempty"`
	StationName                  string   `json:"stationName,omitempty"`
	StationSecondaryVoltageKV    *float64 `json:"stationSecondaryVoltageKv,omitempty"`
}

type peaStationResponse struct {
	Features []struct {
		Attributes struct {
			StationName      string      `json:"STATIONNAME"`
			NameThai         string      `json:"NAME_THAI"`
			SecondaryVoltage interface{} `json:"SECONDARYVOLTAGE"`
		} `json:"attributes"`
		Geometry struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		} `json:"geometry"`
	} `json:"features"`
}

type peaConductorResponse struct {
	Features []struct {
		Attributes struct {
			VoltageCode   string      `json:"OP_VOLT"`
			ConductorSize interface{} `json:"CONDUCTORSIZE"`
			PhaseCode     interface{} `json:"PHASEDESIGNATION"`
			FeederID      string      `json:"FEEDERID"`
		} `json:"attributes"`
		Geometry struct {
			Paths [][][]float64 `json:"paths"`
		} `json:"geometry"`
	} `json:"features"`
}

func NewPEAGridProvider(config PEAGridConfig, client *http.Client, externalCache cache.Cache) *PEAGridProvider {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.SearchRadiusMeters <= 0 {
		config.SearchRadiusMeters = 5000
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 24 * time.Hour
	}
	return &PEAGridProvider{config: config, client: client, cache: externalCache}
}

func (p *PEAGridProvider) Collect(ctx context.Context, site domain.Site, _ int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["electrical"]] = p.missing("ต้องมีพิกัดเพื่อตรวจสอบชั้นข้อมูลโครงข่ายไฟฟ้าสาธารณะของ PEA")
		return observations, nil
	}
	if inMEARoughServiceArea(*site.Latitude, *site.Longitude) {
		return observations, nil
	}

	stationPayload, stationErr := p.query(ctx, p.config.StationURL, *site.Latitude, *site.Longitude, "TAG,STATIONNAME,NAME_THAI,SECONDARYVOLTAGE")
	linePayload, lineErr := p.query(ctx, p.config.ConductorURL, *site.Latitude, *site.Longitude, "TAG,OP_VOLT,CONDUCTORSIZE,PHASEDESIGNATION,FEEDERID")
	if stationErr != nil && lineErr != nil {
		observations[positions["electrical"]] = p.missing("ไม่สามารถอ่านชั้นข้อมูลสถานีและแนวโครงข่ายไฟฟ้าสาธารณะของ PEA ได้ในขณะนี้")
		return observations, nil
	}

	value := peaGridValue{AssessmentType: "pea_public_grid_evidence", SearchRadiusMeters: p.config.SearchRadiusMeters}
	if lineErr == nil {
		var response peaConductorResponse
		if err := json.Unmarshal(linePayload, &response); err == nil {
			value.HighVoltageLineCount = len(response.Features)
			minimum := math.Inf(1)
			for _, feature := range response.Features {
				for _, path := range feature.Geometry.Paths {
					for index := 1; index < len(path); index++ {
						if len(path[index-1]) < 2 || len(path[index]) < 2 {
							continue
						}
						distance := pointToSegmentMeters(*site.Latitude, *site.Longitude, path[index-1][1], path[index-1][0], path[index][1], path[index][0])
						if distance < minimum {
							minimum = distance
							value.VoltageCode = strings.TrimSpace(feature.Attributes.VoltageCode)
							value.ConductorSize = peaText(feature.Attributes.ConductorSize)
							value.PhaseCode = peaText(feature.Attributes.PhaseCode)
							value.FeederID = strings.TrimSpace(feature.Attributes.FeederID)
						}
					}
				}
			}
			if !math.IsInf(minimum, 1) {
				value.NearestHighVoltageLineMeters = &minimum
			}
		}
	}
	if stationErr == nil {
		var response peaStationResponse
		if err := json.Unmarshal(stationPayload, &response); err == nil {
			minimum := math.Inf(1)
			for _, feature := range response.Features {
				distance := haversineMeters(*site.Latitude, *site.Longitude, feature.Geometry.Y, feature.Geometry.X)
				if distance < minimum {
					minimum = distance
					value.StationName = firstNonEmpty(feature.Attributes.NameThai, feature.Attributes.StationName)
					if voltage, ok := peaNumber(feature.Attributes.SecondaryVoltage); ok {
						value.StationSecondaryVoltageKV = &voltage
					}
				}
			}
			if !math.IsInf(minimum, 1) {
				value.NearestStationMeters = &minimum
			}
		}
	}

	raw, _ := json.Marshal(value)
	observations[positions["electrical"]] = Observation{MetricType: "electrical", RawValue: raw, Status: domain.DataPreliminary,
		Source: domain.DataSource{Name: "PEA public GIS — high-voltage grid and station layers", Type: "official_public_gis", Authority: "official", GeographicScope: "pea_service_area", SiteVerification: "preliminary_public_grid_evidence", ReferenceURI: peaGridReference, RetrievedAt: time.Now().UTC(), Methodology: "Query PEA public high-voltage conductor and station layers around the submitted coordinates; calculate nearest returned feature distance from published geometries."},
		Assumptions: []string{
			"พบแนวโครงข่ายไฟฟ้าแรงสูงหรือสถานีไฟฟ้าในชั้นข้อมูล PEA ไม่ได้ยืนยันว่ามีเสาไฟหน้าแปลงหรือจุดเชื่อมต่อใช้งานได้จริง",
			"รหัสระดับแรงดัน รหัสเฟส และขนาดตัวนำแสดงตามชั้นข้อมูล PEA; ไม่ถูกแปลงเป็นกำลังไฟคงเหลือหรือรับรองไฟ 3 เฟส",
			"กำลังไฟที่จ่ายได้จริง จุดติดตั้งหม้อแปลง และการขอใช้ไฟสำหรับ EV Charger ต้องให้ PEA สำรวจและยืนยัน",
		},
	}
	return observations, nil
}

func (p *PEAGridProvider) query(ctx context.Context, endpoint string, latitude, longitude float64, fields string) ([]byte, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("PEA GIS endpoint is not configured")
	}
	form := url.Values{"f": {"json"}, "where": {"1=1"}, "geometry": {strconv.FormatFloat(longitude, 'f', 6, 64) + "," + strconv.FormatFloat(latitude, 'f', 6, 64)}, "geometryType": {"esriGeometryPoint"}, "inSR": {"4326"}, "distance": {strconv.Itoa(p.config.SearchRadiusMeters)}, "units": {"esriSRUnit_Meter"}, "outFields": {fields}, "returnGeometry": {"true"}, "outSR": {"4326"}}
	hash := sha256.Sum256([]byte(endpoint + "?" + form.Encode()))
	cacheKey := "pea:grid:" + hex.EncodeToString(hash[:])
	if value, found, err := p.cache.Get(ctx, cacheKey); err == nil && found {
		return value, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
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
	payload, err := io.ReadAll(io.LimitReader(response.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("PEA GIS returned status %d", response.StatusCode)
	}
	var failure struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(payload, &failure) == nil && len(failure.Error) > 0 {
		return nil, fmt.Errorf("PEA GIS returned an error")
	}
	_ = p.cache.Set(ctx, cacheKey, payload, p.config.CacheTTL)
	return payload, nil
}

func (p *PEAGridProvider) missing(assumption string) Observation {
	return Observation{MetricType: "electrical", Status: domain.DataMissing, Source: domain.DataSource{Name: "PEA public GIS — high-voltage grid and station layers", Type: "official_public_gis", Authority: "official", GeographicScope: "pea_service_area", ReferenceURI: peaGridReference, RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}

func peaNumber(value interface{}) (float64, bool) {
	switch candidate := value.(type) {
	case float64:
		return candidate, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(candidate), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func peaText(value interface{}) string {
	switch candidate := value.(type) {
	case string:
		return strings.TrimSpace(candidate)
	case float64:
		return strconv.FormatFloat(candidate, 'f', -1, 64)
	default:
		return ""
	}
}
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
