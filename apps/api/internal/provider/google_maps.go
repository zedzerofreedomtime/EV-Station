package provider

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidGoogleMapsURL          = errors.New("invalid Google Maps URL")
	ErrGoogleMapsCoordinatesNotFound = errors.New("Google Maps coordinates not found")
)

type GoogleMapsResolution struct {
	InputURL    string  `json:"inputUrl"`
	ResolvedURL string  `json:"resolvedUrl"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

var coordinatePairPattern = regexp.MustCompile(`(-?\d{1,3}(?:\.\d+)?)\s*[,/]\s*(-?\d{1,3}(?:\.\d+)?)`)
var encodedCoordinatePattern = regexp.MustCompile(`!3d(-?\d{1,3}(?:\.\d+)?)!4d(-?\d{1,3}(?:\.\d+)?)`)
var googleMapsPageTitlePattern = regexp.MustCompile(`(?is)<title>\s*(.*?)\s*-\s*Google Maps\s*</title>`)
var parentheticalTextPattern = regexp.MustCompile(`\([^)]*\)`)
var googleMapsAddressStartPattern = regexp.MustCompile(`(?i)\s+(?:\d+\s|หมู่ที่|ถนน|ซอย|ตำบล|อำเภอ|จังหวัด)`)

// ResolveGoogleMapsURL follows only Google Maps hosts and extracts a coordinate pair from the final URL.
// When a Maps short link ends in a Place ID without coordinates, it uses the supplied free geocoder
// to find a preliminary matching pin. It deliberately never uses Google's page-preview map centre,
// which can be far away from the selected place.
func ResolveGoogleMapsURL(ctx context.Context, raw string, geocoder Geocoder) (GoogleMapsResolution, error) {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || !isGoogleMapsHost(parsed.Hostname()) {
		return GoogleMapsResolution{}, ErrInvalidGoogleMapsURL
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(req *http.Request, _ []*http.Request) error {
		if req.URL.Scheme != "https" || !isGoogleMapsHost(req.URL.Hostname()) {
			return ErrInvalidGoogleMapsURL
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return GoogleMapsResolution{}, ErrInvalidGoogleMapsURL
	}
	req.Header.Set("User-Agent", "RBC-EV-Station/1.0")
	response, err := client.Do(req)
	if err != nil {
		return GoogleMapsResolution{}, fmt.Errorf("resolve Google Maps URL: %w", err)
	}
	defer response.Body.Close()
	if response.Request == nil || response.Request.URL == nil {
		return GoogleMapsResolution{}, ErrGoogleMapsCoordinatesNotFound
	}
	finalURL := response.Request.URL.String()
	latitude, longitude, ok := extractGoogleMapsCoordinates(finalURL)
	if !ok {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
		if readErr != nil {
			return GoogleMapsResolution{}, fmt.Errorf("read Google Maps destination: %w", readErr)
		}
		latitude, longitude, ok = resolveGoogleMapsPlaceCoordinates(ctx, finalURL, string(body), geocoder)
	}
	if !ok {
		return GoogleMapsResolution{}, ErrGoogleMapsCoordinatesNotFound
	}
	return GoogleMapsResolution{InputURL: raw, ResolvedURL: finalURL, Latitude: latitude, Longitude: longitude}, nil
}

func resolveGoogleMapsPlaceCoordinates(ctx context.Context, finalURL, page string, geocoder Geocoder) (float64, float64, bool) {
	if geocoder == nil {
		return 0, 0, false
	}
	name := googleMapsPlaceName(page)
	if name == "" {
		name = googleMapsPlaceNameFromURL(finalURL)
	}
	if name == "" {
		return 0, 0, false
	}
	results, err := geocoder.Search(ctx, name, 5)
	if err != nil || len(results) == 0 {
		return 0, 0, false
	}
	return chooseGoogleMapsPlaceMatch(results, finalURL)
}

func googleMapsPlaceNameFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return ""
	}
	marker := "/maps/place/"
	start := strings.Index(path, marker)
	if start < 0 {
		return ""
	}
	placePath := strings.Split(strings.TrimPrefix(path[start:], marker), "/")[0]
	placePath = strings.Join(strings.Fields(strings.ReplaceAll(placePath, "+", " ")), " ")
	if boundary := googleMapsAddressStartPattern.FindStringIndex(placePath); boundary != nil {
		placePath = placePath[:boundary[0]]
	}
	placePath = strings.TrimSpace(parentheticalTextPattern.ReplaceAllString(placePath, ""))
	return placePath
}

func googleMapsPlaceName(page string) string {
	match := googleMapsPageTitlePattern.FindStringSubmatch(page)
	if len(match) != 2 {
		return ""
	}
	name := strings.Join(strings.Fields(html.UnescapeString(match[1])), " ")
	if simplified := strings.TrimSpace(parentheticalTextPattern.ReplaceAllString(name, "")); simplified != "" {
		return simplified
	}
	return name
}

func chooseGoogleMapsPlaceMatch(results []GeocodingResult, finalURL string) (float64, float64, bool) {
	decoded, err := url.PathUnescape(finalURL)
	if err != nil {
		decoded = finalURL
	}
	decoded = strings.ToLower(strings.ReplaceAll(decoded, "+", " "))
	bestIndex, bestScore := -1, -1
	for index, result := range results {
		displayName := strings.ToLower(result.DisplayName)
		score := 0
		for _, token := range strings.Fields(decoded) {
			if len([]rune(token)) >= 4 && strings.Contains(displayName, token) {
				score += len([]rune(token))
			}
		}
		if score > bestScore {
			bestIndex, bestScore = index, score
		}
	}
	if bestIndex < 0 || bestScore == 0 {
		return 0, 0, false
	}
	return results[bestIndex].Latitude, results[bestIndex].Longitude, true
}

func isGoogleMapsHost(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "maps.app.goo.gl" || host == "goo.gl" || host == "maps.google.com" || host == "www.google.com" || host == "google.com" || strings.HasSuffix(host, ".google.com")
}
func extractGoogleMapsCoordinates(value string) (float64, float64, bool) {
	for _, match := range encodedCoordinatePattern.FindAllStringSubmatch(value, -1) {
		if lat, lon, ok := validCoordinatePair(match[1], match[2]); ok {
			return lat, lon, true
		}
	}
	for _, match := range coordinatePairPattern.FindAllStringSubmatch(value, -1) {
		if lat, lon, ok := validCoordinatePair(match[1], match[2]); ok {
			return lat, lon, true
		}
	}
	return 0, 0, false
}
func validCoordinatePair(latValue, lonValue string) (float64, float64, bool) {
	lat, latErr := strconv.ParseFloat(latValue, 64)
	lon, lonErr := strconv.ParseFloat(lonValue, 64)
	return lat, lon, latErr == nil && lonErr == nil && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180
}
