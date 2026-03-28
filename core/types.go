package core

import (
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// QuotaCheckResult represents the result of a quota check
type QuotaCheckResult struct {
	Allowed bool
	Reason  QuotaCheckReason // "OK", "LIMIT_EXCEEDED", "ALLOWANCE_DEPLETED", "WARNING_THRESHOLD", etc.
	Details QuotaCheckDetails
}

// QuotaCheckDetails provides detailed information about quota status
type QuotaCheckDetails struct {
	CurrentUsage  uint64            `json:"current_usage"`
	Limit         *uint64           `json:"limit,omitempty"`          // For HARD_LIMITS/THRESHOLD
	Allowance     *uint64           `json:"allowance,omitempty"`      // For ALLOWANCE
	AllowanceUsed *uint64           `json:"allowance_used,omitempty"` // For ALLOWANCE
	Threshold     *uint64           `json:"threshold,omitempty"`      // For THRESHOLD
	Policy        EnforcementPolicy `json:"policy"`
}

// Usage represents current usage statistics for a user
type Usage struct {
	UserID          uint                 `json:"user_id"`
	BytesUploaded   uint64               `json:"bytes_uploaded"`
	BytesDownloaded uint64               `json:"bytes_downloaded"`
	BytesStored     uint64               `json:"bytes_stored"`
	LastUpdated     time.Time            `json:"last_updated"`
	UsageByType     map[UsageType]uint64 `json:"usage_by_type"`
}

// UsagePoint represents a single data point in usage history
type UsagePoint struct {
	Date   time.Time `json:"date"`
	Bytes  uint64    `json:"bytes"`
	Type   UsageType `json:"type"`
	UserID uint      `json:"user_id"`
}

// EffectiveLimits represents the resolved limits for a user
type EffectiveLimits struct {
	UserID             uint              `json:"user_id"`
	EnforcementPolicy  EnforcementPolicy `json:"enforcement_policy"`
	StorageLimit       *uint64           `json:"storage_limit,omitempty"`
	UploadDailyLimit   *uint64           `json:"upload_daily_limit,omitempty"`
	DownloadDailyLimit *uint64           `json:"download_daily_limit,omitempty"`
	UploadTotalLimit   *uint64           `json:"upload_total_limit,omitempty"`
	DownloadTotalLimit *uint64           `json:"download_total_limit,omitempty"`
	StorageThreshold   *uint64           `json:"storage_threshold,omitempty"`
	UploadThreshold    *uint64           `json:"upload_threshold,omitempty"`
	DownloadThreshold  *uint64           `json:"download_threshold,omitempty"`
	QuotaPlanID        *uint64           `json:"quota_plan_id,omitempty"`

	// Track whether limits were explicitly configured (even if unlimited)
	HasStorageLimitConfig       bool `json:"has_storage_limit_config"`
	HasUploadDailyLimitConfig   bool `json:"has_upload_daily_limit_config"`
	HasDownloadDailyLimitConfig bool `json:"has_download_daily_limit_config"`
	HasUploadTotalLimitConfig   bool `json:"has_upload_total_limit_config"`
	HasDownloadTotalLimitConfig bool `json:"has_download_total_limit_config"`
	HasStorageThresholdConfig   bool `json:"has_storage_threshold_config"`
	HasUploadThresholdConfig    bool `json:"has_upload_threshold_config"`
	HasDownloadThresholdConfig  bool `json:"has_download_threshold_config"`
}

// HasAnyLimits returns true if any limits are configured for this user
// This includes both finite limits and unlimited limits (represented as nil)
func (e EffectiveLimits) HasAnyLimits() bool {
	return e.HasStorageLimitConfig ||
		e.HasUploadDailyLimitConfig ||
		e.HasDownloadDailyLimitConfig ||
		e.HasUploadTotalLimitConfig ||
		e.HasDownloadTotalLimitConfig ||
		e.HasStorageThresholdConfig ||
		e.HasUploadThresholdConfig ||
		e.HasDownloadThresholdConfig
}

// AllowanceBalance represents the current allowance balance for a user
type AllowanceBalance struct {
	StorageAllowance  uint64 `json:"storage_allowance"`
	StorageUsed       uint64 `json:"storage_used"`
	StorageRemaining  uint64 `json:"storage_remaining"`
	UploadAllowance   uint64 `json:"upload_allowance"`
	UploadUsed        uint64 `json:"upload_used"`
	UploadRemaining   uint64 `json:"upload_remaining"`
	DownloadAllowance uint64 `json:"download_allowance"`
	DownloadUsed      uint64 `json:"download_used"`
	DownloadRemaining uint64 `json:"download_remaining"`
}

// GrantConsumption represents the consumption of bytes from a specific grant
type GrantConsumption struct {
	GrantID         uint      `json:"grant_id"`
	BytesConsumed   uint64    `json:"bytes_consumed"`
	ConsumptionDate time.Time `json:"consumption_date"`
}

// Import types from models package to avoid circular imports
// These types are defined in the models package and used here

type (
	// UserQuotaConfig represents a user's quota configuration
	UserQuotaConfig = models.UserQuotaConfig

	// AllowanceGrant represents an individual allowance grant
	AllowanceGrant = models.AllowanceGrant

	// UserUsageDetail represents detailed usage records
	UserUsageDetail = models.UserUsageDetail

	// AllowanceConsumption represents allowance consumption records
	AllowanceConsumption = models.AllowanceConsumption

	// EnforcementPolicy represents the quota enforcement policy
	EnforcementPolicy = models.EnforcementPolicy

	// GrantType represents the type of resource being granted
	GrantType = models.GrantType

	// UsageType represents the type of usage
	UsageType = models.UsageType

	// QuotaCheckReason represents the reason for a quota check result
	QuotaCheckReason = models.QuotaCheckReason
)

const (
	GrantTypeStorage  = models.GrantTypeStorage
	GrantTypeUpload   = models.GrantTypeUpload
	GrantTypeDownload = models.GrantTypeDownload
)
const (
	UsageTypeUpload        = models.UsageTypeUpload
	UsageTypeDownload      = models.UsageTypeDownload
	UsageTypeStorageAdd    = models.UsageTypeStorageAdd
	UsageTypeStorageRemove = models.UsageTypeStorageRemove
)

type QuotaPlan = models.QuotaPlan
type QuotaConfig = config.QuotaConfig

// SystemStats represents system-wide quota statistics
type SystemStats struct {
	TotalUsers      int64  `json:"total_users"`
	ActiveUsers     int64  `json:"active_users"`
	TotalPlans      int64  `json:"total_plans"`
	ActivePlans     int64  `json:"active_plans"`
	TotalGrants     int64  `json:"total_grants"`
	ActiveGrants    int64  `json:"active_grants"`
	CurrentUsage    Usage  `json:"current_usage"`
	TotalUsageBytes uint64 `json:"total_usage_bytes"`
}

// UserQuotaConfigUpdate represents the fields that can be updated for a user's quota config
type UserQuotaConfigUpdate struct {
	EnforcementPolicy  *EnforcementPolicy `json:"enforcement_policy,omitempty"`
	QuotaPlanID        *uint64            `json:"quota_plan_id,omitempty"`
	StorageLimit       *int64             `json:"storage_limit,omitempty"`
	UploadDailyLimit   *int64             `json:"upload_daily_limit,omitempty"`
	DownloadDailyLimit *int64             `json:"download_daily_limit,omitempty"`
	UploadTotalLimit   *int64             `json:"upload_total_limit,omitempty"`
	DownloadTotalLimit *int64             `json:"download_total_limit,omitempty"`
	StorageThreshold   *int64             `json:"storage_threshold,omitempty"`
	UploadThreshold    *int64             `json:"upload_threshold,omitempty"`
	DownloadThreshold  *int64             `json:"download_threshold,omitempty"`
}

