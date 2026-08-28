# Database

PostgreSQL with PostGIS is required from the first migration. `sites.location` is `GEOGRAPHY(POINT, 4326)` with a GiST index.

Core tables:

- `sites`: customer-supplied location and land details.
- `analysis_runs`: immutable analysis attempts and summary state.
- `analysis_metrics`: one row per dimension with raw data, normalized score, status, source JSON and assumptions JSON.
- `poi_records`, `competitor_chargers`: spatial evidence with independent provenance.
- `scoring_configs`: versioned weights and methodology.
- `financial_assumptions`, `financial_results`: explicit inputs and reproducible outputs.
- `external_api_cache`: optional persisted provider response metadata. Redis remains the fast ephemeral cache.

The schema distinguishes `verified`, `estimated`, `preliminary` and `missing`. Electrical readiness is never considered verified unless the source metadata references actual utility evidence.

Apply migrations with `golang-migrate` or the Compose `migrate` service. Down migration removes the initial schema and is intended only for disposable development databases.
