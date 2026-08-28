package financial

import (
	"errors"
	"strings"
)

var ErrUnknownPlan = errors.New("unknown franchise plan")

type FranchisePlan struct {
	Code                        string   `json:"code"`
	Name                        string   `json:"name"`
	RecommendedAreaSqWah        float64  `json:"recommendedAreaSqWah"`
	EVChargingStations          int      `json:"evChargingStations"`
	InvestmentMinTHB            float64  `json:"investmentMinThb"`
	InvestmentMaxTHB            *float64 `json:"investmentMaxThb,omitempty"`
	InvestmentUpperReferenceTHB *float64 `json:"investmentUpperReferenceThb,omitempty"`
	InvestmentOpenEnded         bool     `json:"investmentOpenEnded"`
	FranchiseFeeTHB             float64  `json:"franchiseFeeThb"`
	LocationProfile             []string `json:"locationProfile"`
	CoreServices                []string `json:"coreServices"`
	SourceStatus                string   `json:"sourceStatus"`
	SourceNote                  string   `json:"sourceNote"`
	ROIAvailable                bool     `json:"roiAvailable"`
	MissingForROI               []string `json:"missingForRoi"`
}

func FranchisePlans() []FranchisePlan {
	maxS, maxM, upperL := 2_500_000.0, 5_000_000.0, 10_000_000.0
	missing := []string{"approved charger power and quantity detail", "sessions per day", "energy per session", "electricity tariff", "selling price", "rent", "monthly operating cost"}
	return []FranchisePlan{
		{Code: "S", Name: "S – Smart Café", RecommendedAreaSqWah: 100, EVChargingStations: 1, InvestmentMinTHB: 1_500_000, InvestmentMaxTHB: &maxS, FranchiseFeeTHB: 300_000, LocationProfile: []string{"community", "building frontage", "fuel station", "takeaway location"}, CoreServices: []string{"coffee", "EV charging"}, SourceStatus: "user_supplied", SourceNote: "Transcribed from the franchise information image supplied by the project owner; whether the franchise fee is included in the investment range is not specified.", ROIAvailable: false, MissingForROI: append([]string(nil), missing...)},
		{Code: "M", Name: "M – Lifestyle Café", RecommendedAreaSqWah: 200, EVChargingStations: 2, InvestmentMinTHB: 3_500_000, InvestmentMaxTHB: &maxM, FranchiseFeeTHB: 500_000, LocationProfile: []string{"main road", "community mall", "office", "large fuel station"}, CoreServices: []string{"coffee", "food and bakery", "BPOST65 Express", "post office", "EV charging"}, SourceStatus: "user_supplied", SourceNote: "Transcribed from the franchise information image supplied by the project owner; whether the franchise fee is included in the investment range is not specified.", ROIAvailable: false, MissingForROI: append([]string(nil), missing...)},
		{Code: "L", Name: "L – Lifestyle Hub", RecommendedAreaSqWah: 400, EVChargingStations: 4, InvestmentMinTHB: 7_000_000, InvestmentUpperReferenceTHB: &upperL, InvestmentOpenEnded: true, FranchiseFeeTHB: 700_000, LocationProfile: []string{"main arterial road", "city entrance", "rest stop", "tourist attraction"}, CoreServices: []string{"coffee", "food and bakery", "BPOST65 Express", "post office", "EV charging", "work space", "mobile cafe"}, SourceStatus: "user_supplied", SourceNote: "The supplied image states an investment range of THB 7–10 million or more; no finite maximum is asserted, and whether the franchise fee is included is not specified.", ROIAvailable: false, MissingForROI: append([]string(nil), missing...)},
	}
}

func GetFranchisePlan(code string) (FranchisePlan, error) {
	requested := strings.ToUpper(strings.TrimSpace(code))
	for _, plan := range FranchisePlans() {
		if plan.Code == requested {
			return plan, nil
		}
	}
	return FranchisePlan{}, ErrUnknownPlan
}
