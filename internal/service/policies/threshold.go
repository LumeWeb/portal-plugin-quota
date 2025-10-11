package policies

import (
	"fmt"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
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

	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return t.CheckUploadQuotaWithConfig(config, requestedBytes)
}

// CheckUploadQuotaWithConfig checks if an upload operation is allowed under threshold policy with a given config
func (t *ThresholdPolicyEnforcer) CheckUploadQuotaWithConfig(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	usage, err := t.getCurrentUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check daily upload limit
	if effectiveLimits.UploadDailyLimit != nil {
		if usage.BytesUploaded+requestedBytes > *effectiveLimits.UploadDailyLimit {
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				usage.BytesUploaded,
				*effectiveLimits.UploadDailyLimit,
			), nil
		}

		// Check threshold warning
		if effectiveLimits.UploadThreshold != nil && usage.BytesUploaded+requestedBytes > *effectiveLimits.UploadThreshold {
			return t.createWarningResult(
				models.EnforcementPolicyThreshold,
				usage.BytesUploaded,
				*effectiveLimits.UploadThreshold,
				*effectiveLimits.UploadDailyLimit,
			), nil
		}
	}

	// Check total upload limit using cumulative total
	if effectiveLimits.UploadTotalLimit != nil {
		// Get cumulative uploaded bytes
		cumulativeTotal, err := t.getTotalBytesByType(config.UserID, models.UsageTypeUpload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		
		if cumulativeTotal+requestedBytes > *effectiveLimits.UploadTotalLimit {
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				*effectiveLimits.UploadTotalLimit,
			), nil
		}

		// Check threshold warning using cumulative total
		if effectiveLimits.UploadThreshold != nil && cumulativeTotal+requestedBytes > *effectiveLimits.UploadThreshold {
			return t.createWarningResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				*effectiveLimits.UploadThreshold,
				*effectiveLimits.UploadTotalLimit,
			), nil
		}
	}

	return t.createSuccessResult(models.EnforcementPolicyThreshold), nil
}

// CheckDownloadQuota checks if a download operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckDownloadQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	config, err := t.getUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return t.CheckDownloadQuotaWithConfig(config, requestedBytes)
}

// CheckDownloadQuotaWithConfig checks if a download operation is allowed under threshold policy with a given config
func (t *ThresholdPolicyEnforcer) CheckDownloadQuotaWithConfig(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	usage, err := t.getCurrentUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	effectiveLimits, err := t.resolveEffectiveLimits(config)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check daily download limit
	if effectiveLimits.DownloadDailyLimit != nil {
		if usage.BytesDownloaded+requestedBytes > *effectiveLimits.DownloadDailyLimit {
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				usage.BytesDownloaded,
				*effectiveLimits.DownloadDailyLimit,
			), nil
		}

		// Check threshold warning
		if effectiveLimits.DownloadThreshold != nil && usage.BytesDownloaded+requestedBytes > *effectiveLimits.DownloadThreshold {
			return t.createWarningResult(
				models.EnforcementPolicyThreshold,
				usage.BytesDownloaded,
				*effectiveLimits.DownloadThreshold,
				*effectiveLimits.DownloadDailyLimit,
			), nil
		}
	}

	// Check total download limit using cumulative total
	if effectiveLimits.DownloadTotalLimit != nil {
		// Get cumulative downloaded bytes
		cumulativeTotal, err := t.getTotalBytesByType(config.UserID, models.UsageTypeDownload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		
		if cumulativeTotal+requestedBytes > *effectiveLimits.DownloadTotalLimit {
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				*effectiveLimits.DownloadTotalLimit,
			), nil
		}

		// Check threshold warning using cumulative total
		if effectiveLimits.DownloadThreshold != nil && cumulativeTotal+requestedBytes > *effectiveLimits.DownloadThreshold {
			return t.createWarningResult(
				models.EnforcementPolicyThreshold,
				cumulativeTotal,
				*effectiveLimits.DownloadThreshold,
				*effectiveLimits.DownloadTotalLimit,
			), nil
		}
	}

	return t.createSuccessResult(models.EnforcementPolicyThreshold), nil
}

// CheckStorageQuota checks if a storage operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckStorageQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := t.validateRequestedBytes(requestedBytes); err != nil {
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
		if usage.BytesStored+requestedBytes > *effectiveLimits.StorageLimit {
			return t.createLimitExceededResult(
				models.EnforcementPolicyThreshold,
				usage.BytesStored,
				*effectiveLimits.StorageLimit,
			), nil
		}

		// Check threshold warning
		if effectiveLimits.StorageThreshold != nil && usage.BytesStored+requestedBytes > *effectiveLimits.StorageThreshold {
			return t.createWarningResult(
				models.EnforcementPolicyThreshold,
				usage.BytesStored,
				*effectiveLimits.StorageThreshold,
				*effectiveLimits.StorageLimit,
			), nil
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

	// Check quota before recording
	result, err := t.CheckUploadQuotaWithConfig(config, bytes)
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

	// Check quota before recording
	result, err := t.CheckDownloadQuotaWithConfig(config, bytes)
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
			// If no default plan, continue with nil plan
		}
	}

	// Set limits from plan if available
	if plan != nil {
		if plan.StorageLimit > 0 {
			limits.StorageLimit = &plan.StorageLimit
		}
		if plan.UploadDailyLimit > 0 {
			limits.UploadDailyLimit = &plan.UploadDailyLimit
		}
		if plan.DownloadDailyLimit > 0 {
			limits.DownloadDailyLimit = &plan.DownloadDailyLimit
		}
		if plan.UploadTotalLimit > 0 {
			limits.UploadTotalLimit = &plan.UploadTotalLimit
		}
		if plan.DownloadTotalLimit > 0 {
			limits.DownloadTotalLimit = &plan.DownloadTotalLimit
		}
		if plan.StorageThreshold != nil && *plan.StorageThreshold > 0 {
			limits.StorageThreshold = plan.StorageThreshold
		}
		if plan.UploadThreshold != nil && *plan.UploadThreshold > 0 {
			limits.UploadThreshold = plan.UploadThreshold
		}
		if plan.DownloadThreshold != nil && *plan.DownloadThreshold > 0 {
			limits.DownloadThreshold = plan.DownloadThreshold
		}
		limits.QuotaPlanID = lo.ToPtr(uint64(plan.ID))
	}

	// Override with user-specific limits if set
	if config.StorageLimit != nil {
		limits.StorageLimit = config.StorageLimit
	}
	if config.UploadDailyLimit != nil {
		limits.UploadDailyLimit = config.UploadDailyLimit
	}
	if config.DownloadDailyLimit != nil {
		limits.DownloadDailyLimit = config.DownloadDailyLimit
	}
	if config.UploadTotalLimit != nil {
		limits.UploadTotalLimit = config.UploadTotalLimit
	}
	if config.DownloadTotalLimit != nil {
		limits.DownloadTotalLimit = config.DownloadTotalLimit
	}
	if config.StorageThreshold != nil {
		limits.StorageThreshold = config.StorageThreshold
	}
	if config.UploadThreshold != nil {
		limits.UploadThreshold = config.UploadThreshold
	}
	if config.DownloadThreshold != nil {
		limits.DownloadThreshold = config.DownloadThreshold
	}

	return limits, nil
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
