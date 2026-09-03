package provider

import "testing"

func TestExtractGoogleMapsCoordinates(t *testing.T) {
	cases := []struct {
		name, value         string
		latitude, longitude float64
	}{
		{"at coordinate", "https://www.google.com/maps/@13.7563,100.5018,15z", 13.7563, 100.5018},
		{"encoded coordinate", "https://www.google.com/maps/data=!3d13.8716898!4d100.6213684", 13.8716898, 100.6213684},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			latitude, longitude, ok := extractGoogleMapsCoordinates(tc.value)
			if !ok || latitude != tc.latitude || longitude != tc.longitude {
				t.Fatalf("got (%v, %v, %v), want (%v, %v, true)", latitude, longitude, ok, tc.latitude, tc.longitude)
			}
		})
	}
}

func TestChooseGoogleMapsPlaceMatchUsesAddressContext(t *testing.T) {
	results := []GeocodingResult{
		{DisplayName: "วัดพระงาม, ตำบลภูเขาทอง, อำเภอพระนครศรีอยุธยา", Latitude: 14.3664714, Longitude: 100.5310371},
		{DisplayName: "วัดพระงาม, บ้านคลองสระบัว, ตำบลคลองสระบัว, อำเภอพระนครศรีอยุธยา", Latitude: 14.3710508, Longitude: 100.5554510},
	}
	url := "https://www.google.com/maps/place/วัดพระงาม+24+หมู่ที่+4+ตำบลคลองสระบัว+อำเภอพระนครศรีอยุธยา"
	latitude, longitude, ok := chooseGoogleMapsPlaceMatch(results, url)
	if !ok || latitude != 14.3710508 || longitude != 100.5554510 {
		t.Fatalf("got (%v, %v, %v), want (14.3710508, 100.5554510, true)", latitude, longitude, ok)
	}
}

func TestGoogleMapsPlaceName(t *testing.T) {
	page := `<title>วัดพระงาม (ประตูแห่งกาลเวลา) - Google Maps</title>`
	if got := googleMapsPlaceName(page); got != "วัดพระงาม" {
		t.Fatalf("got %q, want %q", got, "วัดพระงาม")
	}
}

func TestIsGoogleMapsHost(t *testing.T) {
	if !isGoogleMapsHost("maps.app.goo.gl") || !isGoogleMapsHost("www.google.com") || isGoogleMapsHost("example.com") {
		t.Fatal("Google Maps host validation did not match expected hosts")
	}
}
