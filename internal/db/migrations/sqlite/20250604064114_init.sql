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
    deleted_at DATETIME DEFAULT NULL,
    UNIQUE(user_id, date)
);

CREATE TABLE IF NOT EXISTS user_usage_details (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    upload_id INTEGER NOT NULL,
    type VARCHAR(255) NOT NULL,
    bytes INTEGER DEFAULT 0,
    ip VARCHAR(45) NOT NULL,
    shared_with INTEGER DEFAULT 0,
    timestamp DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS quota_plans (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT,
    storage_limit INTEGER DEFAULT 0,
    upload_daily_limit INTEGER DEFAULT 0,
    download_daily_limit INTEGER DEFAULT 0,
    upload_total_limit INTEGER DEFAULT 0,
    download_total_limit INTEGER DEFAULT 0,
    storage_threshold INTEGER,
    upload_threshold INTEGER,
    download_threshold INTEGER,
    is_default BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME DEFAULT NULL
);

CREATE TABLE IF NOT EXISTS user_quota_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL UNIQUE,
    enforcement_policy TEXT NOT NULL,
    quota_plan_id INTEGER,
    storage_limit INTEGER,
    upload_daily_limit INTEGER,
    download_daily_limit INTEGER,
    upload_total_limit INTEGER,
    download_total_limit INTEGER,
    storage_threshold INTEGER,
    upload_threshold INTEGER,
    download_threshold INTEGER,
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
CREATE INDEX IF NOT EXISTS idx_allowance_grants_user_id ON allowance_grants(user_id);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_type ON allowance_grants(type);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_source ON allowance_grants(source);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_expiry_date ON allowance_grants(expiry_date);
CREATE INDEX IF NOT EXISTS idx_allowance_grants_is_active ON allowance_grants(is_active);
CREATE INDEX IF NOT EXISTS idx_allowance_consumptions_grant_id ON allowance_consumptions(grant_id);
CREATE INDEX IF NOT EXISTS idx_allowance_consumptions_usage_detail_id ON allowance_consumptions(usage_detail_id);
CREATE INDEX IF NOT EXISTS idx_allowance_consumptions_consumption_date ON allowance_consumptions(consumption_date);
CREATE INDEX IF NOT EXISTS idx_user_quota_configs_enforcement_policy ON user_quota_configs(enforcement_policy);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE allowance_consumptions;
DROP TABLE allowance_grants;
DROP TABLE user_quota_configs;
DROP TABLE quota_plans;
DROP TABLE user_usage_details;
DROP TABLE user_quotas;
-- +goose StatementEnd
