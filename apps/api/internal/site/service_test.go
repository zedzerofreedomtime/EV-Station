package site

import "testing"

func TestValidateLocation(t *testing.T) {
	lat, lng := 13.7563, 100.5018
	cases := []struct {
		name     string
		address  string
		lat, lng *float64
		wantErr  bool
	}{
		{"address", "Bangkok", nil, nil, false},
		{"coordinates", "", &lat, &lng, false},
		{"missing", "", nil, nil, true},
		{"partial", "", &lat, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateLocation(tc.address, tc.lat, tc.lng)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
