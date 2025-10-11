package policies

import (
	"fmt"
	"time"

	"github.com/docker/go-units"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// HardLimitsPolicyEnforcer implements PolicyEnforcer for hard limits policy
type HardLimitsPolicyEnforcer struct {
	*BasePolicyEnforcer
}

// NewHardLimitsPolicyEnforcer creates a new hard limits policy enforcer
func NewHardLimitsPolicyEnforcer(ctx core.Context) *HardLimitsPolicyEnforcer {
	return &HardLimitsPolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx),
	}
}

// CheckUploadQuota checks if an upload operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckUploadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := h.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := h.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get effective limits for the user
	limits, err := h.getEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get current usage
	usage, err := h.getCurrentUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check daily upload limit
	if limits.UploadDailyLimit != nil {
		if usage.BytesUploaded+requestedBytes > *limits.UploadDailyLimit {
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesUploaded, *limits.UploadDailyLimit), nil
		}
	}

	// Check total upload limit against aggregated usage
	if limits.UploadTotalLimit != nil {
		aggregatedUsage, err := h.getAggregatedUsageByType(config.UserID, models.UsageTypeUpload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if aggregatedUsage+requestedBytes > *limits.UploadTotalLimit {
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, aggregatedUsage, *limits.UploadTotalLimit), nil
		}
	}

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// CheckDownloadQuota checks if a download operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := h.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get effective limits for the user
	limits, err := h.getEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get current usage
	usage, err := h.getCurrentUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check daily download limit
	if limits.DownloadDailyLimit != nil {
		if usage.BytesDownloaded+requestedBytes > *limits.DownloadDailyLimit {
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesDownloaded, *limits.DownloadDailyLimit), nil
		}
	}

	// Check total download limit against aggregated usage
	if limits.DownloadTotalLimit != nil {
		aggregatedUsage, err := h.getAggregatedUsageByType(config.UserID, models.UsageTypeDownload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if aggregatedUsage+requestedBytes > *limits.DownloadTotalLimit {
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, aggregatedUsage, *limits.DownloadTotalLimit), nil
		}
	}

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// CheckStorageQuota checks if a storage operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := h.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get effective limits for the user
	limits, err := h.getEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get current usage
	usage, err := h.getCurrentUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check storage limit
	if limits.StorageLimit != nil {
		if usage.BytesStored+requestedBytes > *limits.StorageLimit {
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesStored, *limits.StorageLimit), nil
		}
	}

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// RecordUpload records an upload operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := h.validateUserID(userID); err != nil {
		return err
	}
	if err := h.validateBytes(bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := h.getUserQuotaConfig(userID)
	if err != nil {
		return err
	}

	// Check quota before recording
	result, err := h.CheckUploadQuota(config, bytes)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("upload blocked: %s", result.Reason)
	}

	// Record the usage detail
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeUpload,
		Bytes:      bytes,
		IP:         ip,
		Timestamp:  time.Now(),
		SharedWith: 1, // Uploads are not shared initially
	}

	if err := h.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily usage
	return h.updateDailyUsage(userID, models.UsageTypeUpload, int64(bytes))
}

// RecordDownload records a download operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := h.validateUserID(userID); err != nil {
		return err
	}
	if err := h.validateBytes(bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := h.getUserQuotaConfig(userID)
	if err != nil {
		return err
	}

	// Check quota before recording
	result, err := h.CheckDownloadQuota(config, bytes)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("download blocked: %s", result.Reason)
	}

	// Record the usage detail
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeDownload,
		Bytes:      bytes,
		IP:         ip,
		Timestamp:  time.Now(),
		SharedWith: 1, // Default to 1, will be updated by shared usage calculation
	}

	if err := h.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily usage
	return h.updateDailyUsage(userID, models.UsageTypeDownload, int64(bytes))
}

// RecordStorageChange records a storage change operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := h.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Get user's quota config
	config, err := h.getUserQuotaConfig(userID)
	if err != nil {
		return err
	}

	// For storage changes, we only enforce limits when adding storage
	if bytes > 0 {
		// Check quota before recording
		result, err := h.CheckStorageQuota(config, uint64(bytes))
		if err != nil {
			return err
		}

		if !result.Allowed {
			return fmt.Errorf("storage change blocked: %s", result.Reason)
		}
	}

	// Determine usage type and byte value for recording
	var usageType models.UsageType
	var recordBytes uint64
	if bytes < 0 {
		usageType = models.UsageTypeStorageRemove
		recordBytes = uint64(-bytes)
	} else {
		usageType = models.UsageTypeStorageAdd
		recordBytes = uint64(bytes)
	}

	// Record the usage detail
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       usageType,
		Bytes:      recordBytes,
		IP:         ip,
		Timestamp:  time.Now(),
		SharedWith: 1, // Default to 1, will be updated by shared usage calculation
	}

	if err := h.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily usage with the correct usage type and byte value
	return h.updateDailyUsage(userID, usageType, int64(recordBytes))
}

// GetDetailedUsage delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return h.getDetailedUsage(userID, start, end)
}

// GetCurrentUsage delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return h.getCurrentUsage(userID)
}

// GetUsageHistory delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	return h.getUsageHistory(userID, period, models.UsageType(usageType))
}

// getEffectiveLimits resolves the effective limits for a user based on their configuration
func (h *HardLimitsPolicyEnforcer) getEffectiveLimits(config *models.UserQuotaConfig) (*pluginCore.EffectiveLimits, error) {
	if err := h.validateUserID(config.UserID); err != nil {
		return nil, err
	}

	limits := &pluginCore.EffectiveLimits{
		UserID:            config.UserID,
		EnforcementPolicy: pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits),
	}

	// If user has custom limits, use those (with validation)
	if config.StorageLimit != nil {
		if err := h.validateLimitValue(*config.StorageLimit); err != nil {
			return nil, fmt.Errorf("invalid storage limit in user config: %w", err)
		}
		limits.StorageLimit = config.StorageLimit
	}
	if config.UploadDailyLimit != nil {
		if err := h.validateLimitValue(*config.UploadDailyLimit); err != nil {
			return nil, fmt.Errorf("invalid upload daily limit in user config: %w", err)
		}
		limits.UploadDailyLimit = config.UploadDailyLimit
	}
	if config.DownloadDailyLimit != nil {
		if err := h.validateLimitValue(*config.DownloadDailyLimit); err != nil {
			return nil, fmt.Errorf("invalid download daily limit in user config: %w", err)
		}
		limits.DownloadDailyLimit = config.DownloadDailyLimit
	}
	if config.UploadTotalLimit != nil {
		if err := h.validateLimitValue(*config.UploadTotalLimit); err != nil {
			return nil, fmt.Errorf("invalid upload total limit in user config: %w", err)
		}
		limits.UploadTotalLimit = config.UploadTotalLimit
	}
	if config.DownloadTotalLimit != nil {
		if err := h.validateLimitValue(*config.DownloadTotalLimit); err != nil {
			return nil, fmt.Errorf("invalid download total limit in user config: %w", err)
		}
		limits.DownloadTotalLimit = config.DownloadTotalLimit
	}

	// If user is assigned to a plan, use plan limits for any unset custom limits
	if config.QuotaPlanID != nil {
		var plan models.QuotaPlan
		err := h.db.Where("id = ?", *config.QuotaPlanID).First(&plan).Error
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve quota plan: %w", err)
		}

		// Only set limits that aren't already set by custom config (with validation)
		if limits.StorageLimit == nil && plan.StorageLimit > 0 {
			if err := h.validateLimitValue(plan.StorageLimit); err != nil {
				return nil, fmt.Errorf("invalid storage limit in quota plan: %w", err)
			}
			limits.StorageLimit = &plan.StorageLimit
		}
		if limits.UploadDailyLimit == nil && plan.UploadDailyLimit > 0 {
			if err := h.validateLimitValue(plan.UploadDailyLimit); err != nil {
				return nil, fmt.Errorf("invalid upload daily limit in quota plan: %w", err)
			}
			limits.UploadDailyLimit = &plan.UploadDailyLimit
		}
		if limits.DownloadDailyLimit == nil && plan.DownloadDailyLimit > 0 {
			if err := h.validateLimitValue(plan.DownloadDailyLimit); err != nil {
				return nil, fmt.Errorf("invalid download daily limit in quota plan: %w", err)
			}
			limits.DownloadDailyLimit = &plan.DownloadDailyLimit
		}
		if limits.UploadTotalLimit == nil && plan.UploadTotalLimit > 0 {
			if err := h.validateLimitValue(plan.UploadTotalLimit); err != nil {
				return nil, fmt.Errorf("invalid upload total limit in quota plan: %w", err)
			}
			limits.UploadTotalLimit = &plan.UploadTotalLimit
		}
		if limits.DownloadTotalLimit == nil && plan.DownloadTotalLimit > 0 {
			if err := h.validateLimitValue(plan.DownloadTotalLimit); err != nil {
				return nil, fmt.Errorf("invalid download total limit in quota plan: %w", err)
			}
			limits.DownloadTotalLimit = &plan.DownloadTotalLimit
		}
	} else {
		// If no plan assigned, check for default plan
		var defaultPlan models.QuotaPlan
		err := h.db.Where("is_default = true").First(&defaultPlan).Error
		if err == nil {
			// Only set limits that aren't already set by custom config (with validation)
			if limits.StorageLimit == nil && defaultPlan.StorageLimit > 0 {
				if err := h.validateLimitValue(defaultPlan.StorageLimit); err != nil {
					return nil, fmt.Errorf("invalid storage limit in default plan: %w", err)
				}
				limits.StorageLimit = &defaultPlan.StorageLimit
			}
			if limits.UploadDailyLimit == nil && defaultPlan.UploadDailyLimit > 0 {
				if err := h.validateLimitValue(defaultPlan.UploadDailyLimit); err != nil {
					return nil, fmt.Errorf("invalid upload daily limit in default plan: %w", err)
				}
				limits.UploadDailyLimit = &defaultPlan.UploadDailyLimit
			}
			if limits.DownloadDailyLimit == nil && defaultPlan.DownloadDailyLimit > 0 {
				if err := h.validateLimitValue(defaultPlan.DownloadDailyLimit); err != nil {
					return nil, fmt.Errorf("invalid download daily limit in default plan: %w", err)
				}
				limits.DownloadDailyLimit = &defaultPlan.DownloadDailyLimit
			}
			if limits.UploadTotalLimit == nil && defaultPlan.UploadTotalLimit > 0 {
				if err := h.validateLimitValue(defaultPlan.UploadTotalLimit); err != nil {
					return nil, fmt.Errorf("invalid upload total limit in default plan: %w", err)
				}
				limits.UploadTotalLimit = &defaultPlan.UploadTotalLimit
			}
			if limits.DownloadTotalLimit == nil && defaultPlan.DownloadTotalLimit > 0 {
				if err := h.validateLimitValue(defaultPlan.DownloadTotalLimit); err != nil {
					return nil, fmt.Errorf("invalid download total limit in default plan: %w", err)
				}
				limits.DownloadTotalLimit = &defaultPlan.DownloadTotalLimit
			}
		}
	}

	// Validate that we have at least some limits configured
	if limits.StorageLimit == nil && limits.UploadDailyLimit == nil && limits.DownloadDailyLimit == nil &&
		limits.UploadTotalLimit == nil && limits.DownloadTotalLimit == nil {
		return nil, fmt.Errorf("no limits configured for user %d with hard limits policy", config.UserID)
	}

	return limits, nil
}

// getAggregatedUsageByType retrieves aggregated usage for a specific type across all time
func (h *HardLimitsPolicyEnforcer) getAggregatedUsageByType(userID uint, usageType models.UsageType) (uint64, error) {
	if err := h.validateUserID(userID); err != nil {
		return 0, err
	}

	var totalBytes uint64
	err := h.db.Model(&models.UserUsageDetail{}).
		Where("user_id = ? AND type = ?", userID, usageType).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&totalBytes).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get aggregated usage for type %s: %w", usageType, err)
	}

	return totalBytes, nil
}

// validateLimitValue validates that a limit value is reasonable
func (h *HardLimitsPolicyEnforcer) validateLimitValue(value uint64) error {
	// Basic validation - ensure the value is not zero (unless that's intended to mean unlimited)
	// and not unreasonably large
	if value == 0 {
		return fmt.Errorf("limit value cannot be zero")
	}

	// Check if the value is unreasonably large (1 PiB should be enough for most use cases)
	if value > uint64(units.PiB) {
		return fmt.Errorf("limit value %d is unreasonably large", value)
	}

	return nil
}
