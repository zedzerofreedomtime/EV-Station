package advisory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var (
	ErrNotConfigured = errors.New("Gemini advisory is not configured")
	ErrUnavailable   = errors.New("Gemini advisory is unavailable")
	ErrInvalidOutput = errors.New("Gemini advisory returned an invalid response")
)

type GeminiConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Timeout time.Duration
}

type Service struct {
	config GeminiConfig
	client *http.Client
}

type geminiRequest struct {
	Contents []struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	} `json:"contents"`
	GenerationConfig struct {
		Temperature        float64        `json:"temperature"`
		MaxOutputTokens    int            `json:"maxOutputTokens"`
		ResponseMimeType   string         `json:"responseMimeType"`
		ResponseJSONSchema map[string]any `json:"responseJsonSchema"`
	} `json:"generationConfig"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

type generatedAssessment struct {
	Summary        string   `json:"summary"`
	Recommendation string   `json:"recommendation"`
	Strengths      []string `json:"strengths"`
	Risks          []string `json:"risks"`
	RequiredChecks []string `json:"requiredChecks"`
	Disclaimer     string   `json:"disclaimer"`
}

type generatedScoring struct {
	MetricScores   []generatedMetricScore `json:"metricScores"`
	Recommendation string                 `json:"recommendation"`
	Disclaimer     string                 `json:"disclaimer"`
}

type generatedMetricScore struct {
	MetricType string  `json:"metricType"`
	Score      float64 `json:"score"`
}

func NewGeminiService(config GeminiConfig, client *http.Client) *Service {
	if config.Model == "" {
		config.Model = "gemini-3.5-flash-lite"
	}
	if config.BaseURL == "" {
		config.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if config.Timeout <= 0 {
		config.Timeout = 20 * time.Second
	}
	if client == nil {
		client = &http.Client{Timeout: config.Timeout}
	}
	return &Service{config: config, client: client}
}

func (s *Service) Generate(ctx context.Context, run domain.AnalysisRun, language string) (domain.AIAssessment, error) {
	if strings.TrimSpace(s.config.APIKey) == "" {
		return domain.AIAssessment{}, ErrNotConfigured
	}
	if language != "th" && language != "en" {
		language = "th"
	}
	payload, err := s.buildRequest(run, language)
	if err != nil {
		return domain.AIAssessment{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.AIAssessment{}, err
	}
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/models/" + url.PathEscape(s.config.Model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.AIAssessment{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.config.APIKey)
	response, err := s.client.Do(req)
	if err != nil {
		return domain.AIAssessment{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return domain.AIAssessment{}, fmt.Errorf("%w: %v", ErrUnavailable, readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.AIAssessment{}, fmt.Errorf("%w: Gemini returned HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var generated geminiResponse
	if err = json.Unmarshal(responseBody, &generated); err != nil || len(generated.Candidates) == 0 || len(generated.Candidates[0].Content.Parts) == 0 {
		return domain.AIAssessment{}, ErrInvalidOutput
	}
	var result generatedAssessment
	if err = json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result); err != nil || !validResult(result) {
		return domain.AIAssessment{}, ErrInvalidOutput
	}
	return domain.AIAssessment{Summary: result.Summary, Recommendation: result.Recommendation, Strengths: result.Strengths, Risks: result.Risks, RequiredChecks: result.RequiredChecks, Disclaimer: result.Disclaimer, Language: language, Model: s.config.Model, GeneratedAt: time.Now().UTC()}, nil
}

// Score asks Gemini to assess only the evidence that the backend has already
// collected. It does not send a precise location, customer details or place
// lists, and its proposed scores are validated again by the analysis service.
func (s *Service) Score(ctx context.Context, run domain.AnalysisRun, language string) (domain.AIScoring, error) {
	if strings.TrimSpace(s.config.APIKey) == "" {
		return domain.AIScoring{}, ErrNotConfigured
	}
	if language != "th" && language != "en" {
		language = "th"
	}
	payload, err := s.buildScoringRequest(run, language)
	if err != nil {
		return domain.AIScoring{}, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return domain.AIScoring{}, err
	}
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/models/" + url.PathEscape(s.config.Model) + ":generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return domain.AIScoring{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", s.config.APIKey)
	response, err := s.client.Do(req)
	if err != nil {
		return domain.AIScoring{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if readErr != nil {
		return domain.AIScoring{}, fmt.Errorf("%w: %v", ErrUnavailable, readErr)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.AIScoring{}, fmt.Errorf("%w: Gemini returned HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var generated geminiResponse
	if err = json.Unmarshal(responseBody, &generated); err != nil || len(generated.Candidates) == 0 || len(generated.Candidates[0].Content.Parts) == 0 {
		return domain.AIScoring{}, ErrInvalidOutput
	}
	var result generatedScoring
	if err = json.Unmarshal([]byte(generated.Candidates[0].Content.Parts[0].Text), &result); err != nil || !validScoringResult(result) {
		return domain.AIScoring{}, ErrInvalidOutput
	}
	metricScores := make(map[string]float64, len(result.MetricScores))
	for _, item := range result.MetricScores {
		metricScores[item.MetricType] = item.Score
	}
	return domain.AIScoring{MetricScores: metricScores, Recommendation: result.Recommendation, Disclaimer: result.Disclaimer, Language: language, Model: s.config.Model, GeneratedAt: time.Now().UTC()}, nil
}

func (s *Service) buildRequest(run domain.AnalysisRun, language string) (geminiRequest, error) {
	facts, err := compactRunFacts(run)
	if err != nil {
		return geminiRequest{}, err
	}
	factJSON, err := json.Marshal(facts)
	if err != nil {
		return geminiRequest{}, err
	}
	languageName := map[string]string{"th": "Thai", "en": "English"}[language]
	prompt := "You are an evidence-bound EV-station analyst. Write in " + languageName + ". Use ONLY the supplied facts. Do not invent location data, demand, traffic, grid capacity, ROI, payback, scores, competitors, or source claims. Do not calculate, alter, or propose numeric scores or ROI. Explain that the result is preliminary where evidence is estimated, preliminary, or missing. Return concise JSON matching the schema.\n\nFACTS:\n" + string(factJSON)
	var request geminiRequest
	request.Contents = append(request.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{Role: "user", Parts: []struct {
		Text string `json:"text"`
	}{{Text: prompt}}})
	request.GenerationConfig.Temperature = 0
	request.GenerationConfig.MaxOutputTokens = 700
	request.GenerationConfig.ResponseMimeType = "application/json"
	request.GenerationConfig.ResponseJSONSchema = assessmentSchema()
	return request, nil
}

func (s *Service) buildScoringRequest(run domain.AnalysisRun, language string) (geminiRequest, error) {
	facts, err := compactRunFacts(run)
	if err != nil {
		return geminiRequest{}, err
	}
	eligible := make([]string, 0, len(run.Metrics))
	for _, metric := range run.Metrics {
		if metric.NormalizedScore != nil {
			eligible = append(eligible, metric.Type)
		}
	}
	facts["eligibleMetricTypes"] = eligible
	factJSON, err := json.Marshal(facts)
	if err != nil {
		return geminiRequest{}, err
	}
	languageName := map[string]string{"th": "Thai", "en": "English"}[language]
	prompt := "You are an evidence-bound EV-station screening scorer. Write in " + languageName + ". Use ONLY the supplied facts and score ONLY eligibleMetricTypes. For each eligible metric, return one metricScores array item with its metricType and a score from 0 to 100 based on observed values, source status and limitations. Do not include missing data or electrical capacity. Do not invent location data, traffic, demand, grid capacity, ROI, payback, competitors or sources. The backend will calculate the overall weighted total; do not provide an overall score. Return concise JSON matching the schema. The disclaimer must state that the result is preliminary and requires human review.\n\nFACTS:\n" + string(factJSON)
	return scoringRequest(prompt), nil
}

func scoringRequest(prompt string) geminiRequest {
	var request geminiRequest
	request.Contents = append(request.Contents, struct {
		Role  string `json:"role"`
		Parts []struct {
			Text string `json:"text"`
		} `json:"parts"`
	}{Role: "user", Parts: []struct {
		Text string `json:"text"`
	}{{Text: prompt}}})
	request.GenerationConfig.Temperature = 0
	request.GenerationConfig.MaxOutputTokens = 700
	request.GenerationConfig.ResponseMimeType = "application/json"
	request.GenerationConfig.ResponseJSONSchema = scoringSchema()
	return request
}

func compactRunFacts(run domain.AnalysisRun) (map[string]any, error) {
	metrics := make([]map[string]any, 0, len(run.Metrics))
	for _, metric := range run.Metrics {
		item := map[string]any{"type": metric.Type, "status": metric.Status, "source": metric.Source.Name, "methodology": metric.Source.Methodology, "assumptions": metric.Assumptions}
		var raw map[string]any
		if len(metric.RawValue) > 0 && json.Unmarshal(metric.RawValue, &raw) == nil {
			delete(raw, "places")
			delete(raw, "sources")
			delete(raw, "latitude")
			delete(raw, "longitude")
			item["observedValues"] = raw
		}
		metrics = append(metrics, item)
	}
	return map[string]any{"assessmentStatus": run.AssessmentStatus, "analysisRadiusMeters": run.AnalysisRadiusMeters, "overallScore": run.OverallScore, "metrics": metrics, "financialAvailable": run.Financial != nil}, nil
}

func assessmentSchema() map[string]any {
	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "maxItems": 5}
	return map[string]any{"type": "object", "properties": map[string]any{"summary": map[string]any{"type": "string"}, "recommendation": map[string]any{"type": "string"}, "strengths": stringArray, "risks": stringArray, "requiredChecks": stringArray, "disclaimer": map[string]any{"type": "string"}}, "required": []string{"summary", "recommendation", "strengths", "risks", "requiredChecks", "disclaimer"}, "additionalProperties": false}
}

func scoringSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{
		"metricScores":   map[string]any{"type": "array", "minItems": 1, "maxItems": 7, "items": map[string]any{"type": "object", "properties": map[string]any{"metricType": map[string]any{"type": "string", "enum": []string{"traffic", "road_accessibility", "ev_demand", "population", "poi", "competition", "flood"}}, "score": map[string]any{"type": "number", "minimum": 0, "maximum": 100}}, "required": []string{"metricType", "score"}, "additionalProperties": false}},
		"recommendation": map[string]any{"type": "string"},
		"disclaimer":     map[string]any{"type": "string"},
	}, "required": []string{"metricScores", "recommendation", "disclaimer"}, "additionalProperties": false}
}

func validResult(value generatedAssessment) bool {
	return strings.TrimSpace(value.Summary) != "" && strings.TrimSpace(value.Recommendation) != "" && strings.TrimSpace(value.Disclaimer) != ""
}

func validScoringResult(value generatedScoring) bool {
	if len(value.MetricScores) == 0 || strings.TrimSpace(value.Recommendation) == "" || strings.TrimSpace(value.Disclaimer) == "" {
		return false
	}
	seen := make(map[string]bool, len(value.MetricScores))
	for _, item := range value.MetricScores {
		if item.MetricType == "" || seen[item.MetricType] || item.Score < 0 || item.Score > 100 {
			return false
		}
		seen[item.MetricType] = true
	}
	return true
}
