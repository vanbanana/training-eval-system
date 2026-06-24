-- ============================================================
-- Add evaluation columns that the model/scorer rely on:
--   objective_ratio: AI objective weight (alpha), default 0.6
--   overall_comment: system-level comment (e.g. AI failure reason)
-- These were previously present on the Go model but missing in the schema,
-- causing EvaluationRepo.Update to silently drop the values.
-- ============================================================
ALTER TABLE evaluations ADD COLUMN objective_ratio REAL;
ALTER TABLE evaluations ADD COLUMN overall_comment TEXT NOT NULL DEFAULT '';
