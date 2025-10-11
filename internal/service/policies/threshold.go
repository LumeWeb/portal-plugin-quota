package policies

import (
	"errors"
	"fmt"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
)

// ThresholdPolicyEnforcer implements the PolicyEnforcer interface for THRESHOLD policy
type ThresholdPolicyEnforcer struct {
	*BasePolicyEnforcer
}

// NewThresholdPolicyEnforcer creates a new threshold policy enforcer
func NewThresholdPolicyEnforcer(ctx core.Context) *ThresholdPolicyEnforcer {
	return &ThresholdPolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx),
	}
}

// CheckUploadQuota checks if an upload operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckUploadQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(userID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return t.CheckUploadQuotaWithConfig(userID, config, requestedBytes, effectiveLimits)
}

// CheckUploadQuotaWithConfig checks if an upload operation is allowed under threshold policy with a given config
func (t *ThresholdPolicyEnforcer) CheckUploadQuotaWithConfig(userID uint, config *models.UserQuotaConfig, requestedBytes uint64, effectiveLimits *pluginCore.EffectiveLimits) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(userID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get today's usage for daily limit checks
	todayUsage, err := t.getTodayUsage(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check daily upload limit
	if effectiveLimits.UploadDailyLimit != nil {
		if *effectiveLimits.UploadDailyLimit == 0 {
			// Limit is 0, which means disabled - deny the operation
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				todayUsage.BytesUploaded,
				0,
			), nil
		} else if todayUsage.BytesUploaded+requestedBytes > *effectiveLimits.UploadDailyLimit {
			// Normal limit check for positive values
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				todayUsage.BytesUploaded,
				*effectiveLimits.UploadDailyLimit,
			), nil
		}

		// Check threshold warning
		if effectiveLimits.UploadThreshold != nil && *effectiveLimits.UploadThreshold > 0 {
			if todayUsage.BytesUploaded+requestedBytes > *effectiveLimits.UploadThreshold {
				// Only warn if we're still within the limit
				if todayUsage.BytesUploaded+requestedBytes <= *effectiveLimits.UploadDailyLimit {
					return t.createWarningResult(
						models.EnforcementPolicyThreshold,
						todayUsage.BytesUploaded,
						*effectiveLimits.UploadThreshold,
						*effectiveLimits.UploadDailyLimit,
					), nil
				}
			}
		}
	}

	// Check total upload limit using cumulative total
	if effectiveLimits.UploadTotalLimit != nil {
		// Get cumulative uploaded bytes
		cumulativeTotal, err := t.getTotalBytesByType(config.UserID, models.UsageTypeUpload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		
		if *effectiveLimits.UploadTotalLimit == 0 {
			// Limit is 0, which means disabled - deny the operation
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				0,
			), nil
		} else if cumulativeTotal+requestedBytes > *effectiveLimits.UploadTotalLimit {
			// Normal limit check for positive values
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				*effectiveLimits.UploadTotalLimit,
			), nil
		}

		// Check threshold warning using cumulative total
		if effectiveLimits.UploadThreshold != nil && *effectiveLimits.UploadThreshold > 0 {
			if cumulativeTotal+requestedBytes > *effectiveLimits.UploadThreshold {
				// Only warn if we're still within the limit
				if cumulativeTotal+requestedBytes <= *effectiveLimits.UploadTotalLimit {
					return t.createWarningResult(
						models.EnforcementPolicyThreshold,
						cumulativeTotal,
						*effectiveLimits.UploadThreshold,
						*effectiveLimits.UploadTotalLimit,
					), nil
				}
			}
		}
	}

	return t.createSuccessResult(models.EnforcementPolicyThreshold), nil
}

// CheckDownloadQuota checks if a download operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckDownloadQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(userID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return t.CheckDownloadQuotaWithConfig(userID, config, requestedBytes, effectiveLimits)
}

// CheckDownloadQuotaWithConfig checks if a download operation is allowed under threshold policy with a given config
func (t *ThresholdPolicyEnforcer) CheckDownloadQuotaWithConfig(userID uint, config *models.UserQuotaConfig, requestedBytes uint64, effectiveLimits *pluginCore.EffectiveLimits) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(userID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get today's usage for daily limit checks
	todayUsage, err := t.getTodayUsage(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check daily download limit
	if effectiveLimits.DownloadDailyLimit != nil {
		if *effectiveLimits.DownloadDailyLimit == 0 {
			// Limit is 0, which means disabled - deny the operation
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				todayUsage.BytesDownloaded,
				0,
			), nil
		} else if todayUsage.BytesDownloaded+requestedBytes > *effectiveLimits.DownloadDailyLimit {
			// Normal limit check for positive values
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				todayUsage.BytesDownloaded,
				*effectiveLimits.DownloadDailyLimit,
			), nil
		}

		// Check threshold warning
		if effectiveLimits.DownloadThreshold != nil && *effectiveLimits.DownloadThreshold > 0 {
			if todayUsage.BytesDownloaded+requestedBytes > *effectiveLimits.DownloadThreshold {
				// Only warn if we're still within the limit
				if todayUsage.BytesDownloaded+requestedBytes <= *effectiveLimits.DownloadDailyLimit {
					return t.createWarningResult(
						models.EnforcementPolicyThreshold,
						todayUsage.BytesDownloaded,
						*effectiveLimits.DownloadThreshold,
						*effectiveLimits.DownloadDailyLimit,
					), nil
				}
			}
		}
	}

	// Check total download limit using cumulative total
	if effectiveLimits.DownloadTotalLimit != nil {
		// Get cumulative downloaded bytes
		cumulativeTotal, err := t.getTotalBytesByType(config.UserID, models.UsageTypeDownload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		
		if *effectiveLimits.DownloadTotalLimit == 0 {
			// Limit is 0, which means disabled - deny the operation
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				0,
			), nil
		} else if cumulativeTotal+requestedBytes > *effectiveLimits.DownloadTotalLimit {
			// Normal limit check for positive values
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				*effectiveLimits.DownloadTotalLimit,
			), nil
		}

		// Check threshold warning using cumulative total
		if effectiveLimits.DownloadThreshold != nil && *effectiveLimits.DownloadThreshold > 0 {
			if cumulativeTotal+requestedBytes > *effectiveLimits.DownloadThreshold {
				// Only warn if we're still within the limit
				if cumulativeTotal+requestedBytes <= *effectiveLimits.DownloadTotalLimit {
					return t.createWarningResult(
						models.EnforcementPolicyThreshold,
						cumulativeTotal,
						*effectiveLimits.DownloadThreshold,
						*effectiveLimits.DownloadTotalLimit,
					), nil
				}
			}
		}
	}

	return t.createSuccessResult(models.EnforcementPolicyThreshold), nil
}

// CheckStorageQuota checks if a storage operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckStorageQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(userID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	usage, err := t.getCurrentUsage(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check storage limit
	if effectiveLimits.StorageLimit != nil {
		if *effectiveLimits.StorageLimit == 0 {
			// Limit is 0, which means disabled - deny the operation
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				usage.BytesStored,
				0,
			), nil
		} else if usage.BytesStored+requestedBytes > *effectiveLimits.StorageLimit {
			// Normal limit check for positive values
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				usage.BytesStored,
				*effectiveLimits.StorageLimit,
			), nil
		}

		// Check threshold warning
		if effectiveLimits.StorageThreshold != nil && *effectiveLimits.StorageThreshold > 0 && *effectiveLimits.StorageLimit > 0 {
			if usage.BytesStored+requestedBytes > *effectiveLimits.StorageThreshold && usage.BytesStored+requestedBytes <= *effectiveLimits.StorageLimit {
				return t.createWarningResult(
					models.EnforcementPolicyThreshold,
					usage.BytesStored,
					*effectiveLimits.StorageThreshold,
					*effectiveLimits.StorageLimit,
				), nil
			}
		}
	}

	return t.createSuccessResult(models.EnforcementPolicyThreshold), nil
}

// RecordUpload records an upload operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := t.validateUserID(userID); err != nil {
		return err
	}
	if err := t.validateBytes(bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return err
	}

	// Check quota before recording
	result, err := t.CheckUploadQuotaWithConfig(userID, config, bytes, effectiveLimits)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("upload blocked: %s", result.Reason)
	}

	// Record the detailed usage
	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      models.UsageTypeUpload,
		Bytes:     bytes,
		IP:        ip,
		Timestamp: time.Now(),
	}

	if err := t.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily usage
	if err := t.updateDailyUsage(userID, models.UsageTypeUpload, int64(bytes)); err != nil {
		return err
	}

	return nil
}

// RecordDownload records a download operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := t.validateUserID(userID); err != nil {
		return err
	}
	if err := t.validateBytes(bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return err
	}

	// Check quota before recording
	result, err := t.CheckDownloadQuotaWithConfig(userID, config, bytes, effectiveLimits)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("download blocked: %s", result.Reason)
	}

	// Record the detailed usage
	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      models.UsageTypeDownload,
		Bytes:     bytes,
		IP:        ip,
		Timestamp: time.Now(),
	}

	if err := t.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily usage
	if err := t.updateDailyUsage(userID, models.UsageTypeDownload, int64(bytes)); err != nil {
		return err
	}

	return nil
}

// RecordStorageChange records a storage change operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := t.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Only check quota when adding storage (positive bytes)
	if bytes > 0 {
		result, err := t.CheckStorageQuota(userID, uint64(bytes))
		if err != nil {
			return err
		}

		if !result.Allowed {
			return fmt.Errorf("storage change blocked: %s", result.Reason)
		}
	}

	// Record the detailed usage
	usageType := models.UsageTypeStorageAdd
	if bytes < 0 {
		usageType = models.UsageTypeStorageRemove
	}

	// Use absolute value for bytes when recording usage
	recordBytes := uint64(bytes)
	if bytes < 0 {
		recordBytes = uint64(-bytes)
	}

	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      usageType,
		Bytes:     recordBytes,
		IP:        ip,
		Timestamp: time.Now(),
	}

	if err := t.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily usage with signed delta
	if err := t.updateDailyUsage(userID, usageType, bytes); err != nil {
		return err
	}

	return nil
}

// GetDetailedUsage returns detailed usage records for a user
func (t *ThresholdPolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return t.getDetailedUsage(userID, start, end)
}

// GetCurrentUsage returns current usage statistics for a user
func (t *ThresholdPolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return t.getCurrentUsage(userID)
}

// GetUsageHistory returns usage history for a user
func (t *ThresholdPolicyEnforcer) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	return t.getUsageHistory(userID, period, models.UsageType(usageType))
}

// resolveEffectiveLimits resolves the effective limits for a user based on their configuration
func (t *ThresholdPolicyEnforcer) resolveEffectiveLimits(config *models.UserQuotaConfig) (*pluginCore.EffectiveLimits, error) {
	limits := &pluginCore.EffectiveLimits{
		UserID:            config.UserID,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
	}

	// If user has a quota plan, use it as the base
	var plan *models.QuotaPlan
	if config.QuotaPlanID != nil {
		var err error
		plan, err = t.getQuotaPlan(*config.QuotaPlanID)
		if err != nil {
			return nil, err
		}
	} else {
		// Try to get default plan
		var err error
		plan, err = t.getDefaultQuotaPlan()
		if err != nil {
			// Only ignore ErrRecordNotFound, propagate other errors
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			// If no default plan, continue with nil plan
		}
	}

	// Set limits from plan if available
	if plan != nil {
		limits.StorageLimit = t.BasePolicyEnforcer.convertLimitValue(plan.StorageLimit)
		limits.UploadDailyLimit = t.BasePolicyEnforcer.convertLimitValue(plan.UploadDailyLimit)
		limits.DownloadDailyLimit = t.BasePolicyEnforcer.convertLimitValue(plan.DownloadDailyLimit)
		limits.UploadTotalLimit = t.BasePolicyEnforcer.convertLimitValue(plan.UploadTotalLimit)
		limits.DownloadTotalLimit = t.BasePolicyEnforcer.convertLimitValue(plan.DownloadTotalLimit)
		if plan.StorageThreshold != nil {
			limits.StorageThreshold = t.BasePolicyEnforcer.convertLimitValue(*plan.StorageThreshold)
		}
		if plan.UploadThreshold != nil {
			limits.UploadThreshold = t.BasePolicyEnforcer.convertLimitValue(*plan.UploadThreshold)
		}
		if plan.DownloadThreshold != nil {
			limits.DownloadThreshold = t.BasePolicyEnforcer.convertLimitValue(*plan.DownloadThreshold)
		}
		limits.QuotaPlanID = lo.ToPtr(uint64(plan.ID))
	}

	// Override with user-specific limits if set
	if config.StorageLimit != nil {
		limits.StorageLimit = t.BasePolicyEnforcer.convertLimitValue(*config.StorageLimit)
	}
	if config.UploadDailyLimit != nil {
		limits.UploadDailyLimit = t.BasePolicyEnforcer.convertLimitValue(*config.UploadDailyLimit)
	}
	if config.DownloadDailyLimit != nil {
		limits.DownloadDailyLimit = t.BasePolicyEnforcer.convertLimitValue(*config.DownloadDailyLimit)
	}
	if config.UploadTotalLimit != nil {
		limits.UploadTotalLimit = t.BasePolicyEnforcer.convertLimitValue(*config.UploadTotalLimit)
	}
	if config.DownloadTotalLimit != nil {
		limits.DownloadTotalLimit = t.BasePolicyEnforcer.convertLimitValue(*config.DownloadTotalLimit)
	}
	if config.StorageThreshold != nil {
		limits.StorageThreshold = t.BasePolicyEnforcer.convertLimitValue(*config.StorageThreshold)
	}
	if config.UploadThreshold != nil {
		limits.UploadThreshold = t.BasePolicyEnforcer.convertLimitValue(*config.UploadThreshold)
	}
	if config.DownloadThreshold != nil {
		limits.DownloadThreshold = t.BasePolicyEnforcer.convertLimitValue(*config.DownloadThreshold)
	}

	return limits, nil
}

// applyLimit sets a limit field if it passes validation
func (t *ThresholdPolicyEnforcer) applyLimit(dest **uint64, source *int64, limitName string) error {
	if source == nil {
		return nil
	}
	
	if *dest == nil {
		convertedValue := t.BasePolicyEnforcer.convertLimitValue(*source)
		*dest = convertedValue
	}
	return nil
}

// getQuotaPlan retrieves a quota plan by ID
func (t *ThresholdPolicyEnforcer) getQuotaPlan(planID uint64) (*models.QuotaPlan, error) {
	var plan models.QuotaPlan
	err := t.db.Where("id = ?", planID).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}

// getTodayUsage retrieves today's usage for a user
func (t *ThresholdPolicyEnforcer) getTodayUsage(userID uint) (*pluginCore.Usage, error) {
	today := time.Now().Truncate(24 * time.Hour)
	
	var quota models.UserQuota
	err := t.db.Where("user_id = ? AND date = ?", userID, today).First(&quota).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return zero usage for today if no record exists
			return &pluginCore.Usage{
				UserID:          userID,
				BytesUploaded:   0,
				BytesDownloaded: 0,
				BytesStored:     0,
				LastUpdated:     today,
			}, nil
		}
		return nil, err
	}
	
	return &pluginCore.Usage{
		UserID:          userID,
		BytesUploaded:   quota.BytesUploaded,
		BytesDownloaded: quota.BytesDownloaded,
		BytesStored:     quota.BytesStored,
		LastUpdated:     quota.Date,
	}, nil
}

// getTotalBytesByType retrieves the total bytes consumed for a specific usage type across all time
func (t *ThresholdPolicyEnforcer) getTotalBytesByType(userID uint, usageType models.UsageType) (uint64, error) {
	var totalBytes uint64
	err := t.db.Model(&models.UserUsageDetail{}).
		Where("user_id = ? AND type = ?", userID, usageType).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&totalBytes).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get total bytes for type %s: %w", usageType, err)
	}

	return totalBytes, nil
}


// getDefaultQuotaPlan retrieves the default quota plan
func (t *ThresholdPolicyEnforcer) getDefaultQuotaPlan() (*models.QuotaPlan, error) {
	var plan models.QuotaPlan
	err := t.db.Where("is_default = ? AND is_active = ?", true, true).First(&plan).Error
	if err != nil {
		return nil, err
	}
	return &plan, nil
}
