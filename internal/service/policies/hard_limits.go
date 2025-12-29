package policies

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// HardLimitsPolicyEnforcer implements PolicyEnforcer for hard limits policy
type HardLimitsPolicyEnforcer struct {
	*BasePolicyEnforcer
	quotaService  pluginCore.QuotaService
	limitResolver pluginCore.LimitResolver
}

// NewHardLimitsPolicyEnforcer creates a new hard limits policy enforcer
func NewHardLimitsPolicyEnforcer(ctx core.Context, quotaService pluginCore.QuotaService) *HardLimitsPolicyEnforcer {
	return &HardLimitsPolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx, quotaService.GetUsageManager()),
		quotaService:       quotaService,
		limitResolver:      NewLimitResolver(ctx, quotaService),
	}
}

// CheckUploadQuota checks if an upload operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckUploadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.CheckUploadQuota")
	defer span.End()

	return h.trackPolicyCheck(models.EnforcementPolicyHardLimits, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("quota config is nil")
		}

		if err := h.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := h.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Resolve effective limits for the user
		limits, err := h.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get today's usage
		usage, err := h.quotaService.GetTodayUsage(ctx, config.UserID)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Check daily upload limit
		if limits.UploadDailyLimit != nil {
			limitValue := uint64(*limits.UploadDailyLimit)

			if limitValue == 0 {
				// Limit is 0, which means disabled - deny the operation
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: usage.BytesUploaded,
						Limit:        lo.ToPtr(uint64(0)),
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			} else if limitValue < usage.BytesUploaded || requestedBytes > limitValue-usage.BytesUploaded {
				// Normal limit check for positive values using overflow-safe subtraction
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: usage.BytesUploaded,
						Limit:        &limitValue,
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			}
		}

		// Check total upload limit against aggregated usage
		if limits.UploadTotalLimit != nil {
			aggregatedUsage, err := h.quotaService.GetUsageAggregator().GetAggregatedUsageByType(ctx, config.UserID, models.UsageTypeUpload)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}

			limitValue := uint64(*limits.UploadTotalLimit)

			if limitValue == 0 {
				// Limit is 0, which means disabled - deny the operation
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: aggregatedUsage,
						Limit:        lo.ToPtr(uint64(0)),
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			} else if limitValue < aggregatedUsage || requestedBytes > limitValue-aggregatedUsage {
				// Normal limit check for positive values using overflow-safe subtraction
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: aggregatedUsage,
						Limit:        &limitValue,
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			}
		}

		result := h.createSuccessResult(models.EnforcementPolicyHardLimits)
		return result, nil
	})
}

// CheckDownloadQuota checks if a download operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckDownloadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.CheckDownloadQuota")
	defer span.End()

	return h.trackPolicyCheck(models.EnforcementPolicyHardLimits, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("quota config is nil")
		}

		if err := h.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := h.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get effective limits for the user
		limits, err := h.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get today's usage
		usage, err := h.quotaService.GetTodayUsage(ctx, config.UserID)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Check daily download limit
		if limits.DownloadDailyLimit != nil {
			limitValue := uint64(*limits.DownloadDailyLimit)
			if limitValue == 0 {
				// Limit is 0, which means disabled - deny the operation
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: usage.BytesDownloaded,
						Limit:        lo.ToPtr(uint64(0)),
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			} else if limitValue < usage.BytesDownloaded || requestedBytes > limitValue-usage.BytesDownloaded {
				// Normal limit check for positive values using overflow-safe subtraction
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: usage.BytesDownloaded,
						Limit:        &limitValue,
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			}
		}

		// Check total download limit against aggregated usage
		if limits.DownloadTotalLimit != nil {
			aggregatedUsage, err := h.quotaService.GetUsageAggregator().GetAggregatedUsageByType(ctx, config.UserID, models.UsageTypeDownload)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, err
			}

			limitValue := uint64(*limits.DownloadTotalLimit)
			if limitValue == 0 {
				// Limit is 0, which means disabled - deny the operation
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: aggregatedUsage,
						Limit:        lo.ToPtr(uint64(0)),
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			} else if limitValue < aggregatedUsage || requestedBytes > limitValue-aggregatedUsage {
				// Normal limit check for positive values using overflow-safe subtraction
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: aggregatedUsage,
						Limit:        &limitValue,
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			}
		}

		result := h.createSuccessResult(models.EnforcementPolicyHardLimits)
		return result, nil
	})
}

// CheckStorageQuota checks if a storage operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckStorageQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.CheckStorageQuota")
	defer span.End()

	return h.trackPolicyCheck(models.EnforcementPolicyHardLimits, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("quota config is nil")
		}

		if err := h.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := h.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get effective limits for the user
		limits, err := h.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get current usage
		usage, err := h.quotaService.GetTodayUsage(ctx, config.UserID)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Check storage limit
		if limits.StorageLimit != nil {
			limitValue := uint64(*limits.StorageLimit)
			if limitValue == 0 {
				// Limit is 0, which means disabled - deny the operation
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: usage.BytesStored,
						Limit:        lo.ToPtr(uint64(0)),
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			} else if limitValue < usage.BytesStored || requestedBytes > limitValue-usage.BytesStored {
				// Normal limit check for positive values using overflow-safe subtraction
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: usage.BytesStored,
						Limit:        &limitValue,
						Policy:       models.EnforcementPolicyHardLimits,
					},
				), nil
			}
		}

		result := h.createSuccessResult(models.EnforcementPolicyHardLimits)
		return result, nil
	})
}

// RecordUpload records an upload operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.RecordUpload")
	defer span.End()

	if err := h.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := h.quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
	if err != nil {
		return err
	}

	// Check quota before recording
	result, err := h.CheckUploadQuota(ctx, config, bytes)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("upload blocked: %s", result.Reason)
	}

	return h.delegateRecordUpload(ctx, userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.RecordDownload")
	defer span.End()

	if err := h.validateUserID(userID); err != nil {
		return err
	}
	if err := h.validateRequestedBytes(bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := h.quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
	if err != nil {
		return err
	}

	// Check quota before recording
	result, err := h.CheckDownloadQuota(ctx, config, bytes)
	if err != nil {
		return err
	}

	if !result.Allowed {
		return fmt.Errorf("download blocked: %s", result.Reason)
	}

	// Delegate to BasePolicyEnforcer for actual recording (includes validation)
	return h.delegateRecordDownload(ctx, userID, uploadID, bytes, ip)
}

// RecordStorageChange records a storage change operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.RecordStorageChange")
	defer span.End()

	if err := h.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Get user's quota config
	config, err := h.quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
	if err != nil {
		return err
	}

	// For storage changes, we only enforce limits when adding storage
	if bytes > 0 {
		// Check quota before recording
		result, err := h.CheckStorageQuota(ctx, config, uint64(bytes))
		if err != nil {
			return err
		}

		if !result.Allowed {
			return fmt.Errorf("storage change blocked: %s", result.Reason)
		}
	}

	// Delegate to UsageManager for actual recording
	return h.quotaService.GetUsageManager().RecordStorageChange(ctx, userID, uploadID, bytes, ip)
}

// GetDetailedUsage delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.GetDetailedUsage")
	defer span.End()

	return h.quotaService.GetUsageManager().GetDetailedUsage(ctx, userID, start, end)
}

// GetCurrentUsage delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetCurrentUsage(ctx context.Context, userID uint) (*pluginCore.Usage, error) {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.GetCurrentUsage")
	defer span.End()

	return h.quotaService.GetUsageManager().GetCurrentUsage(ctx, userID)
}

// GetUsageHistory delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetUsageHistory(ctx context.Context, userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	ctx, span := core.TraceMethod(ctx, "HardLimitsPolicyEnforcer.GetUsageHistory")
	defer span.End()

	return h.quotaService.GetUsageManager().GetUsageHistory(ctx, userID, period, usageType)
}
