package policies

import (
	"context"
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
func (t *ThresholdPolicyEnforcer) CheckUploadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.CheckUploadQuota")
	defer span.End()

	return t.trackPolicyCheck(models.EnforcementPolicyThreshold, func() (pluginCore.QuotaCheckResult, error) {
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
		limits, err := t.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyThreshold)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get upload limit from config
		uploadLimit := t.extractLimitBytes(limits.UploadLimitConfig)
		if uploadLimit != nil {
			// Get usage for upload window
			windowUsage, err := t.getUsageForWindow(ctx, config.UserID, limits.UploadLimitConfig, models.UsageTypeUpload, 0)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}

			result := t.checkThresholdWithLimit(
				windowUsage,
				requestedBytes,
				limits.UploadThreshold,
				*uploadLimit,
				models.EnforcementPolicyThreshold,
			)
			if result != nil {
				return *result, nil
			}
		}

		// Create success result
		windowUsage, err := t.getUsageForWindow(ctx, config.UserID, limits.UploadLimitConfig, models.UsageTypeUpload, 0)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		details := pluginCore.QuotaCheckDetails{
			CurrentUsage: windowUsage,
			Limit:        uploadLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
		return t.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyThreshold, details), nil
	})
}

// CheckDownloadQuota checks if a download operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckDownloadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.CheckDownloadQuota")
	defer span.End()

	return t.trackPolicyCheck(models.EnforcementPolicyThreshold, func() (pluginCore.QuotaCheckResult, error) {
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
		limits, err := t.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyThreshold)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get download limit from config
		downloadLimit := t.extractLimitBytes(limits.DownloadLimitConfig)
		if downloadLimit != nil {
			// Get usage for download window
			windowUsage, err := t.getUsageForWindow(ctx, config.UserID, limits.DownloadLimitConfig, models.UsageTypeDownload, 0)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}

			result := t.checkThresholdWithLimit(
				windowUsage,
				requestedBytes,
				limits.DownloadThreshold,
				*downloadLimit,
				models.EnforcementPolicyThreshold,
			)
			if result != nil {
				return *result, nil
			}
		}

		// Create success result
		windowUsage, err := t.getUsageForWindow(ctx, config.UserID, limits.DownloadLimitConfig, models.UsageTypeDownload, 0)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		details := pluginCore.QuotaCheckDetails{
			CurrentUsage: windowUsage,
			Limit:        downloadLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
		return t.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyThreshold, details), nil
	})
}

// CheckStorageQuota checks if a storage operation is allowed under threshold policy
func (t *ThresholdPolicyEnforcer) CheckStorageQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.CheckStorageQuota")
	defer span.End()

	return t.trackPolicyCheck(models.EnforcementPolicyThreshold, func() (pluginCore.QuotaCheckResult, error) {
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
		limits, err := t.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyThreshold)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get storage limit from config
		storageLimit := t.extractLimitBytes(limits.StorageLimitConfig)
		if storageLimit != nil {
			// Get usage for storage window
			windowUsage, err := t.getUsageForWindow(ctx, config.UserID, limits.StorageLimitConfig, models.UsageTypeStorageAdd, 0)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}

			result := t.checkThresholdWithLimit(
				windowUsage,
				requestedBytes,
				limits.StorageThreshold,
				*storageLimit,
				models.EnforcementPolicyThreshold,
			)
			if result != nil {
				return *result, nil
			}
		}

		// Create success result
		windowUsage, err := t.getUsageForWindow(ctx, config.UserID, limits.StorageLimitConfig, models.UsageTypeStorageAdd, 0)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		details := pluginCore.QuotaCheckDetails{
			CurrentUsage: windowUsage,
			Limit:        storageLimit,
			Policy:       models.EnforcementPolicyThreshold,
			Threshold:    nil, // Explicitly set to nil when no warning
		}
		return t.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyThreshold, details), nil
	})
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
			result := t.createWarningResult(policy, thresholdResult.CurrentUsage, *threshold, limit)
			return &result
		}
	}

	// No action needed
	return nil
}

// RecordUpload records an upload operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.RecordUpload")
	defer span.End()

	if err := t.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return t.delegateRecordUpload(ctx, userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.RecordDownload")
	defer span.End()

	if err := t.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return t.delegateRecordDownload(ctx, userID, uploadID, bytes, ip)
}

// RecordStorageChange records a storage change operation under threshold policy
func (t *ThresholdPolicyEnforcer) RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.RecordStorageChange")
	defer span.End()

	if err := t.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Delegate to BasePolicyEnforcer for validation and recording
	return t.delegateRecordStorageChange(ctx, userID, uploadID, bytes, ip)
}

// GetDetailedUsage returns detailed usage records for a user
func (t *ThresholdPolicyEnforcer) GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.GetDetailedUsage")
	defer span.End()

	return t.quotaService.GetUsageManager().GetDetailedUsage(ctx, userID, start, end)
}

// GetCurrentUsage returns current usage statistics for a user
func (t *ThresholdPolicyEnforcer) GetCurrentUsage(ctx context.Context, userID uint) (*pluginCore.Usage, error) {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.GetCurrentUsage")
	defer span.End()

	return t.quotaService.GetUsageManager().GetCurrentUsage(ctx, userID)
}

// GetUsageHistory returns usage history for a user
func (t *ThresholdPolicyEnforcer) GetUsageHistory(ctx context.Context, userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	ctx, span := core.TraceMethod(ctx, "ThresholdPolicyEnforcer.GetUsageHistory")
	defer span.End()

	return t.quotaService.GetUsageManager().GetUsageHistory(ctx, userID, period, usageType)
}

// extractLimitBytes extracts the byte limit from a LimitConfig
func (t *ThresholdPolicyEnforcer) extractLimitBytes(limitConfig *pluginCore.Limit) *uint64 {
	if limitConfig == nil {
		return nil
	}
	return &limitConfig.Bytes
}

// getUsageForWindow gets usage for the configured window, falls back to provided default usage
func (t *ThresholdPolicyEnforcer) getUsageForWindow(
	ctx context.Context,
	userID uint,
	limitConfig *pluginCore.Limit,
	usageType models.UsageType,
	defaultUsage uint64,
) (uint64, error) {
	if limitConfig == nil || limitConfig.Window.IsNil() {
		// No window configured, use default usage
		return defaultUsage, nil
	}

	// Get usage for the configured window
	usageAggregator := t.quotaService.GetUsageManager()
	windowUsage, _, _, err := usageAggregator.GetUsageForWindow(
		ctx, userID, pluginCore.UsageType(usageType), limitConfig.Window)
	if err != nil {
		return 0, err
	}

	return windowUsage, nil
}
