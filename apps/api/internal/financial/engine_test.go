package financial

import "testing"

func TestCalculate(t *testing.T) {
	result, err := Calculate(Input{InitialInvestment: 1_200_000, MonthlyRevenue: 150_000, MonthlyOperatingCost: 50_000})
	if err != nil {
		t.Fatal(err)
	}
	if result.MonthlyProfit != 100_000 || result.AnnualProfit != 1_200_000 {
		t.Fatalf("unexpected profit: %+v", result)
	}
	if result.ROIPercentage == nil || *result.ROIPercentage != 100 {
		t.Fatalf("unexpected ROI: %+v", result.ROIPercentage)
	}
	if result.PaybackMonths == nil || *result.PaybackMonths != 12 {
		t.Fatalf("unexpected payback: %+v", result.PaybackMonths)
	}
}

func TestNoPaybackForNonPositiveProfit(t *testing.T) {
	result, err := Calculate(Input{InitialInvestment: 1_000, MonthlyRevenue: 100, MonthlyOperatingCost: 100})
	if err != nil {
		t.Fatal(err)
	}
	if result.PaybackMonths != nil {
		t.Fatal("payback must be unavailable when monthly profit is non-positive")
	}
}
