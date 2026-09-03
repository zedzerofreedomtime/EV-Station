# RBC EV STATION

RBC EV STATION is a B2B location-intelligence application for screening candidate land before an EV charging station franchise investment decision.

The foundation is a modular monolith in a monorepo:

- `apps/web`: React, TypeScript, Vite and Tailwind CSS
- `apps/api`: Go and Gin REST API
- PostgreSQL + PostGIS: durable records and spatial data
- Redis: optional external-provider cache
- `docs`: architecture, schema, API and calculation rules

## Data integrity principles

The system does not use AI to invent location facts, scores or financial values. Every metric stores a data status, source metadata and assumptions. Without a factual provider, the default analysis completes with missing metrics and no score. An explicitly enabled development fixture is available only to test the workflow and is always labelled preliminary.

## External data providers

Docker Compose enables free OpenStreetMap, WorldPop, GISTDA and Department of Highways providers for MVP validation. It queries OpenStreetMap through the Overpass API for tagged POIs and mapped EV charging stations, WorldPop for modelled population, GISTDA's published flood-risk GIS layer, and the Department of Highways AADT CSV plus matched public road control-section geometry around coordinates supplied by the user.

- Results preserve the source URL, retrieval time, methodology, ODbL licence and coverage assumptions.
- Returned counts are factual observations from the query response, but OpenStreetMap coverage may be incomplete.
- The provider does not generate normalized POI or competition scores.
- Traffic uses a verified AADT value only when its official road number and control section match the closest public DOH road geometry; otherwise it remains an OSM accessibility proxy. Population remains a modelled estimate, flood remains a published-layer overlap observation, and EV demand/electrical capacity remain missing until factual providers are configured.
- Responses are cached in Redis under `rbc:external:osm:*` with a finite TTL.

Set `ANALYSIS_PROVIDER_MODE=unavailable` to disable external collection or `fixture` in a non-production environment for deterministic workflow testing.

The site form also provides a manual address search backed by OpenStreetMap Nominatim. It is deliberately user-triggered rather than autocomplete, rate-limited to one outgoing public request per second, and cached for 30 days. Selected coordinates remain preliminary customer inputs until confirmed.

The web map falls back to a free OpenStreetMap preview when no Google Maps browser key is configured. Paid providers remain disabled. See `docs/DATA_SOURCES.md` and the in-app **Data Sources** page for the free-key backlog and the separate paid/manual provider list.

## Run with Docker

Requirements: Docker Desktop with Compose.

```bash
docker compose up --build
```

Open `http://localhost:8081`. API health is at `http://localhost:8080/health`.

To reset local development data, stop the stack and explicitly remove its named volumes with `docker compose down -v`.

## Run locally

Frontend requires Node.js and pnpm:

```bash
pnpm install
pnpm dev:web
```

Backend requires Go 1.23+ and PostgreSQL/PostGIS. Copy `.env.example` to `.env`, run the migration, then:

```bash
cd apps/api
go run ./cmd/api
```

If `DATABASE_URL` is absent, the API uses a non-persistent in-memory repository and logs a warning. Redis is optional and must never prevent local startup.

## Tests

```bash
pnpm test:web
cd apps/api && go test ./...
```

## Core workflow

1. Create a candidate site from an address or coordinates.
2. Save the site with user-supplied provenance.
3. Run analysis.
4. Collect provider observations (OpenStreetMap POIs and mapped charging stations, WorldPop population estimates, and GISTDA flood-risk layer overlap in Docker Compose; unavailable remains the safe configuration default).
5. Calculate a score only when every required metric has an explicit score.
6. Store the result, source metadata and assumptions.
7. Display an honest result with verified, estimated, preliminary and missing states.

See [Architecture](docs/ARCHITECTURE.md), [Database](docs/DATABASE.md), [API](docs/API.md), [Scoring](docs/SCORING.md), and [Financial model](docs/FINANCIAL_MODEL.md).
