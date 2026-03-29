package policies

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// AllowancePolicyEnforcer implements PolicyEnforcer for the ALLOWANCE policy
type AllowancePolicyEnforcer struct {
	*BasePolicyEnforcer
	quotaService  pluginCore.QuotaService
	limitResolver pluginCore.LimitResolver
}

// NewAllowancePolicyEnforcer creates a new allowance policy enforcer
func NewAllowancePolicyEnforcer(ctx core.Context, quotaService pluginCore.QuotaService) *AllowancePolicyEnforcer {
	return &AllowancePolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx, quotaService.GetUsageManager()),
		quotaService:       quotaService,
		limitResolver:      NewLimitResolver(ctx, quotaService),
	}
}

// CheckUploadQuota checks if an upload operation is allowed under allowance policy
func (a *AllowancePolicyEnforcer) CheckUploadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.CheckUploadQuota")
	defer span.End()

	return a.trackPolicyCheck(models.EnforcementPolicyAllowance, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, errors.New("config cannot be nil")
		}
		if err := a.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := a.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get active grants for upload
		grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(ctx, config.UserID, models.GrantTypeUpload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get active upload grants: %w", err)
		}

		// Calculate available bytes from grants
		availableBytes := a.quotaService.GetGrantManager().CalculateAvailableBytes(grants)

		// Calculate used bytes
		used := uint64(0)
		for _, grant := range grants {
			used += grant.BytesUsed
		}

		details := pluginCore.QuotaCheckDetails{
			CurrentUsage:  used,
			Allowance:     &availableBytes,
			AllowanceUsed: &used,
			Policy:        pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance),
		}

		if requestedBytes > availableBytes {
			return a.createFailureResult(models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, details), nil
		}

		return a.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyAllowance, details), nil
	})
}

// CheckDownloadQuota checks if a download operation is allowed under allowance policy
func (a *AllowancePolicyEnforcer) CheckDownloadQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.CheckDownloadQuota")
	defer span.End()

	return a.trackPolicyCheck(models.EnforcementPolicyAllowance, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, errors.New("config cannot be nil")
		}
		if err := a.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := a.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get active grants for download
		grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(ctx, config.UserID, models.GrantTypeDownload)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get active download grants: %w", err)
		}

		// Calculate available bytes from grants
		availableBytes := a.quotaService.GetGrantManager().CalculateAvailableBytes(grants)

		// Calculate used bytes
		used := uint64(0)
		for _, grant := range grants {
			used += grant.BytesUsed
		}

		details := pluginCore.QuotaCheckDetails{
			CurrentUsage:  used,
			Allowance:     &availableBytes,
			AllowanceUsed: &used,
			Policy:        pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance),
		}

		if requestedBytes > availableBytes {
			return a.createFailureResult(models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, details), nil
		}

		return a.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyAllowance, details), nil
	})
}

// CheckStorageQuota checks if a storage operation is allowed under allowance policy
func (a *AllowancePolicyEnforcer) CheckStorageQuota(ctx context.Context, config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.CheckStorageQuota")
	defer span.End()

	return a.trackPolicyCheck(models.EnforcementPolicyAllowance, func() (pluginCore.QuotaCheckResult, error) {
		if config == nil {
			return pluginCore.QuotaCheckResult{}, errors.New("config cannot be nil")
		}
		if err := a.validateRequestedBytes(requestedBytes); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}
		if err := a.validateUserID(config.UserID); err != nil {
			return pluginCore.QuotaCheckResult{}, err
		}

		// Get active grants for storage
		grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(ctx, config.UserID, models.GrantTypeStorage)
		if err != nil {
			return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get active storage grants: %w", err)
		}

		// Calculate available bytes from grants
		availableBytes := a.quotaService.GetGrantManager().CalculateAvailableBytes(grants)

		// Calculate used bytes
		used := uint64(0)
		for _, grant := range grants {
			used += grant.BytesUsed
		}

		details := pluginCore.QuotaCheckDetails{
			CurrentUsage:  used,
			Allowance:     &availableBytes,
			AllowanceUsed: &used,
			Policy:        pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance),
		}

		if requestedBytes > availableBytes {
			return a.createFailureResult(models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, details), nil
		}

		return a.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyAllowance, details), nil
	})
}

// RecordUpload records an upload operation and consumes allowance
func (a *AllowancePolicyEnforcer) RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.RecordUpload")
	defer span.End()

	if err := a.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	// Create usage detail record
	usageDetail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeUpload,
		Bytes:      bytes,
		IP:         models.IPAddr(ip),
		SharedWith: 1,
		Timestamp:  time.Now().UTC(),
	}

	// Record usage and consume allowance in a single transaction
	if err := a.quotaService.GetUsageManager().RecordUsageAndConsume(ctx, usageDetail, models.GrantTypeUpload, bytes); err != nil {
		return err
	}

	return a.delegateRecordUpload(ctx, userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation and consumes allowance
func (a *AllowancePolicyEnforcer) RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.RecordDownload")
	defer span.End()

	if err := a.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	// Create usage detail record
	usageDetail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeDownload,
		Bytes:      bytes,
		IP:         models.IPAddr(ip),
		SharedWith: 1,
		Timestamp:  time.Now().UTC(),
	}

	// Record usage and consume allowance in a single transaction
	if err := a.quotaService.GetUsageManager().RecordUsageAndConsume(ctx, usageDetail, models.GrantTypeDownload, bytes); err != nil {
		return err
	}

	return a.delegateRecordDownload(ctx, userID, uploadID, bytes, ip)
}

// RecordStorageChange records a storage change operation and consumes allowance
func (a *AllowancePolicyEnforcer) RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.RecordStorageChange")
	defer span.End()

	if err := a.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Determine usage type for recording
	var usageType models.UsageType
	var recordBytes uint64

	if bytes < 0 {
		usageType = models.UsageTypeStorageRemove
		// Handle math.MinInt64 case to prevent overflow when converting to uint64
		// math.MinInt64 is -9223372036854775808, which when negated would exceed math.MaxInt64 (9223372036854775807)
		// Since uint64(-math.MinInt64) would cause overflow, we use 1 << 63 which equals 9223372036854775808
		// This represents the absolute value of math.MinInt64 without causing overflow
		if bytes == math.MinInt64 {
			recordBytes = 1 << 63
		} else {
			recordBytes = uint64(-bytes)
		}
	} else {
		usageType = models.UsageTypeStorageAdd
		recordBytes = uint64(bytes)
	}

	// Create usage detail record
	usageDetail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       usageType,
		Bytes:      recordBytes,
		IP:         models.IPAddr(ip),
		SharedWith: 1,
		Timestamp:  time.Now().UTC(),
	}

	// For storage changes, we only consume allowance when adding storage (positive bytes)
	if bytes > 0 {
		// Record usage and consume allowance in a single transaction
		if err := a.quotaService.GetUsageManager().RecordUsageAndConsume(ctx, usageDetail, models.GrantTypeStorage, uint64(bytes)); err != nil {
			return err
		}
	} else {
		// For storage removal, just record the usage detail without consuming allowance
		if err := a.quotaService.GetUsageManager().RecordUserUsageDetail(ctx, usageDetail, nil); err != nil {
			return err
		}
	}

	// Validate and delegate (ensures uploadID > 0 and non-zero bytes)
	if err := a.validateStorageRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}
	return a.delegateRecordStorageChange(ctx, userID, uploadID, bytes, ip)
}

// GetDetailedUsage delegates to the usage manager
func (a *AllowancePolicyEnforcer) GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.GetDetailedUsage")
	defer span.End()

	return a.quotaService.GetUsageManager().GetDetailedUsage(ctx, userID, start, end)
}

// GetCurrentUsage delegates to the usage manager
func (a *AllowancePolicyEnforcer) GetCurrentUsage(ctx context.Context, userID uint) (*pluginCore.Usage, error) {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.GetCurrentUsage")
	defer span.End()

	return a.quotaService.GetUsageManager().GetCurrentUsage(ctx, userID)
}

// GetUsageHistory delegates to the usage manager
func (a *AllowancePolicyEnforcer) GetUsageHistory(ctx context.Context, userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	ctx, span := core.TraceMethod(ctx, "AllowancePolicyEnforcer.GetUsageHistory")
	defer span.End()

	return a.quotaService.GetUsageManager().GetUsageHistory(ctx, userID, period, usageType)
}
