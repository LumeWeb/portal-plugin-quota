-- +goose Up
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
    ip VARCHAR(45) NULL,
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
    name VARCHAR(255) NOT NULL,
    description TEXT,
    storage_limit_bytes BIGINT DEFAULT 0,
    upload_limit_bytes BIGINT DEFAULT 0,
    download_limit_bytes BIGINT DEFAULT 0,
    storage_threshold BIGINT,
    upload_threshold BIGINT,
    download_threshold BIGINT,
    is_default BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    window_type VARCHAR(255),
    window_duration BIGINT,
    window_start_hour INTEGER,
    window_timezone VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    -- Enforce at most one row where is_default=1 AND is_active=1
    default_active_one TINYINT GENERATED ALWAYS AS (
      CASE WHEN is_default AND is_active THEN 1 ELSE NULL END
    ) STORED,
    -- Soft-delete aware unique constraint for name
    name_live VARCHAR(255) GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN name ELSE NULL END) STORED,
    UNIQUE KEY uniq_default_active_one (default_active_one),
    UNIQUE KEY uniq_quota_plans_name_live (name_live),
    INDEX idx_quota_plans_default_active (is_default, is_active)
);

CREATE TABLE IF NOT EXISTS user_quota_configs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    enforcement_policy VARCHAR(255) NOT NULL,
    quota_plan_id BIGINT UNSIGNED,
    storage_limit_bytes BIGINT,
    upload_limit_bytes BIGINT,
    download_limit_bytes BIGINT,
    storage_threshold BIGINT,
    upload_threshold BIGINT,
    download_threshold BIGINT,
    window_type VARCHAR(255),
    window_duration BIGINT,
    window_start_hour INTEGER,
    window_timezone VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    -- Soft-delete aware unique constraint for user_id
    user_id_live BIGINT UNSIGNED GENERATED ALWAYS AS (CASE WHEN deleted_at IS NULL THEN user_id ELSE NULL END) STORED,
    UNIQUE KEY uniq_user_quota_configs_user_live (user_id_live),
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

CREATE TABLE IF NOT EXISTS quota_reservations (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(20) NOT NULL,
    bytes BIGINT UNSIGNED NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    upload_id BIGINT UNSIGNED NULL,
    source_ip VARCHAR(45) NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_user_id (user_id),
    INDEX idx_user_status (user_id, status),
    INDEX idx_deleted_at (deleted_at),
    INDEX idx_upload_id (upload_id)
);

-- +goose Down
DROP TABLE allowance_consumptions;
DROP TABLE quota_reservations;
DROP TABLE allowance_grants;
DROP TABLE user_quota_configs;
DROP TABLE quota_plans;
DROP TABLE user_usage_details;
DROP TABLE user_quotas;
