ALTER TABLE analysis_runs ADD COLUMN scoring_summary JSONB;

INSERT INTO scoring_configs (name, version, weights, methodology, status)
VALUES (
  'preliminary-location-screening',
  1,
  '{"traffic":0.25,"ev_demand":0.20,"population":0.15,"poi":0.15,"competition":0.15,"flood":0.05,"electrical":0.05}',
  'Deterministic preliminary-v1 normalization. Missing metrics are excluded, available weights are renormalized, and at least 60 percent weighted coverage is required. Public electrical planning data is not scored without utility-confirmed site capacity.',
  'provisional'
);
