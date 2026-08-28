# Financial model

Calculations run in Go and never in AI output.

## Franchise plan selection

The S, M and L plan catalogue is transcribed from the project-owner supplied
franchise image and is labelled `user_supplied`. It preserves the recommended
area, mapped EV charging-station count, investment range and starting franchise
fee. The source does not state whether the fee is included in the investment
range, so the system never adds the two values together.

Selecting a plan does not produce ROI or payback. Those results remain absent
until approved utilisation, energy, tariff, price, rent and operating-cost
assumptions are supplied.

```text
monthly_profit = monthly_revenue - monthly_operating_cost
annual_profit = monthly_profit * 12
roi_percentage = annual_profit / initial_investment * 100
payback_months = initial_investment / monthly_profit
```

Initial investment must be positive. Payback is unavailable when monthly profit is zero or negative. Results must retain the assumptions used for investment, utilization, tariff, pricing, operating cost, tax treatment and analysis date. The current formula is pre-tax and does not claim a guaranteed return.
