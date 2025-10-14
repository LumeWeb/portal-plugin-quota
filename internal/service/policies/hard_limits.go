package policies

import (
	"fmt"
	"time"

	"github.com/docker/go-units"
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
func (h *HardLimitsPolicyEnforcer) CheckUploadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
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
	limits, err := h.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get today's usage
	usage, err := h.quotaService.GetTodayUsage(config.UserID)
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
				},
			), nil
		}
	}

	// Check total upload limit against aggregated usage
	if limits.UploadTotalLimit != nil {
		aggregatedUsage, err := h.quotaService.GetUsageAggregator().GetAggregatedUsageByType(config.UserID, models.UsageTypeUpload)
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

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// CheckDownloadQuota checks if a download operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
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
	limits, err := h.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get today's usage
	usage, err := h.quotaService.GetTodayUsage(config.UserID)
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
		aggregatedUsage, err := h.quotaService.GetUsageAggregator().GetAggregatedUsageByType(config.UserID, models.UsageTypeDownload)
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

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// CheckStorageQuota checks if a storage operation is allowed under hard limits policy
func (h *HardLimitsPolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
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
	limits, err := h.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	// Get current usage
	usage, err := h.quotaService.GetTodayUsage(config.UserID)
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

	return h.createSuccessResult(models.EnforcementPolicyHardLimits), nil
}

// RecordUpload records an upload operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := h.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := h.quotaService.GetUsageManager().GetUserQuotaConfig(userID)
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

	return h.delegateRecordUpload(userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation and enforces hard limits
func (h *HardLimitsPolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := h.validateUserID(userID); err != nil {
		return err
	}
	if err := h.validateRequestedBytes(bytes); err != nil {
		return err
	}

	// Get user's quota config
	config, err := h.quotaService.GetUsageManager().GetUserQuotaConfig(userID)
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

	// Delegate to BasePolicyEnforcer for actual recording (includes validation)
	return h.delegateRecordDownload(userID, uploadID, bytes, ip)
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
	config, err := h.quotaService.GetUsageManager().GetUserQuotaConfig(userID)
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

	// Delegate to UsageManager for actual recording
	return h.quotaService.GetUsageManager().RecordStorageChange(userID, uploadID, bytes, ip)
}

// GetDetailedUsage delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return h.quotaService.GetUsageManager().GetDetailedUsage(userID, start, end)
}

// GetCurrentUsage delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return h.quotaService.GetUsageManager().GetCurrentUsage(userID)
}

// GetUsageHistory delegates to base enforcer
func (h *HardLimitsPolicyEnforcer) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	return h.quotaService.GetUsageManager().GetUsageHistory(userID, period, usageType)
}

// validateLimitValue validates that a limit value is reasonable
func (h *HardLimitsPolicyEnforcer) validateLimitValue(value int64) error {
	// Allow -1 (unlimited), 0 (disabled), and positive values
	if value < -1 {
		return fmt.Errorf("invalid limit value: %d (must be -1, 0, or positive)", value)
	}

	// Check if the value is unreasonably large (1 PiB should be enough for most use cases)
	if value > 0 && value > int64(units.PiB) {
		return fmt.Errorf("limit value %d is unreasonably large", value)
	}

	return nil
}
