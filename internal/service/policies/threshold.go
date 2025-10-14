package policies

import (
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// ThresholdPolicyEnforcer implements the PolicyEnforcer interface for THRESHOLD policy
type ThresholdPolicyEnforcer struct {
	*BasePolicyEnforcer
	quotaService  pluginCore.QuotaService
	limitResolver pluginCore.LimitResolver
}

// NewThresholdPolicyEnforcer creates a new threshold policy enforcer
func NewThresholdPolicyEnforcer(ctx core.Context, quotaService pluginCore.QuotaService) *ThresholdPolicyEnforcer {
	return &ThresholdPolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx, quotaService.GetUsageManager()),
		quotaService:       quotaService,
		limitResolver:      NewLimitResolver(ctx, quotaService),
	}
}

// CheckUploadQuota checks if an upload operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckUploadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if config == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config cannot be nil")
	}

	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	limits, err := t.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get today's usage
	usage, err := t.quotaService.GetTodayUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Cache total usage if we need to check total limits or use in details
	var totalUsage uint64
	haveTotal := false

	// Check daily upload limit
	if limits.UploadDailyLimit != nil {
		result := t.checkThresholdWithLimit(
			usage.BytesUploaded,
			requestedBytes,
			limits.UploadThreshold,
			*limits.UploadDailyLimit,
			models.EnforcementPolicyThreshold,
		)
		if result != nil {
			return *result, nil
		}
	}

	// Check total upload limit
	if limits.UploadTotalLimit != nil {
		tu, err := t.quotaService.GetUsageManager().GetTotalBytesByType(config.UserID, models.UsageTypeUpload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		totalUsage = tu
		haveTotal = true

		result := t.checkThresholdWithLimit(
			totalUsage,
			requestedBytes,
			limits.UploadThreshold,
			*limits.UploadTotalLimit,
			models.EnforcementPolicyThreshold,
		)
		if result != nil {
			return *result, nil
		}
	}

	// Create success result with current usage and limit, but no threshold
	var details pluginCore.QuotaCheckDetails
	if limits.UploadDailyLimit != nil {
		details = pluginCore.QuotaCheckDetails{
			CurrentUsage: usage.BytesUploaded,
			Limit:        limits.UploadDailyLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
	} else if limits.UploadTotalLimit != nil {
		if !haveTotal {
			// Re-fetch total usage for consistency with test expectations
			tu, err := t.quotaService.GetUsageManager().GetTotalBytesByType(config.UserID, models.UsageTypeUpload)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}
			totalUsage = tu
		}
		details = pluginCore.QuotaCheckDetails{
			CurrentUsage: totalUsage,
			Limit:        limits.UploadTotalLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
	} else {
		details = pluginCore.QuotaCheckDetails{
			CurrentUsage: usage.BytesUploaded,
			Limit:        nil,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
	}
	return t.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyThreshold, details), nil
}

// CheckDownloadQuota checks if a download operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if config == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config cannot be nil")
	}

	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	limits, err := t.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get today's usage
	usage, err := t.quotaService.GetTodayUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Cache total usage if we need to check total limits or use in details
	var totalUsage uint64
	haveTotal := false

	// Check daily download limit
	if limits.DownloadDailyLimit != nil {
		result := t.checkThresholdWithLimit(
			usage.BytesDownloaded,
			requestedBytes,
			limits.DownloadThreshold,
			*limits.DownloadDailyLimit,
			models.EnforcementPolicyThreshold,
		)
		if result != nil {
			return *result, nil
		}
	}

	// Check total download limit
	if limits.DownloadTotalLimit != nil {
		tu, err := t.quotaService.GetUsageManager().GetTotalBytesByType(config.UserID, models.UsageTypeDownload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		totalUsage = tu
		haveTotal = true

		result := t.checkThresholdWithLimit(
			totalUsage,
			requestedBytes,
			limits.DownloadThreshold,
			*limits.DownloadTotalLimit,
			models.EnforcementPolicyThreshold,
		)
		if result != nil {
			return *result, nil
		}
	}

	// Create success result with current usage and limit, but no threshold
	var details pluginCore.QuotaCheckDetails
	if limits.DownloadDailyLimit != nil {
		details = pluginCore.QuotaCheckDetails{
			CurrentUsage: usage.BytesDownloaded,
			Limit:        limits.DownloadDailyLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
	} else if limits.DownloadTotalLimit != nil {
		if !haveTotal {
			// Re-fetch total usage for consistency with test expectations
			tu, err := t.quotaService.GetUsageManager().GetTotalBytesByType(config.UserID, models.UsageTypeDownload)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}
			totalUsage = tu
		}
		details = pluginCore.QuotaCheckDetails{
			CurrentUsage: totalUsage,
			Limit:        limits.DownloadTotalLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
	} else {
		details = pluginCore.QuotaCheckDetails{
			CurrentUsage: usage.BytesDownloaded,
			Limit:        nil,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
	}
	return t.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyThreshold, details), nil
}

// CheckStorageQuota checks if a storage operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if config == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config cannot be nil")
	}

	if err := t.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := t.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Resolve effective limits
	limits, err := t.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get current usage
	usage, err := t.quotaService.GetTodayUsage(config.UserID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Check storage limit
	if limits.StorageLimit != nil {
		result := t.checkThresholdWithLimit(
			usage.BytesStored,
			requestedBytes,
			limits.StorageThreshold,
			*limits.StorageLimit,
			models.EnforcementPolicyThreshold,
		)
		if result != nil {
			return *result, nil
		}
	}

	// Create success result with current usage and limit, but no threshold
	details := pluginCore.QuotaCheckDetails{
		CurrentUsage: usage.BytesStored,
		Limit:        limits.StorageLimit,
		Policy:       models.EnforcementPolicyThreshold,
		Threshold:    nil, // Explicitly set to nil when no warning
	}
	return t.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyThreshold, details), nil
}

// checkThresholdWithLimit is a helper method that checks threshold logic for a given limit
func (t *ThresholdPolicyEnforcer) checkThresholdWithLimit(
	currentUsage uint64,
	requestedBytes uint64,
	threshold *uint64,
	limit uint64,
	policy models.EnforcementPolicy,
) *pluginCore.QuotaCheckResult {
	// Check if limit would be exceeded
	// First check if already over limit, then check if requested bytes would push over limit
	// This prevents uint64 overflow when currentUsage + requestedBytes > max uint64
	if currentUsage >= limit || requestedBytes > limit-currentUsage {
		details := pluginCore.QuotaCheckDetails{
			CurrentUsage: currentUsage,
			Limit:        &limit,
			Policy:       policy,
		}
		result := t.createFailureResult(models.QuotaCheckReasonLimitExceeded, policy, details)
		return &result
	}

	// Check threshold warning
	if threshold != nil {
		thresholdResult := pluginCore.EvaluateThreshold(currentUsage, requestedBytes, *threshold, limit)
		if thresholdResult.ShouldWarn {
			result := t.createWarningResult(policy, currentUsage, *threshold, limit)
			return &result
		}
	}

	// No action needed
	return nil
}

// RecordUpload records an upload operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := t.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return t.delegateRecordUpload(userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := t.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return t.delegateRecordDownload(userID, uploadID, bytes, ip)
}

// RecordStorageChange records a storage change operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := t.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Delegate to UsageManager for actual recording
	return t.quotaService.GetUsageManager().RecordStorageChange(userID, uploadID, bytes, ip)
}

// GetDetailedUsage returns detailed usage records for a user
func (t *ThresholdPolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return t.quotaService.GetUsageManager().GetDetailedUsage(userID, start, end)
}

// GetCurrentUsage returns current usage statistics for a user
func (t *ThresholdPolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return t.quotaService.GetUsageManager().GetCurrentUsage(userID)
}

// GetUsageHistory returns usage history for a user
func (t *ThresholdPolicyEnforcer) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	return t.quotaService.GetUsageManager().GetUsageHistory(userID, period, usageType)
}
