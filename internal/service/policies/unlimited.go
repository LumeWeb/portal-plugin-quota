package policies

import (
	"context"
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// UnlimitedPolicyEnforcer implements PolicyEnforcer for the UNLIMITED policy
type UnlimitedPolicyEnforcer struct {
	*BasePolicyEnforcer
	usageManager pluginCore.UsageManager
}

// NewUnlimitedPolicyEnforcer creates a new unlimited policy enforcer
func NewUnlimitedPolicyEnforcer(ctx core.Context, quotaService pluginCore.QuotaService) *UnlimitedPolicyEnforcer {
	return &UnlimitedPolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx, quotaService.GetUsageManager(), quotaService.GetReservationManager()),
		usageManager:       quotaService.GetUsageManager(),
	}
}

// CheckUploadQuota always allows uploads since there are no limits
func (u *UnlimitedPolicyEnforcer) CheckUploadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.CheckUploadQuota")
	defer span.End()

	return u.trackPolicyCheck(models.EnforcementPolicyUnlimited, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("config cannot be nil")
		}
		if err := u.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := u.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		result := u.createSuccessResult(models.EnforcementPolicyUnlimited)
		return result, nil
	})
}

// CheckDownloadQuota always allows downloads since there are no limits
func (u *UnlimitedPolicyEnforcer) CheckDownloadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.CheckDownloadQuota")
	defer span.End()

	return u.trackPolicyCheck(models.EnforcementPolicyUnlimited, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("config cannot be nil")
		}
		if err := u.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := u.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		result := u.createSuccessResult(models.EnforcementPolicyUnlimited)
		return result, nil
	})
}

// CheckStorageQuota always allows storage since there are no limits
func (u *UnlimitedPolicyEnforcer) CheckStorageQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.CheckStorageQuota")
	defer span.End()

	return u.trackPolicyCheck(models.EnforcementPolicyUnlimited, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("config cannot be nil")
		}
		if err := u.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := u.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		result := u.createSuccessResult(models.EnforcementPolicyUnlimited)
		return result, nil
	})
}

// RecordUpload simply records usage without any limit checking
func (u *UnlimitedPolicyEnforcer) RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.RecordUpload")
	defer span.End()

	if err := u.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return u.delegateRecordUpload(ctx, userID, uploadID, bytes, ip)
}

// RecordDownload simply records usage without any limit checking
func (u *UnlimitedPolicyEnforcer) RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.RecordDownload")
	defer span.End()

	if err := u.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return u.delegateRecordDownload(ctx, userID, uploadID, bytes, ip)
}

// RecordStorageChange simply records usage without any limit checking
func (u *UnlimitedPolicyEnforcer) RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.RecordStorageChange")
	defer span.End()

	if err := u.validateStorageRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	return u.delegateRecordStorageChange(ctx, userID, uploadID, bytes, ip)
}

// GetDetailedUsage delegates to the base enforcer
func (u *UnlimitedPolicyEnforcer) GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.GetDetailedUsage")
	defer span.End()

	return u.usageManager.GetDetailedUsage(ctx, userID, start, end)
}

// GetCurrentUsage delegates to the base enforcer
func (u *UnlimitedPolicyEnforcer) GetCurrentUsage(ctx context.Context, userID uint) (*pluginCore.Usage, error) {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.GetCurrentUsage")
	defer span.End()

	return u.usageManager.GetCurrentUsage(ctx, userID)
}

// GetUsageHistory delegates to the base enforcer
func (u *UnlimitedPolicyEnforcer) GetUsageHistory(ctx context.Context, userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	ctx, span := core.TraceMethod(ctx, "UnlimitedPolicyEnforcer.GetUsageHistory")
	defer span.End()

	return u.usageManager.GetUsageHistory(ctx, userID, period, usageType)
}
