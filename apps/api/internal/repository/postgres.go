package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rbc/ev-station/apps/api/internal/domain"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(ctx context.Context, databaseURL string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Postgres{pool: pool}, nil
}

func (p *Postgres) Close() { p.pool.Close() }

const siteColumns = `id, name, address, latitude, longitude, land_size, land_size_unit, google_maps_url, notes, input_status, created_at, updated_at`

func scanSite(row pgx.Row) (domain.Site, error) {
	var site domain.Site
	err := row.Scan(&site.ID, &site.Name, &site.Address, &site.Latitude, &site.Longitude, &site.LandSize, &site.LandSizeUnit, &site.GoogleMapsURL, &site.Notes, &site.InputStatus, &site.CreatedAt, &site.UpdatedAt)
	return site, err
}

func (p *Postgres) CreateSite(ctx context.Context, site domain.Site) (domain.Site, error) {
	query := `INSERT INTO sites (id,name,address,latitude,longitude,location,land_size,land_size_unit,google_maps_url,notes,input_status,created_at,updated_at)
	VALUES ($1,$2,$3,$4,$5,CASE WHEN $4::double precision IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($5,$4),4326)::geography END,$6,$7,$8,$9,$10,$11,$12)
	RETURNING ` + siteColumns
	return scanSite(p.pool.QueryRow(ctx, query, site.ID, site.Name, site.Address, site.Latitude, site.Longitude, site.LandSize, site.LandSizeUnit, site.GoogleMapsURL, site.Notes, site.InputStatus, site.CreatedAt, site.UpdatedAt))
}

func (p *Postgres) ListSites(ctx context.Context) ([]domain.Site, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+siteColumns+` FROM sites ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []domain.Site
	for rows.Next() {
		site, scanErr := scanSite(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sites = append(sites, site)
	}
	return sites, rows.Err()
}

func (p *Postgres) GetSite(ctx context.Context, id uuid.UUID) (domain.Site, error) {
	site, err := scanSite(p.pool.QueryRow(ctx, `SELECT `+siteColumns+` FROM sites WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Site{}, ErrNotFound
	}
	return site, err
}

func (p *Postgres) CreateAnalysis(ctx context.Context, run domain.AnalysisRun) (domain.AnalysisRun, error) {
	_, err := p.pool.Exec(ctx, `INSERT INTO analysis_runs (id,site_id,status,analysis_radius_meters,assessment_status,recommendation,started_at,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`, run.ID, run.SiteID, run.Status, run.AnalysisRadiusMeters, run.AssessmentStatus, run.Recommendation, run.StartedAt, run.CreatedAt)
	return run, err
}

func (p *Postgres) CompleteAnalysis(ctx context.Context, run domain.AnalysisRun) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	financialJSON, _ := json.Marshal(run.Financial)
	_, err = tx.Exec(ctx, `UPDATE analysis_runs SET status=$2,overall_score=$3,assessment_status=$4,recommendation=$5,financial_result=$6,completed_at=$7 WHERE id=$1`, run.ID, run.Status, run.OverallScore, run.AssessmentStatus, run.Recommendation, financialJSON, run.CompletedAt)
	if err != nil {
		return err
	}
	for _, metric := range run.Metrics {
		sourceJSON, _ := json.Marshal(metric.Source)
		assumptionsJSON, _ := json.Marshal(metric.Assumptions)
		_, err = tx.Exec(ctx, `INSERT INTO analysis_metrics (id,analysis_run_id,metric_type,raw_value,normalized_score,data_status,source,assumptions,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, metric.ID, metric.AnalysisRunID, metric.Type, metric.RawValue, metric.NormalizedScore, metric.Status, sourceJSON, assumptionsJSON, metric.CreatedAt)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (p *Postgres) GetAnalysis(ctx context.Context, id uuid.UUID) (domain.AnalysisRun, error) {
	var run domain.AnalysisRun
	var financialJSON []byte
	err := p.pool.QueryRow(ctx, `SELECT id,site_id,status,analysis_radius_meters,overall_score,assessment_status,recommendation,financial_result,started_at,completed_at,created_at FROM analysis_runs WHERE id=$1`, id).Scan(&run.ID, &run.SiteID, &run.Status, &run.AnalysisRadiusMeters, &run.OverallScore, &run.AssessmentStatus, &run.Recommendation, &financialJSON, &run.StartedAt, &run.CompletedAt, &run.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.AnalysisRun{}, ErrNotFound
	}
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	if len(financialJSON) > 0 && string(financialJSON) != "null" {
		_ = json.Unmarshal(financialJSON, &run.Financial)
	}
	rows, err := p.pool.Query(ctx, `SELECT id,analysis_run_id,metric_type,raw_value,normalized_score,data_status,source,assumptions,created_at FROM analysis_metrics WHERE analysis_run_id=$1 ORDER BY created_at`, id)
	if err != nil {
		return domain.AnalysisRun{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var metric domain.Metric
		var sourceJSON, assumptionsJSON []byte
		if err = rows.Scan(&metric.ID, &metric.AnalysisRunID, &metric.Type, &metric.RawValue, &metric.NormalizedScore, &metric.Status, &sourceJSON, &assumptionsJSON, &metric.CreatedAt); err != nil {
			return domain.AnalysisRun{}, err
		}
		_ = json.Unmarshal(sourceJSON, &metric.Source)
		_ = json.Unmarshal(assumptionsJSON, &metric.Assumptions)
		run.Metrics = append(run.Metrics, metric)
	}
	return run, rows.Err()
}
