package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/cache"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

const peaPowerMapReference = "https://www.pea.co.th/pea-power-map"
const peaConsumerPowerMapURL = "https://ppmload.pea.co.th/"

type PEAPowerMapConfig struct {
	PageURL  string
	CacheTTL time.Duration
}

type PEAPowerMapProvider struct {
	config PEAPowerMapConfig
}

type peaPowerMapValue struct {
	AssessmentType string `json:"assessmentType"`
	MapURL         string `json:"mapUrl"`
	ServiceArea    string `json:"serviceArea"`
	DataAvailable  bool   `json:"dataAvailable"`
}

func NewPEAPowerMapProvider(config PEAPowerMapConfig, _ cache.Cache) *PEAPowerMapProvider {
	if config.PageURL == "" {
		config.PageURL = peaConsumerPowerMapURL
	}
	return &PEAPowerMapProvider{config: config}
}

// Collect exposes the official PEA planning-map entry point for sites outside
// the MEA service area. PEA publishes the map as an interactive application,
// not as a public point-query API with a contractual capacity result, so this
// provider deliberately does not infer a kW/MW value.
func (p *PEAPowerMapProvider) Collect(_ context.Context, site domain.Site, _ int) ([]Observation, error) {
	observations, positions := unavailableObservations()
	if site.Latitude == nil || site.Longitude == nil {
		observations[positions["electrical"]] = p.missing("ต้องมีพิกัดเพื่อเปิดแผนที่ศักยภาพระบบไฟฟ้า PEA")
		return observations, nil
	}
	if inMEARoughServiceArea(*site.Latitude, *site.Longitude) {
		// MEA is the authoritative provider for its service area. Returning a
		// missing observation lets CompositeProvider retain MEA's result.
		return observations, nil
	}

	raw, _ := json.Marshal(peaPowerMapValue{
		AssessmentType: "official_public_map_guideline",
		MapURL:         p.config.PageURL,
		ServiceArea:    "PEA (พื้นที่ต่างจังหวัดและพื้นที่นอกเขต MEA)",
		DataAvailable:  true,
	})
	now := time.Now().UTC()
	observations[positions["electrical"]] = Observation{
		MetricType: "electrical", RawValue: raw, Status: domain.DataPreliminary,
		Source: domain.DataSource{
			Name: "PEA Power Map สำหรับผู้ใช้ไฟฟ้า", Type: "official_public_power_map", Authority: "official",
			GeographicScope: "pea_service_area", SiteVerification: "preliminary_map_reference",
			ReferenceURI: peaPowerMapReference, DatasetVersion: "PEA Power Map (แผนที่ออนไลน์ปัจจุบัน)", RetrievedAt: now,
			Methodology: "ระบุพื้นที่ให้บริการ PEA จากพิกัดเบื้องต้น และเชื่อมไปยัง PEA Power Map สำหรับตรวจสอบศักยภาพรายพื้นที่",
		},
		Assumptions: []string{
			"PEA Power Map ใช้สำหรับวางแผนเบื้องต้นเท่านั้น ไม่ใช่ผลยืนยันกำลังไฟคงเหลือของแปลงนี้",
			"ระบบยังไม่สร้างค่ากำลังไฟหรือยืนยันไฟ 3 เฟสจากแผนที่สาธารณะ",
			"ต้องให้ PEA ตรวจสอบหม้อแปลง จุดเชื่อมต่อ และความสามารถรองรับก่อนตัดสินใจลงทุน",
		},
	}
	return observations, nil
}

// MEA serves Bangkok, Nonthaburi and Samut Prakan. This conservative envelope
// prevents the PEA reference from replacing a real MEA result in the overlap.
func inMEARoughServiceArea(latitude, longitude float64) bool {
	return latitude >= 13.45 && latitude <= 13.98 && longitude >= 100.25 && longitude <= 100.95
}

func (p *PEAPowerMapProvider) missing(assumption string) Observation {
	return Observation{MetricType: "electrical", Status: domain.DataMissing, Source: domain.DataSource{
		Name: "PEA Power Map", Type: "official_public_power_map", Authority: "official",
		GeographicScope: "pea_service_area", ReferenceURI: peaPowerMapReference, RetrievedAt: time.Now().UTC(),
	}, Assumptions: []string{assumption}}
}
