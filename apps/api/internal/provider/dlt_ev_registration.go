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
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/extrame/xls"
	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const dltEVRegistrationReference = "https://data.go.th/dataset/dataset_1_1_04"
const dltEVRegistrationPreviousURL = "https://web.dlt.go.th/statistics/load_file_select_new_car.php?t=3&data_file=1057"
const dltCentralOfficeSheet = "ส่วนกลาง"

type DLTEVRegistrationConfig struct {
	CSVURL              string
	DatasetDate         string
	PreviousCSVURL      string
	PreviousDatasetDate string
	CacheTTL            time.Duration
	UserAgent           string
}
type DLTEVRegistrationProvider struct {
	config DLTEVRegistrationConfig
	client *http.Client
	cache  cache.Cache
}
type dltReverseResponse struct {
	Address struct {
		Province string `json:"province"`
		State    string `json:"state"`
		City     string `json:"city"`
	} `json:"address"`
}
type dltEVValue struct {
	Province              string   `json:"province"`
	RegisteredBEV         int      `json:"registeredBev"`
	DatasetDate           string   `json:"datasetDate"`
	PreviousRegisteredBEV *int     `json:"previousRegisteredBev,omitempty"`
	PreviousDatasetDate   string   `json:"previousDatasetDate,omitempty"`
	ChangePercent         *float64 `json:"changePercent,omitempty"`
	Trend                 string   `json:"trend,omitempty"`
	ProvinceResolution    string   `json:"provinceResolution"`
}

func NewDLTEVRegistrationProvider(config DLTEVRegistrationConfig, client *http.Client, externalCache cache.Cache) *DLTEVRegistrationProvider {
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	if externalCache == nil {
		externalCache = cache.Noop{}
	}
	if config.CSVURL == "" {
		config.CSVURL = "https://gdcatalog.dlt.go.th/dataset/e8b7d87e-5a3c-4b8b-854c-974d3b600256/resource/e5a037c6-3cc2-415e-b559-e6e85cfba2ca/download/stt_car_fuel_at_25690228.csv"
	}
	if config.DatasetDate == "" {
		config.DatasetDate = "28 February 2569"
	}
	if config.PreviousCSVURL == "" {
		config.PreviousCSVURL = dltEVRegistrationPreviousURL
	}
	if config.PreviousDatasetDate == "" {
		config.PreviousDatasetDate = "28 February 2568"
	}
	if config.CacheTTL <= 0 {
		config.CacheTTL = 30 * 24 * time.Hour
	}
	return &DLTEVRegistrationProvider{config: config, client: client, cache: externalCache}
}
func (p *DLTEVRegistrationProvider) Collect(ctx context.Context, site domain.Site, _ int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["ev_demand"]] = p.missing("Valid coordinates are required to determine the province for the provincial BEV registration dataset.")
		return observations, nil
	}
	province, err := p.reverseProvince(ctx, *site.Latitude, *site.Longitude)
	if err != nil || province == "" {
		observations[positions["ev_demand"]] = p.missing("The province could not be resolved from the submitted coordinates; no provincial BEV estimate was produced.")
		return observations, nil
	}
	registrations, err := p.fetch(ctx, p.config.CSVURL)
	if err != nil {
		observations[positions["ev_demand"]] = p.missing("The DLT provincial fuel-registration CSV was unavailable; no BEV estimate was produced.")
		return observations, nil
	}
	bev, found := registrations[province]
	if !found {
		observations[positions["ev_demand"]] = p.missing("The resolved province has no exact BEV row in the current DLT release; no value was inferred.")
		return observations, nil
	}
	value := dltEVValue{Province: province, RegisteredBEV: bev, DatasetDate: p.config.DatasetDate, ProvinceResolution: "OpenStreetMap Nominatim reverse geocoding"}
	assumptions := []string{"This is an official provincial registered-BEV count, not EV demand measured at the submitted coordinates.", "The province is resolved through OpenStreetMap Nominatim reverse geocoding; staff should confirm the submitted coordinates when the site is close to a provincial boundary.", "Hybrid and plug-in hybrid fuel categories are excluded; this value is not a count of all electrified vehicles.", "Provincial charging-station datasets on data.go.th are unevenly published and are not combined into a nationwide factual count in this MVP.", "The provider supplies evidence only; deterministic preliminary-v1 scoring is applied separately by backend logic."}
	datasetVersion := "Fuel-classified registered vehicles as at " + p.config.DatasetDate
	if p.config.PreviousCSVURL == "" || p.config.PreviousDatasetDate == "" {
		assumptions = append(assumptions, "No comparable prior official DLT release is configured, so no year-over-year EV registration trend is calculated.")
	} else if previousRegistrations, previousErr := p.fetch(ctx, p.config.PreviousCSVURL); previousErr != nil {
		assumptions = append(assumptions, "The configured prior official DLT release was unavailable, so no year-over-year EV registration trend is calculated.")
	} else if previousBEV, previousFound := previousRegistrations[province]; !previousFound {
		assumptions = append(assumptions, "The configured prior official DLT release has no exact BEV row for this province, so no year-over-year EV registration trend is calculated.")
	} else {
		value.PreviousRegisteredBEV = &previousBEV
		value.PreviousDatasetDate = p.config.PreviousDatasetDate
		value.Trend = registrationTrend(bev, previousBEV)
		if previousBEV > 0 {
			changePercent := roundDLTPercent((float64(bev-previousBEV) / float64(previousBEV)) * 100)
			value.ChangePercent = &changePercent
		}
		datasetVersion += "; compared with " + p.config.PreviousDatasetDate
		assumptions = append(assumptions, "The change compares official provincial BEV counts from two configured DLT releases using the same fuel category. It is a registration trend, not EV demand at the submitted coordinates.")
	}
	raw, _ := json.Marshal(value)
	observations[positions["ev_demand"]] = Observation{MetricType: "ev_demand", RawValue: raw, Status: domain.DataVerified, Source: domain.DataSource{Name: "DLT registered BEV by province", Type: "official_open_data", Authority: "official", GeographicScope: "province", SiteVerification: "verified_at_dataset_scope", ReferenceURI: dltEVRegistrationReference, DatasetVersion: datasetVersion, RetrievedAt: time.Now().UTC(), Methodology: "Counts only DLT rows whose fuel type is exactly ไฟฟ้า (electric) for the province resolved from submitted coordinates. When a comparable prior official release is configured, calculates (current count − prior count) ÷ prior count × 100."}, Assumptions: assumptions}
	return observations, nil
}
func (p *DLTEVRegistrationProvider) fetch(ctx context.Context, sourceURL string) (map[string]int, error) {
	key := "dlt:bev:" + hashDLT(sourceURL)
	if payload, found, err := p.cache.Get(ctx, key); err == nil && found {
		return parseDLTBEVPayload(payload)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", p.config.UserAgent)
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("DLT registration source returned %d", res.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	values, err := parseDLTBEVPayload(payload)
	if err == nil {
		_ = p.cache.Set(ctx, key, payload, p.config.CacheTTL)
	}
	return values, err
}

func parseDLTBEVPayload(payload []byte) (map[string]int, error) {
	if len(payload) >= 8 && bytes.Equal(payload[:8], []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}) {
		return parseDLTBEVXLS(payload)
	}
	return parseDLTBEV(payload)
}

func parseDLTBEVXLS(payload []byte) (map[string]int, error) {
	book, err := xls.OpenReader(bytes.NewReader(payload), "tis-620")
	if err != nil {
		return nil, fmt.Errorf("invalid DLT registration XLS: %w", err)
	}

	values := map[string]int{}
	for sheetIndex := 0; sheetIndex < book.NumSheets(); sheetIndex++ {
		sheet := book.GetSheet(sheetIndex)
		if sheet == nil {
			continue
		}
		province := normalizeDLTProvince(sheet.Name)
		if province == dltCentralOfficeSheet {
			province = "กรุงเทพมหานคร"
		}
		if !isDLTProvince(province) {
			continue
		}
		if value, found := parseDLTProvinceFuelSheet(sheet); found {
			values[province] = value
		}
	}
	if len(values) > 0 {
		return values, nil
	}
	return nil, fmt.Errorf("DLT registration XLS has no provincial BEV rows")
}

func parseDLTProvinceFuelSheet(sheet *xls.WorkSheet) (int, bool) {
	const maxRows = 200
	const maxColumns = 40
	for rowIndex := 0; rowIndex <= maxRows; rowIndex++ {
		for columnIndex := 0; columnIndex < maxColumns; columnIndex++ {
			if normalizeDLTCell(dltSheetCell(sheet, rowIndex, columnIndex)) != "ไฟฟ้า" {
				continue
			}
			for valueRow := rowIndex + 1; valueRow <= min(rowIndex+4, maxRows); valueRow++ {
				if value, ok := parseDLTInteger(dltSheetCell(sheet, valueRow, columnIndex)); ok {
					return value, true
				}
			}
			return 0, false
		}
	}
	return 0, false
}

func dltSheetCell(sheet *xls.WorkSheet, rowIndex, columnIndex int) (value string) {
	defer func() {
		if recover() != nil {
			value = ""
		}
	}()
	row := sheet.Row(rowIndex)
	if row == nil {
		return ""
	}
	return row.Col(columnIndex)
}

func parseDLTInteger(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if parsedDate, err := time.Parse(time.RFC3339, value); err == nil {
		base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		days := parsedDate.Sub(base).Hours() / 24
		const durationOverflowDays = float64(1<<63) / float64(24*time.Hour)
		if parsedDate.Before(base) {
			for days < durationOverflowDays {
				days += durationOverflowDays
			}
		}
		return int(math.Round(days)), true
	}
	value = strings.NewReplacer(",", "", " ", "", "-", "").Replace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, false
	}
	return int(math.Round(parsed)), true
}

func normalizeDLTCell(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), "")
}

func normalizeDLTProvince(value string) string {
	return strings.TrimPrefix(normalizeDLTCell(value), "จังหวัด")
}

func isDLTProvince(value string) bool {
	_, found := dltProvinces[value]
	return found
}

var dltProvinces = map[string]struct{}{
	"กรุงเทพมหานคร": {}, "กระบี่": {}, "กาญจนบุรี": {}, "กาฬสินธุ์": {}, "กำแพงเพชร": {}, "ขอนแก่น": {}, "จันทบุรี": {}, "ฉะเชิงเทรา": {}, "ชลบุรี": {}, "ชัยนาท": {}, "ชัยภูมิ": {}, "ชุมพร": {}, "เชียงราย": {}, "เชียงใหม่": {}, "ตรัง": {}, "ตราด": {}, "ตาก": {}, "นครนายก": {}, "นครปฐม": {}, "นครพนม": {}, "นครราชสีมา": {}, "นครศรีธรรมราช": {}, "นครสวรรค์": {}, "นนทบุรี": {}, "นราธิวาส": {}, "น่าน": {}, "บึงกาฬ": {}, "บุรีรัมย์": {}, "ปทุมธานี": {}, "ประจวบคีรีขันธ์": {}, "ปราจีนบุรี": {}, "ปัตตานี": {}, "พะเยา": {}, "พังงา": {}, "พัทลุง": {}, "พิจิตร": {}, "พิษณุโลก": {}, "เพชรบุรี": {}, "เพชรบูรณ์": {}, "แพร่": {}, "ภูเก็ต": {}, "มหาสารคาม": {}, "มุกดาหาร": {}, "แม่ฮ่องสอน": {}, "ยโสธร": {}, "ยะลา": {}, "ร้อยเอ็ด": {}, "ระนอง": {}, "ระยอง": {}, "ราชบุรี": {}, "ลพบุรี": {}, "ลำปาง": {}, "ลำพูน": {}, "เลย": {}, "ศรีสะเกษ": {}, "สกลนคร": {}, "สงขลา": {}, "สตูล": {}, "สมุทรปราการ": {}, "สมุทรสงคราม": {}, "สมุทรสาคร": {}, "สระแก้ว": {}, "สระบุรี": {}, "สิงห์บุรี": {}, "สุโขทัย": {}, "สุพรรณบุรี": {}, "สุราษฎร์ธานี": {}, "สุรินทร์": {}, "หนองคาย": {}, "หนองบัวลำภู": {}, "อ่างทอง": {}, "อำนาจเจริญ": {}, "อุดรธานี": {}, "อุตรดิตถ์": {}, "อุทัยธานี": {}, "อุบลราชธานี": {},
}

func registrationTrend(current, previous int) string {
	switch {
	case current > previous:
		return "increased"
	case current < previous:
		return "decreased"
	default:
		return "unchanged"
	}
}

func roundDLTPercent(value float64) float64 { return math.Round(value*10) / 10 }
func (p *DLTEVRegistrationProvider) reverseProvince(ctx context.Context, lat, lon float64) (string, error) {
	params := url.Values{"format": {"jsonv2"}, "lat": {strconv.FormatFloat(lat, 'f', 6, 64)}, "lon": {strconv.FormatFloat(lon, 'f', 6, 64)}, "zoom": {"10"}, "addressdetails": {"1"}, "accept-language": {"th,en"}}
	key := "nominatim:reverse-province:" + hashDLT(params.Encode())
	if payload, found, err := p.cache.Get(ctx, key); err == nil && found {
		return parseDLTProvince(payload)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://nominatim.openstreetmap.org/reverse?"+params.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", p.config.UserAgent)
	res, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("reverse geocoding returned %d", res.StatusCode)
	}
	province, err := parseDLTProvince(payload)
	if err == nil {
		_ = p.cache.Set(ctx, key, payload, 30*24*time.Hour)
	}
	return province, err
}
func parseDLTBEV(payload []byte) (map[string]int, error) {
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(payload), "\ufeff")))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, fmt.Errorf("invalid DLT registration CSV")
	}
	result := map[string]int{}
	for _, row := range rows[1:] {
		if len(row) < 5 || strings.TrimSpace(row[3]) != "ไฟฟ้า" {
			continue
		}
		value, err := strconv.Atoi(strings.ReplaceAll(strings.TrimSpace(row[4]), ",", ""))
		if err != nil {
			continue
		}
		result[strings.TrimSpace(row[2])] += value
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("DLT registration CSV has no BEV rows")
	}
	return result, nil
}
func parseDLTProvince(payload []byte) (string, error) {
	var response dltReverseResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return "", err
	}
	province := strings.TrimPrefix(strings.TrimSpace(response.Address.Province), "จังหวัด")
	if province == "" {
		province = strings.TrimPrefix(strings.TrimSpace(response.Address.State), "จังหวัด")
	}
	if province == "" {
		city := strings.TrimSpace(response.Address.City)
		if city == "กรุงเทพมหานคร" {
			province = city
		}
	}
	if province == "" {
		return "", fmt.Errorf("reverse geocoder returned no province")
	}
	return province, nil
}
func hashDLT(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func (p *DLTEVRegistrationProvider) missing(assumption string) Observation {
	return Observation{MetricType: "ev_demand", Status: domain.DataMissing, Source: domain.DataSource{Name: "DLT registered BEV by province", Type: "official_open_data", Authority: "official", GeographicScope: "province", ReferenceURI: dltEVRegistrationReference, RetrievedAt: time.Now().UTC()}, Assumptions: []string{assumption}}
}
