package policies

import (
	"context"
	"fmt"
	"time"

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

		// Check upload limit (window-based)
		if limits.UploadLimitConfig != nil && limits.UploadLimitConfig.Bytes > 0 {
			windowLimits := *limits.UploadLimitConfig

			// Validate window configuration
			if err := windowLimits.Window.Validate(); err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("invalid upload window configuration: %w", err)
			}

			// Query usage for this window
			currentUsage, _, _, err := h.quotaService.GetUsageManager().GetUsageForWindow(
				ctx, config.UserID, models.UsageTypeUpload, windowLimits.Window)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get usage for upload window: %w", err)
			}

			limitValue := windowLimits.Bytes

			// Normal limit check for positive values using overflow-safe subtraction
			if limitValue < currentUsage || requestedBytes > limitValue-currentUsage {
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: currentUsage,
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

		// Check download limit (window-based)
		if limits.DownloadLimitConfig != nil && limits.DownloadLimitConfig.Bytes > 0 {
			windowLimits := *limits.DownloadLimitConfig

			// Validate window configuration
			if err := windowLimits.Window.Validate(); err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("invalid download window configuration: %w", err)
			}

			// Query usage for this window
			currentUsage, _, _, err := h.quotaService.GetUsageManager().GetUsageForWindow(
				ctx, config.UserID, models.UsageTypeDownload, windowLimits.Window)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get usage for download window: %w", err)
			}

			limitValue := windowLimits.Bytes

			// Normal limit check for positive values using overflow-safe subtraction
			if limitValue < currentUsage || requestedBytes > limitValue-currentUsage {
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: currentUsage,
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

		// Check storage limit (window-based)
		if limits.StorageLimitConfig != nil && limits.StorageLimitConfig.Bytes > 0 {
			windowLimits := *limits.StorageLimitConfig

			// Validate window configuration
			if err := windowLimits.Window.Validate(); err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("invalid storage window configuration: %w", err)
			}

			// Query usage for this window
			currentUsage, _, _, err := h.quotaService.GetUsageManager().GetUsageForWindow(
				ctx, config.UserID, models.UsageTypeStorageAdd, windowLimits.Window)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get usage for storage window: %w", err)
			}

			limitValue := windowLimits.Bytes

			// Normal limit check for positive values using overflow-safe subtraction
			if limitValue < currentUsage || requestedBytes > limitValue-currentUsage {
				return h.createFailureResult(
					models.QuotaCheckReasonLimitExceeded,
					models.EnforcementPolicyHardLimits,
					pluginCore.QuotaCheckDetails{
						CurrentUsage: currentUsage,
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
