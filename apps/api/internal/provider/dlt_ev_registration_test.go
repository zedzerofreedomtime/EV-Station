package provider

import "testing"

func TestParseDLTBEVGroupsElectricVehiclesByProvince(t *testing.T) {
	payload := []byte("ปี,ประเภทรถ,จังหวัด,ชนิดเชื้อเพลิง,จำนวน\n2569,รถยนต์นั่ง,เชียงใหม่,ไฟฟ้า,120\n2569,รถจักรยานยนต์,เชียงใหม่,ไฟฟ้า,30\n2569,รถยนต์นั่ง,เชียงใหม่,ไฮบริด,999\n2569,รถยนต์นั่ง,ลำพูน,ไฟฟ้า,45\n")
	values, err := parseDLTBEV(payload)
	if err != nil {
		t.Fatalf("parseDLTBEV returned error: %v", err)
	}
	if values["เชียงใหม่"] != 150 {
		t.Fatalf("เชียงใหม่ = %d, want 150", values["เชียงใหม่"])
	}
	if values["ลำพูน"] != 45 {
		t.Fatalf("ลำพูน = %d, want 45", values["ลำพูน"])
	}
}

func TestRegistrationTrend(t *testing.T) {
	tests := []struct {
		name              string
		current, previous int
		want              string
	}{
		{name: "increase", current: 150, previous: 100, want: "increased"},
		{name: "decrease", current: 80, previous: 100, want: "decreased"},
		{name: "same", current: 100, previous: 100, want: "unchanged"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := registrationTrend(test.current, test.previous); got != test.want {
				t.Fatalf("registrationTrend(%d, %d) = %q, want %q", test.current, test.previous, got, test.want)
			}
		})
	}
	if got := roundDLTPercent(18.449); got != 18.4 {
		t.Fatalf("roundDLTPercent = %v, want 18.4", got)
	}
}

func TestParseDLTIntegerRecoversLargeXLSNumberRenderedAsDate(t *testing.T) {
	value, found := parseDLTInteger("1754-04-14T00:25:26Z")
	if !found {
		t.Fatal("expected date-formatted XLS value to be parsed")
	}
	if value != 160284 {
		t.Fatalf("parsed value = %d, want 160284", value)
	}
}
