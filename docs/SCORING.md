# Scoring

The scoring engine is deterministic. Scores are normalized to 0–100 and combined by a versioned weighted average.

Initial provisional weights: Traffic 25%, EV Demand 20%, Population 15%, POI 15%, Competition 15%, Flood 5%, Electrical 5%.

The engine refuses to calculate if any required normalized metric is absent or outside 0–100. The API therefore returns no overall score when factual provider data is missing. Development fixtures are opt-in, non-production, clearly preliminary, and are not evidence.

Business experts must validate normalization rules, directionality (especially competition and flood risk), thresholds and weights before production use.
