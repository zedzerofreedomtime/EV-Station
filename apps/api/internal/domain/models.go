package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type DataStatus string

const (
	DataVerified    DataStatus = "verified"
	DataEstimated   DataStatus = "estimated"
	DataPreliminary DataStatus = "preliminary"
	DataMissing     DataStatus = "missing"
)

type Site struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Address       string     `json:"address,omitempty"`
	Latitude      *float64   `json:"latitude,omitempty"`
	Longitude     *float64   `json:"longitude,omitempty"`
	LandSize      float64    `json:"landSize"`
	LandSizeUnit  string     `json:"landSizeUnit"`
	GoogleMapsURL string     `json:"googleMapsUrl,omitempty"`
	Notes         string     `json:"notes,omitempty"`
	InputStatus   DataStatus `json:"inputStatus"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type CreateSiteInput struct {
	Name          string   `json:"name" binding:"required,max=160"`
	Address       string   `json:"address" binding:"max=1000"`
	Latitude      *float64 `json:"latitude"`
	Longitude     *float64 `json:"longitude"`
	LandSize      float64  `json:"landSize" binding:"required,gt=0"`
	LandSizeUnit  string   `json:"landSizeUnit" binding:"required,oneof=sqm rai ngan sqwah"`
	GoogleMapsURL string   `json:"googleMapsUrl" binding:"omitempty,url"`
	Notes         string   `json:"notes" binding:"max=5000"`
}

type DataSource struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// Authority identifies who publishes the underlying dataset. GeographicScope
	// makes clear the level at which the source is factual; neither field alone
	// means that a conclusion has been confirmed for the submitted plot.
	Authority        string     `json:"authority,omitempty"`
	GeographicScope  string     `json:"geographicScope,omitempty"`
	SiteVerification string     `json:"siteVerification,omitempty"`
	ReferenceURI     string     `json:"referenceUri,omitempty"`
	DatasetVersion   string     `json:"datasetVersion,omitempty"`
	ObservedAt       *time.Time `json:"observedAt,omitempty"`
	RetrievedAt      time.Time  `json:"retrievedAt"`
	Methodology      string     `json:"methodology,omitempty"`
	License          string     `json:"license,omitempty"`
}

type Metric struct {
	ID              uuid.UUID       `json:"id"`
	AnalysisRunID   uuid.UUID       `json:"analysisRunId"`
	Type            string          `json:"type"`
	RawValue        json.RawMessage `json:"rawValue,omitempty"`
	NormalizedScore *float64        `json:"normalizedScore,omitempty"`
	Status          DataStatus      `json:"status"`
	Source          DataSource      `json:"source"`
	Assumptions     []string        `json:"assumptions"`
	CreatedAt       time.Time       `json:"createdAt"`
}

type FinancialResult struct {
	InitialInvestment    float64  `json:"initialInvestment"`
	MonthlyRevenue       float64  `json:"monthlyRevenue"`
	MonthlyOperatingCost float64  `json:"monthlyOperatingCost"`
	MonthlyProfit        float64  `json:"monthlyProfit"`
	AnnualProfit         float64  `json:"annualProfit"`
	ROIPercentage        *float64 `json:"roiPercentage,omitempty"`
	PaybackMonths        *float64 `json:"paybackMonths,omitempty"`
	Assumptions          []string `json:"assumptions"`
}

type ScoringSummary struct {
	Version                string   `json:"version"`
	CoveragePercentage     float64  `json:"coveragePercentage"`
	ScoredMetricCount      int      `json:"scoredMetricCount"`
	RequiredMetricCount    int      `json:"requiredMetricCount"`
	ExcludedMetrics        []string `json:"excludedMetrics"`
	MinimumCoveragePercent float64  `json:"minimumCoveragePercentage"`
	Limitations            []string `json:"limitations"`
}

type AnalysisRun struct {
	ID                   uuid.UUID        `json:"id"`
	SiteID               uuid.UUID        `json:"siteId"`
	Status               string           `json:"status"`
	AnalysisRadiusMeters int              `json:"analysisRadiusMeters"`
	OverallScore         *float64         `json:"overallScore,omitempty"`
	AssessmentStatus     DataStatus       `json:"assessmentStatus"`
	Recommendation       string           `json:"recommendation"`
	Metrics              []Metric         `json:"metrics"`
	Financial            *FinancialResult `json:"financial,omitempty"`
	Scoring              *ScoringSummary  `json:"scoring,omitempty"`
	StartedAt            time.Time        `json:"startedAt"`
	CompletedAt          *time.Time       `json:"completedAt,omitempty"`
	CreatedAt            time.Time        `json:"createdAt"`
}

// AIAssessment is explanatory output generated from an existing analysis run.
type AIAssessment struct {
	Summary        string    `json:"summary"`
	Recommendation string    `json:"recommendation"`
	Strengths      []string  `json:"strengths"`
	Risks          []string  `json:"risks"`
	RequiredChecks []string  `json:"requiredChecks"`
	Disclaimer     string    `json:"disclaimer"`
	Language       string    `json:"language"`
	Model          string    `json:"model"`
	GeneratedAt    time.Time `json:"generatedAt"`
}

// AIScoring contains Gemini's evidence-bound scoring recommendation. It can only
// score metrics for which the backend has already collected usable evidence.
// The backend validates every value and calculates the weighted total itself.
type AIScoring struct {
	MetricScores   map[string]float64 `json:"metricScores"`
	Recommendation string             `json:"recommendation"`
	Disclaimer     string             `json:"disclaimer"`
	Language       string             `json:"language"`
	Model          string             `json:"model"`
	GeneratedAt    time.Time          `json:"generatedAt"`
}
