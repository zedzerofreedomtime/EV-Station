# Architecture

## Shape

RBC EV STATION is a monorepo and a modular monolith. The React SPA calls a versioned Go/Gin REST API. The API owns validation, provider orchestration, deterministic scoring and deterministic financial calculations. PostgreSQL/PostGIS is the source of truth; Redis is an optional cache, never durable storage.

```text
React SPA -> Go/Gin API -> Site / Analysis / Scoring / Financial modules
                           |                         |
                           v                         v
                    Provider interfaces       PostgreSQL/PostGIS
                           |
                 API / Open Data / GIS / Import
                           |
                         Redis cache
```

## Provider contract

Providers return observations, not conclusions. Every observation includes metric type, raw value, optional normalized score, data status, source metadata and assumptions. The safe default provider returns missing data and no score. Real providers can replace it without changing the analysis service.

AI is outside the calculation path. A future AI provider may explain or summarize stored evidence, but may not create factual measurements, normalized scores, financial values or utility-capacity claims.

## MVP phases

- Days 1–10: monorepo, data model, API, site workflow, analysis orchestration, honest unavailable-provider behavior, frontend result surface, Docker and tests.
- Days 11–30: factual providers, dataset ingestion, scoring calibration, financial assumptions, provider/cache tests, deployment and operational hardening.
