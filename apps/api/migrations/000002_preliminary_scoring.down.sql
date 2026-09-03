DELETE FROM scoring_configs WHERE name = 'preliminary-location-screening' AND version = 1;
ALTER TABLE analysis_runs DROP COLUMN scoring_summary;
