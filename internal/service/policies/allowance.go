package policies

import (
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// AllowancePolicyEnforcer implements PolicyEnforcer for the ALLOWANCE policy
type AllowancePolicyEnforcer struct {
	*BasePolicyEnforcer
	grantManager pluginCore.GrantManager
	usageManager pluginCore.UsageManager
}

// NewAllowancePolicyEnforcer creates a new allowance policy enforcer
func NewAllowancePolicyEnforcer(ctx core.Context, grantManager pluginCore.GrantManager, usageManager pluginCore.UsageManager) *AllowancePolicyEnforcer {
	base := NewBasePolicyEnforcer(ctx)
	return &AllowancePolicyEnforcer{
		BasePolicyEnforcer: base,
		grantManager:       grantManager,
		usageManager:       usageManager,
	}
}

// CheckUploadQuota checks if an upload operation is allowed under the allowance policy
func (a *AllowancePolicyEnforcer) CheckUploadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := a.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := a.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	userID := config.UserID

	// Get active grants for upload
	grants, err := a.grantManager.GetActiveGrantsByType(userID, models.GrantTypeUpload)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get active upload grants: %w", err)
	}

	// Calculate available bytes
	availableBytes := a.grantManager.CalculateAvailableBytes(grants)

	// Check if we have enough allowance
	if requestedBytes > availableBytes {
		return a.createQuotaCheckResult(false, models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, pluginCore.QuotaCheckDetails{
			CurrentUsage: 0, // Current usage isn't tracked in allowance policy the same way
			Allowance:    &availableBytes,
			AllowanceUsed: func() *uint64 {
				used := uint64(0)
				for _, grant := range grants {
					used += grant.BytesUsed
				}
				return &used
			}(),
			Policy: pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance),
		}), nil
	}

	return a.createSuccessResult(models.EnforcementPolicyAllowance), nil
}

// CheckDownloadQuota checks if a download operation is allowed under the allowance policy
func (a *AllowancePolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := a.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := a.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	userID := config.UserID

	// Get active grants for download
	grants, err := a.grantManager.GetActiveGrantsByType(userID, models.GrantTypeDownload)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get active download grants: %w", err)
	}

	// Calculate available bytes
	availableBytes := a.grantManager.CalculateAvailableBytes(grants)

	// Check if we have enough allowance
	if requestedBytes > availableBytes {
		return a.createQuotaCheckResult(false, models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, pluginCore.QuotaCheckDetails{
			CurrentUsage: 0, // Current usage isn't tracked in allowance policy the same way
			Allowance:    &availableBytes,
			AllowanceUsed: func() *uint64 {
				used := uint64(0)
				for _, grant := range grants {
					used += grant.BytesUsed
				}
				return &used
			}(),
			Policy: pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance),
		}), nil
	}

	return a.createSuccessResult(models.EnforcementPolicyAllowance), nil
}

// CheckStorageQuota checks if a storage operation is allowed under the allowance policy
func (a *AllowancePolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := a.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := a.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	userID := config.UserID

	// Get active grants for storage
	grants, err := a.grantManager.GetActiveGrantsByType(userID, models.GrantTypeStorage)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get active storage grants: %w", err)
	}

	// Calculate available bytes
	availableBytes := a.grantManager.CalculateAvailableBytes(grants)

	// Check if we have enough allowance
	if requestedBytes > availableBytes {
		return a.createQuotaCheckResult(false, models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, pluginCore.QuotaCheckDetails{
			CurrentUsage: 0, // Current usage isn't tracked in allowance policy the same way
			Allowance:    &availableBytes,
			AllowanceUsed: func() *uint64 {
				used := uint64(0)
				for _, grant := range grants {
					used += grant.BytesUsed
				}
				return &used
			}(),
			Policy: pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance),
		}), nil
	}

	return a.createSuccessResult(models.EnforcementPolicyAllowance), nil
}

// RecordUpload records an upload operation and consumes from grants based on prioritization rules
func (a *AllowancePolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := a.validateUserID(userID); err != nil {
		return err
	}
	if err := a.validateBytes(bytes); err != nil {
		return err
	}

	// Consume bytes from grants based on prioritization
	_, err := a.grantManager.ConsumeFromGrants(userID, models.GrantTypeUpload, bytes)
	if err != nil {
		return fmt.Errorf("failed to consume upload allowance: %w", err)
	}

	// Delegate to UsageManager for actual recording
	return a.usageManager.RecordUpload(userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation and consumes from grants based on prioritization rules
func (a *AllowancePolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := a.validateUserID(userID); err != nil {
		return err
	}
	if err := a.validateBytes(bytes); err != nil {
		return err
	}

	// Consume bytes from grants based on prioritization
	_, err := a.grantManager.ConsumeFromGrants(userID, models.GrantTypeDownload, bytes)
	if err != nil {
		return fmt.Errorf("failed to consume download allowance: %w", err)
	}

	// Delegate to UsageManager for actual recording
	return a.usageManager.RecordDownload(userID, uploadID, bytes, ip)
}

// RecordStorageChange records a storage change operation and consumes from grants based on prioritization rules
func (a *AllowancePolicyEnforcer) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := a.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// For storage changes, we only consume allowance when adding storage
	if bytes > 0 {
		// Consume bytes from grants based on prioritization
		_, err := a.grantManager.ConsumeFromGrants(userID, models.GrantTypeStorage, uint64(bytes))
		if err != nil {
			return fmt.Errorf("failed to consume storage allowance: %w", err)
		}
	}

	// Delegate to UsageManager for actual recording
	return a.usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
}

// GetDetailedUsage delegates to the base enforcer
func (a *AllowancePolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return a.getDetailedUsage(userID, start, end)
}

// GetCurrentUsage delegates to the base enforcer
func (a *AllowancePolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return a.getCurrentUsage(userID)
}

// GetUsageHistory delegates to the base enforcer
func (a *AllowancePolicyEnforcer) GetUsageHistory(userID uint, period int, usageType models.UsageType) ([]*pluginCore.UsagePoint, error) {
	return a.getUsageHistory(userID, period, usageType)
}
