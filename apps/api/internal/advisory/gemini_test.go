package advisory

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func TestGeminiServiceDoesNotSendCoordinatesOrPlaceLists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Fatalf("expected server-side API key header")
		}
		body, _ := io.ReadAll(request.Body)
		text := string(body)
		if strings.Contains(text, "secret-place") || strings.Contains(text, "13.7563") {
			t.Fatalf("sensitive raw fields must not be sent to Gemini: %s", text)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"summary\":\"สรุปจากข้อมูลที่มี\",\"recommendation\":\"ตรวจสอบระบบไฟฟ้าเพิ่มเติม\",\"strengths\":[\"มีข้อมูลจราจร\"],\"risks\":[\"ข้อมูลไฟฟ้าเป็นเบื้องต้น\"],\"requiredChecks\":[\"ยืนยันกับการไฟฟ้า\"],\"disclaimer\":\"ไม่ใช่คะแนนหรือผลตอบแทน\"}"}]}}]}`))
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	result, err := service.Generate(context.Background(), domain.AnalysisRun{AssessmentStatus: domain.DataPreliminary, AnalysisRadiusMeters: 3000, Metrics: []domain.Metric{{Type: "competition", Status: domain.DataVerified, Source: domain.DataSource{Name: "OSM"}, RawValue: []byte(`{"count":1,"latitude":13.7563,"places":[{"name":"secret-place"}]}`)}}}, "th")
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "test-model" || result.Language != "th" || result.Summary == "" {
		t.Fatalf("unexpected assessment: %+v", result)
	}
}

func TestGeminiServiceRequiresAPIKey(t *testing.T) {
	service := NewGeminiService(GeminiConfig{}, nil)
	_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "th")
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestGeminiServiceScoresOnlyEvidenceWithoutCoordinates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		text := string(body)
		if strings.Contains(text, "13.7563") || strings.Contains(text, "secret-place") {
			t.Fatalf("sensitive raw fields must not be sent to Gemini: %s", text)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"{\"metricScores\":[{\"metricType\":\"traffic\",\"score\":71}],\"recommendation\":\"ตรวจสอบทางเข้าออกเพิ่มเติม\",\"disclaimer\":\"เป็นผลคัดกรองเบื้องต้น ต้องตรวจสอบโดยพนักงาน\"}"}]}}]}`))
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: time.Second}, server.Client())
	score := 65.0
	result, err := service.Score(context.Background(), domain.AnalysisRun{AssessmentStatus: domain.DataPreliminary, AnalysisRadiusMeters: 3000, Metrics: []domain.Metric{{Type: "traffic", Status: domain.DataVerified, NormalizedScore: &score, Source: domain.DataSource{Name: "DOH"}, RawValue: []byte(`{"aadt":50000,"latitude":13.7563,"places":[{"name":"secret-place"}]}`)}}}, "th")
	if err != nil {
		t.Fatal(err)
	}
	if result.MetricScores["traffic"] != 71 || result.Model != "test-model" {
		t.Fatalf("unexpected scoring result: %+v", result)
	}
}
