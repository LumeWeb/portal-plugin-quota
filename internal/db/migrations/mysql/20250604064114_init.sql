-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_quotas (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    date DATE NOT NULL,
    bytes_uploaded BIGINT UNSIGNED DEFAULT 0,
    bytes_downloaded BIGINT UNSIGNED DEFAULT 0,
    bytes_stored BIGINT UNSIGNED DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY user_date (user_id, date),
    INDEX idx_date (date)
);

CREATE TABLE IF NOT EXISTS user_usage_details (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    upload_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(255) NOT NULL,
    bytes BIGINT UNSIGNED DEFAULT 0,
    ip VARCHAR(45) NOT NULL,
    shared_with BIGINT UNSIGNED DEFAULT 0,
    timestamp TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_upload_id (upload_id),
    INDEX idx_type (type),
    INDEX idx_ip (ip),
    INDEX idx_timestamp (timestamp)
);

CREATE TABLE IF NOT EXISTS quota_plans (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    storage_limit BIGINT DEFAULT 0,
    upload_daily_limit BIGINT DEFAULT 0,
    download_daily_limit BIGINT DEFAULT 0,
    upload_total_limit BIGINT DEFAULT 0,
    download_total_limit BIGINT DEFAULT 0,
    storage_threshold BIGINT,
    upload_threshold BIGINT,
    download_threshold BIGINT,
    is_default BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    -- Enforce at most one row where is_default=1 AND is_active=1
    default_active_one TINYINT GENERATED ALWAYS AS (
      CASE WHEN is_default AND is_active THEN 1 ELSE NULL END
    ) STORED,
    UNIQUE KEY uniq_default_active_one (default_active_one),
    INDEX idx_quota_plans_default_active (is_default, is_active)
);

CREATE TABLE IF NOT EXISTS user_quota_configs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    enforcement_policy VARCHAR(255) NOT NULL,
    quota_plan_id BIGINT UNSIGNED,
    storage_limit BIGINT,
    upload_daily_limit BIGINT,
    download_daily_limit BIGINT,
    upload_total_limit BIGINT,
    download_total_limit BIGINT,
    storage_threshold BIGINT,
    upload_threshold BIGINT,
    download_threshold BIGINT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    UNIQUE KEY idx_user_id (user_id),
    INDEX idx_quota_plan_id (quota_plan_id),
    INDEX idx_enforcement_policy (enforcement_policy)
);

CREATE TABLE IF NOT EXISTS allowance_grants (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(255) NOT NULL,
    source VARCHAR(255) NOT NULL,
    bytes BIGINT UNSIGNED DEFAULT 0,
    bytes_used BIGINT UNSIGNED DEFAULT 0,
    bytes_remaining BIGINT UNSIGNED DEFAULT 0,
    expiry_date TIMESTAMP NULL,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_type (type),
    INDEX idx_source (source),
    INDEX idx_expiry_date (expiry_date),
    INDEX idx_is_active (is_active)
);

CREATE TABLE IF NOT EXISTS allowance_consumptions (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    grant_id BIGINT UNSIGNED NOT NULL,
    usage_detail_id BIGINT UNSIGNED NOT NULL,
    bytes_consumed BIGINT UNSIGNED DEFAULT 0,
    consumption_date TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_grant_id (grant_id),
    INDEX idx_usage_detail_id (usage_detail_id),
    INDEX idx_consumption_date (consumption_date)
);
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
