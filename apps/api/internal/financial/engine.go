package financial

import (
	"errors"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

var ErrInvalidInvestment = errors.New("initial investment must be greater than zero")

type Input struct {
	InitialInvestment    float64  `json:"initialInvestment" binding:"required,gt=0"`
	MonthlyRevenue       float64  `json:"monthlyRevenue" binding:"gte=0"`
	MonthlyOperatingCost float64  `json:"monthlyOperatingCost" binding:"gte=0"`
	Assumptions          []string `json:"assumptions"`
}

func Calculate(input Input) (domain.FinancialResult, error) {
	if input.InitialInvestment <= 0 {
		return domain.FinancialResult{}, ErrInvalidInvestment
	}
	monthlyProfit := input.MonthlyRevenue - input.MonthlyOperatingCost
	annualProfit := monthlyProfit * 12
	result := domain.FinancialResult{
		InitialInvestment:    input.InitialInvestment,
		MonthlyRevenue:       input.MonthlyRevenue,
		MonthlyOperatingCost: input.MonthlyOperatingCost,
		MonthlyProfit:        monthlyProfit,
		AnnualProfit:         annualProfit,
		Assumptions:          input.Assumptions,
	}
	roi := annualProfit / input.InitialInvestment * 100
	result.ROIPercentage = &roi
	if monthlyProfit > 0 {
		payback := input.InitialInvestment / monthlyProfit
		result.PaybackMonths = &payback
	}
	return result, nil
}
