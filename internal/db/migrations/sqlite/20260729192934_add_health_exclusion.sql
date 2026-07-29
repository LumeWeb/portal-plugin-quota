-- +goose Up
ALTER TABLE quota_plans ADD COLUMN excluded_from_health_reports BOOLEAN DEFAULT FALSE;
ALTER TABLE user_quota_configs ADD COLUMN excluded_from_health_reports BOOLEAN DEFAULT FALSE;

-- +goose Down
ALTER TABLE quota_plans DROP COLUMN excluded_from_health_reports;
ALTER TABLE user_quota_configs DROP COLUMN excluded_from_health_reports;
