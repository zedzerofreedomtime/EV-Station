CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE data_status AS ENUM ('verified', 'estimated', 'preliminary', 'missing');
CREATE TYPE analysis_status AS ENUM ('pending', 'running', 'completed', 'failed');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(320) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sites (
    id UUID PRIMARY KEY,
    name VARCHAR(160) NOT NULL,
    address TEXT NOT NULL DEFAULT '',
    latitude DOUBLE PRECISION,
    longitude DOUBLE PRECISION,
    location GEOGRAPHY(POINT, 4326),
    land_size NUMERIC(14, 2) NOT NULL CHECK (land_size > 0),
    land_size_unit VARCHAR(16) NOT NULL CHECK (land_size_unit IN ('sqm', 'rai', 'ngan', 'sqwah')),
    google_maps_url TEXT NOT NULL DEFAULT '',
    notes TEXT NOT NULL DEFAULT '',
    input_status data_status NOT NULL DEFAULT 'preliminary',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((latitude IS NULL) = (longitude IS NULL)),
    CHECK (latitude IS NULL OR latitude BETWEEN -90 AND 90),
    CHECK (longitude IS NULL OR longitude BETWEEN -180 AND 180),
    CHECK (address <> '' OR location IS NOT NULL)
);
CREATE INDEX sites_location_gix ON sites USING GIST (location);
CREATE INDEX sites_created_at_idx ON sites (created_at DESC);

CREATE TABLE analysis_runs (
    id UUID PRIMARY KEY,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    status analysis_status NOT NULL DEFAULT 'pending',
    analysis_radius_meters INTEGER NOT NULL CHECK (analysis_radius_meters IN (1000, 3000, 5000)),
    overall_score NUMERIC(5, 2) CHECK (overall_score BETWEEN 0 AND 100),
    assessment_status data_status NOT NULL DEFAULT 'preliminary',
    recommendation TEXT NOT NULL DEFAULT '',
    financial_result JSONB,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX analysis_runs_site_created_idx ON analysis_runs (site_id, created_at DESC);

CREATE TABLE analysis_metrics (
    id UUID PRIMARY KEY,
    analysis_run_id UUID NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    metric_type VARCHAR(64) NOT NULL,
    raw_value JSONB,
    normalized_score NUMERIC(5, 2) CHECK (normalized_score BETWEEN 0 AND 100),
    data_status data_status NOT NULL,
    source JSONB NOT NULL,
    assumptions JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (analysis_run_id, metric_type)
);
CREATE INDEX analysis_metrics_run_idx ON analysis_metrics (analysis_run_id);
CREATE INDEX analysis_metrics_source_gin ON analysis_metrics USING GIN (source);

CREATE TABLE poi_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), analysis_run_id UUID NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    provider_id TEXT, name TEXT NOT NULL, category TEXT, location GEOGRAPHY(POINT, 4326) NOT NULL,
    data_status data_status NOT NULL, source JSONB NOT NULL, observed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX poi_records_location_gix ON poi_records USING GIST (location);

CREATE TABLE competitor_chargers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), analysis_run_id UUID NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    provider_id TEXT, name TEXT NOT NULL, operator TEXT, connector_data JSONB NOT NULL DEFAULT '[]'::jsonb,
    location GEOGRAPHY(POINT, 4326) NOT NULL, data_status data_status NOT NULL, source JSONB NOT NULL,
    observed_at TIMESTAMPTZ, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX competitor_chargers_location_gix ON competitor_chargers USING GIST (location);

CREATE TABLE scoring_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), name TEXT NOT NULL, version INTEGER NOT NULL,
    weights JSONB NOT NULL, methodology TEXT NOT NULL, status VARCHAR(24) NOT NULL DEFAULT 'provisional',
    effective_from TIMESTAMPTZ NOT NULL DEFAULT now(), created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE(name, version)
);

CREATE TABLE financial_assumptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), analysis_run_id UUID NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    values JSONB NOT NULL, source JSONB NOT NULL, data_status data_status NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE financial_results (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), analysis_run_id UUID NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    values JSONB NOT NULL, assumptions_snapshot JSONB NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE external_api_cache (
    cache_key TEXT PRIMARY KEY, provider TEXT NOT NULL, request_fingerprint TEXT NOT NULL,
    response JSONB NOT NULL, retrieved_at TIMESTAMPTZ NOT NULL, expires_at TIMESTAMPTZ NOT NULL,
    source_metadata JSONB NOT NULL
);
CREATE INDEX external_api_cache_expiry_idx ON external_api_cache (expires_at);

INSERT INTO scoring_configs (name, version, weights, methodology, status)
VALUES ('default-location-score', 1,
 '{"traffic":0.25,"ev_demand":0.20,"population":0.15,"poi":0.15,"competition":0.15,"flood":0.05,"electrical":0.05}',
 'Weighted average of normalized 0-100 metrics. Configuration is provisional and must be validated by business experts.',
 'provisional');
