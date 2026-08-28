package financial

import "testing"

func TestFranchisePlansPreserveUserSuppliedRangesWithoutInventingROI(t *testing.T) {
	plans := FranchisePlans()
	if len(plans) != 3 {
		t.Fatalf("expected three plans, got %d", len(plans))
	}
	large, err := GetFranchisePlan("l")
	if err != nil {
		t.Fatal(err)
	}
	if large.InvestmentMaxTHB != nil {
		t.Fatal("L plan maximum must remain open-ended because the source says 10 million or more")
	}
	if large.InvestmentUpperReferenceTHB == nil || *large.InvestmentUpperReferenceTHB != 10_000_000 || !large.InvestmentOpenEnded {
		t.Fatal("L plan must preserve the 10 million reference and its open-ended wording")
	}
	if large.ROIAvailable || len(large.MissingForROI) == 0 {
		t.Fatal("ROI must remain unavailable until explicit operating assumptions exist")
	}
}
