-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_quotas (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    date DATE NOT NULL,
    bytes_uploaded INTEGER DEFAULT 0,
    bytes_downloaded INTEGER DEFAULT 0,
    bytes_stored INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS user_usage_details (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    upload_id INTEGER NOT NULL,
    type VARCHAR(255) NOT NULL,
    bytes INTEGER DEFAULT 0,
    ip VARCHAR(45) NULL,
    shared_with INTEGER DEFAULT 0,
    timestamp DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS quota_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT,
    storage_limit_bytes INTEGER DEFAULT 0,
    upload_limit_bytes INTEGER DEFAULT 0,
    download_limit_bytes INTEGER DEFAULT 0,
    storage_threshold INTEGER,
    upload_threshold INTEGER,
    download_threshold INTEGER,
    is_default BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    window_type TEXT,
    window_duration INTEGER,
    window_start_hour INTEGER,
    window_timezone TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);


CREATE TABLE IF NOT EXISTS user_quota_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    enforcement_policy TEXT NOT NULL,
    quota_plan_id INTEGER,
    storage_limit_bytes INTEGER,
    upload_limit_bytes INTEGER,
    download_limit_bytes INTEGER,
    storage_threshold INTEGER,
    upload_threshold INTEGER,
    download_threshold INTEGER,
    window_type TEXT,
    window_duration INTEGER,
    window_start_hour INTEGER,
    window_timezone TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS allowance_grants (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    type VARCHAR(255) NOT NULL,
    source VARCHAR(255) NOT NULL,
    bytes INTEGER DEFAULT 0,
    bytes_used INTEGER DEFAULT 0,
    bytes_remaining INTEGER DEFAULT 0,
    expiry_date DATETIME,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS allowance_consumptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    grant_id INTEGER NOT NULL,
    usage_detail_id INTEGER NOT NULL,
    bytes_consumed INTEGER DEFAULT 0,
    consumption_date DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE INDEX IF NOT EXISTS idx_user_quotas_date ON user_quotas(date);
CREATE INDEX IF NOT EXISTS idx_user_usage_details_user_id ON user_usage_details(user_id);
CREATE INDEX IF NOT EXISTS idx_user_usage_details_upload_id ON user_usage_details(upload_id);
CREATE INDEX IF NOT EXISTS idx_user_usage_details_type ON user_usage_details(type);
CREATE INDEX IF NOT EXISTS idx_user_usage_details_ip ON user_usage_details(ip);
CREATE INDEX IF NOT EXISTS idx_user_usage_details_timestamp ON user_usage_details(timestamp);
CREATE INDEX IF NOT EXISTS idx_user_quota_configs_quota_plan_id ON user_quota_configs(quota_plan_id);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_user_id ON allowance_grants(user_id);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_type ON allowance_grants(type);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_source ON allowance_grants(source);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_expiry_date ON allowance_grants(expiry_date);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_is_active ON allowance_grants(is_active);
CREATE INDEX IF NOT EXISTS idx_allowance_consumptions_grant_id ON allowance_consumptions(grant_id);
CREATE INDEX IF NOT EXISTS idx_allowance_consumptions_usage_detail_id ON allowance_consumptions(usage_detail_id);
CREATE INDEX IF NOT EXISTS idx_allowance_consumptions_consumption_date ON allowance_consumptions(consumption_date);
CREATE INDEX IF NOT EXISTS idx_user_quota_configs_enforcement_policy ON user_quota_configs(enforcement_policy);
CREATE INDEX IF NOT EXISTS idx_quota_plans_default_active ON quota_plans(is_default, is_active);
CREATE UNIQUE INDEX IF NOT EXISTS uniq_quota_plans_default_active
  ON quota_plans(is_default, is_active)
  WHERE is_default = 1 AND is_active = 1;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_user_quotas_user_date_live
  ON user_quotas(user_id, date) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_user_quota_configs_user_live
  ON user_quota_configs(user_id) WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uniq_quota_plans_name_live
  ON quota_plans(name) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS uniq_user_quotas_user_date_live;
DROP INDEX IF EXISTS uniq_quota_plans_default_active;
DROP INDEX IF EXISTS idx_quota_plans_default_active;
DROP INDEX IF EXISTS idx_user_quota_configs_enforcement_policy;
DROP INDEX IF EXISTS idx_allowance_consumptions_consumption_date;
DROP INDEX IF EXISTS idx_allowance_consumptions_usage_detail_id;
DROP INDEX IF EXISTS idx_allowance_consumptions_grant_id;
DROP INDEX IF EXISTS idx_allowance_grants_is_active;
DROP INDEX IF EXISTS idx_allowance_grants_expiry_date;
DROP INDEX IF EXISTS idx_allowance_grants_source;
DROP INDEX IF EXISTS idx_allowance_grants_type;
DROP INDEX IF EXISTS idx_allowance_grants_user_id;
DROP INDEX IF EXISTS idx_user_quota_configs_quota_plan_id;
DROP INDEX IF EXISTS idx_user_usage_details_timestamp;
DROP INDEX IF EXISTS idx_user_usage_details_ip;
DROP INDEX IF EXISTS idx_user_usage_details_type;
DROP INDEX IF EXISTS idx_user_usage_details_upload_id;
DROP INDEX IF EXISTS idx_user_usage_details_user_id;
DROP INDEX IF EXISTS idx_user_quotas_date;
DROP TABLE allowance_consumptions;
DROP TABLE allowance_grants;
DROP TABLE user_quota_configs;
DROP TABLE quota_plans;
DROP TABLE user_usage_details;
DROP TABLE user_quotas;
-- +goose StatementEnd
