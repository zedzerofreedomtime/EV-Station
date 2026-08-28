# API

Base path: `/api/v1`. Responses use `{ "data": ... }`; errors use `{ "error": { "code": "...", "message": "..." } }`.

| Method | Path | Purpose |
|---|---|---|
| GET | `/health` | Liveness check (outside versioned base) |
| POST | `/api/v1/sites` | Create a site |
| GET | `/api/v1/sites` | List sites |
| GET | `/api/v1/sites/:id` | Get a site |
| POST | `/api/v1/sites/:id/analyses` | Run analysis at 1, 3 or 5 km |
| GET | `/api/v1/analyses/:id` | Get result with provenance |
| GET | `/api/v1/geocoding/search?q=...&limit=5` | User-triggered Thailand address search via cached Nominatim |
| GET | `/api/v1/data-sources` | Provider cost, readiness, provenance and quality catalogue |
| POST | `/api/v1/financial/calculate` | Deterministic financial calculation |
| GET | `/api/v1/scoring/config` | Current provisional weights |

Authentication routes and enforcement are intentionally not blocking the first site-to-analysis workflow. JWT architecture is reserved for the next phase.

The geocoding endpoint is intentionally not an autocomplete endpoint. Queries must contain 3–200 characters, are limited to five results, are cached, and the API process enforces at most one outgoing public Nominatim request per second.
