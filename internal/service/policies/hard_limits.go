package policies

import (
	"errors"
	"fmt"
	"time"

	"github.com/docker/go-units"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
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
	if err := h.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := h.validateUserID(config.UserID); err != nil {
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
		limitValue := uint64(*limits.UploadDailyLimit)
		if limitValue == 0 {
			// Limit is 0, which means disabled - deny the operation
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesUploaded, 0), nil
		} else if usage.BytesUploaded+requestedBytes > limitValue {
			// Normal limit check for positive values
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesUploaded, limitValue), nil
		}
	}

	// Check total upload limit against aggregated usage
	if limits.UploadTotalLimit != nil {
		aggregatedUsage, err := h.getAggregatedUsageByType(config.UserID, models.UsageTypeUpload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		limitValue := uint64(*limits.UploadTotalLimit)
		if limitValue == 0 {
			// Limit is 0, which means disabled - deny the operation
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, aggregatedUsage, 0), nil
		} else {
			if aggregatedUsage+requestedBytes > limitValue {
				// Normal limit check for positive values
				return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, aggregatedUsage, limitValue), nil
			}
		}
	}

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// CheckDownloadQuota checks if a download operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := h.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := h.validateUserID(config.UserID); err != nil {
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
		limitValue := uint64(*limits.DownloadDailyLimit)
		if limitValue == 0 {
			// Limit is 0, which means disabled - deny the operation
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesDownloaded, 0), nil
		} else if usage.BytesDownloaded+requestedBytes > limitValue {
			// Normal limit check for positive values
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesDownloaded, limitValue), nil
		}
	}

	// Check total download limit against aggregated usage
	if limits.DownloadTotalLimit != nil {
		aggregatedUsage, err := h.getAggregatedUsageByType(config.UserID, models.UsageTypeDownload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		limitValue := uint64(*limits.DownloadTotalLimit)
		if limitValue == 0 {
			// Limit is 0, which means disabled - deny the operation
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, aggregatedUsage, 0), nil
		} else {
			if aggregatedUsage+requestedBytes > limitValue {
				// Normal limit check for positive values
				return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, aggregatedUsage, limitValue), nil
			}
		}
	}

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// CheckStorageQuota checks if a storage operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := h.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := h.validateUserID(config.UserID); err != nil {
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
		limitValue := uint64(*limits.StorageLimit)
		if limitValue == 0 {
			// Limit is 0, which means disabled - deny the operation
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesStored, 0), nil
		} else if usage.BytesStored+requestedBytes > limitValue {
			// Normal limit check for positive values
			return h.createLimitExceededResult(models.EnforcementPolicyHardLimits, usage.BytesStored, limitValue), nil
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
	// For storage operations, we need to pass the signed bytes value to properly handle removals
	if usageType == models.UsageTypeStorageRemove {
		return h.updateDailyUsage(userID, usageType, -int64(recordBytes))
	}
	return h.updateDailyUsage(userID, usageType, bytes)
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

// applyLimit sets a limit field if it passes validation
func (h *HardLimitsPolicyEnforcer) applyLimit(dest **uint64, source int64, limitName string) error {
	if *dest == nil {
		if err := h.validateLimitValue(source); err != nil {
			return fmt.Errorf("invalid %s: %w", limitName, err)
		}
		convertedValue := h.BasePolicyEnforcer.convertLimitValue(source)
		*dest = convertedValue
	}
	return nil
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
		if err := h.applyLimit(&limits.StorageLimit, *config.StorageLimit, "storage limit in user config"); err != nil {
			return nil, err
		}
	}
	if config.UploadDailyLimit != nil {
		if err := h.applyLimit(&limits.UploadDailyLimit, *config.UploadDailyLimit, "upload daily limit in user config"); err != nil {
			return nil, err
		}
	}
	if config.DownloadDailyLimit != nil {
		if err := h.applyLimit(&limits.DownloadDailyLimit, *config.DownloadDailyLimit, "download daily limit in user config"); err != nil {
			return nil, err
		}
	}
	if config.UploadTotalLimit != nil {
		if err := h.applyLimit(&limits.UploadTotalLimit, *config.UploadTotalLimit, "upload total limit in user config"); err != nil {
			return nil, err
		}
	}
	if config.DownloadTotalLimit != nil {
		if err := h.applyLimit(&limits.DownloadTotalLimit, *config.DownloadTotalLimit, "download total limit in user config"); err != nil {
			return nil, err
		}
	}

	// If user is assigned to a plan, use plan limits for any unset custom limits
	if config.QuotaPlanID != nil {
		var plan models.QuotaPlan
		err := h.db.Where("id = ?", *config.QuotaPlanID).First(&plan).Error
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve quota plan: %w", err)
		}

		// Only set limits that aren't already set by custom config (with validation)
		if err := h.applyLimit(&limits.StorageLimit, plan.StorageLimit, "storage limit in quota plan"); err != nil {
			return nil, err
		}
		if err := h.applyLimit(&limits.UploadDailyLimit, plan.UploadDailyLimit, "upload daily limit in quota plan"); err != nil {
			return nil, err
		}
		if err := h.applyLimit(&limits.DownloadDailyLimit, plan.DownloadDailyLimit, "download daily limit in quota plan"); err != nil {
			return nil, err
		}
		if err := h.applyLimit(&limits.UploadTotalLimit, plan.UploadTotalLimit, "upload total limit in quota plan"); err != nil {
			return nil, err
		}
		if err := h.applyLimit(&limits.DownloadTotalLimit, plan.DownloadTotalLimit, "download total limit in quota plan"); err != nil {
			return nil, err
		}
	} else {
		// If no plan assigned, check for default plan that is active
		var defaultPlan models.QuotaPlan
		err := h.db.Where("is_default = true AND is_active = true").First(&defaultPlan).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("failed to retrieve default quota plan: %w", err)
			}
			// No default plan found, continue with nil plan
		} else {
			// Only set limits that aren't already set by custom config (with validation)
			if err := h.applyLimit(&limits.StorageLimit, defaultPlan.StorageLimit, "storage limit in default plan"); err != nil {
				return nil, err
			}
			if err := h.applyLimit(&limits.UploadDailyLimit, defaultPlan.UploadDailyLimit, "upload daily limit in default plan"); err != nil {
				return nil, err
			}
			if err := h.applyLimit(&limits.DownloadDailyLimit, defaultPlan.DownloadDailyLimit, "download daily limit in default plan"); err != nil {
				return nil, err
			}
			if err := h.applyLimit(&limits.UploadTotalLimit, defaultPlan.UploadTotalLimit, "upload total limit in default plan"); err != nil {
				return nil, err
			}
			if err := h.applyLimit(&limits.DownloadTotalLimit, defaultPlan.DownloadTotalLimit, "download total limit in default plan"); err != nil {
				return nil, err
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
func (h *HardLimitsPolicyEnforcer) validateLimitValue(value int64) error {
	// Valid values: -1 (unlimited), 0 (disabled), or positive values
	if value < -1 {
		return fmt.Errorf("invalid limit value %d: must be -1, 0, or positive", value)
	}

	// Check if the value is unreasonably large (1 PiB should be enough for most use cases)
	if value > int64(units.PiB) {
		return fmt.Errorf("limit value %d is unreasonably large", value)
	}

	return nil
}
