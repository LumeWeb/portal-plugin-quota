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
}

// NewAllowancePolicyEnforcer creates a new allowance policy enforcer
func NewAllowancePolicyEnforcer(ctx core.Context, grantManager pluginCore.GrantManager) *AllowancePolicyEnforcer {
	base := NewBasePolicyEnforcer(ctx)
	return &AllowancePolicyEnforcer{
		BasePolicyEnforcer: base,
		grantManager:       grantManager,
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

	// Record usage detail
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeUpload,
		Bytes:      bytes,
		IP:         ip,
		SharedWith: 1, // Count the owner for cardinality semantics
		Timestamp:  time.Now(),
	}

	if err := a.recordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record upload usage detail: %w", err)
	}

	// Update daily usage
	if err := a.updateDailyUsage(userID, models.UsageTypeUpload, int64(bytes)); err != nil {
		return fmt.Errorf("failed to update daily upload usage: %w", err)
	}

	return nil
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

	// Record usage detail
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeDownload,
		Bytes:      bytes,
		IP:         ip,
		SharedWith: 1, // Count the owner for cardinality semantics
		Timestamp:  time.Now(),
	}

	if err := a.recordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record download usage detail: %w", err)
	}

	// Update daily usage
	if err := a.updateDailyUsage(userID, models.UsageTypeDownload, int64(bytes)); err != nil {
		return fmt.Errorf("failed to update daily download usage: %w", err)
	}

	return nil
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

	// Determine usage type and byte value for recording
	var usageType models.UsageType
	var recordBytes uint64
	if bytes < 0 {
		usageType = models.UsageTypeStorageRemove
		recordBytes = uint64(-bytes)
	} else {
		usageType = models.UsageTypeStorageAdd
		recordBytes = uint64(bytes)
	}

	// Record usage detail
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       usageType,
		Bytes:      recordBytes,
		IP:         ip,
		SharedWith: 1, // Count the owner for cardinality semantics
		Timestamp:  time.Now(),
	}

	if err := a.recordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record storage usage detail: %w", err)
	}

	// Update daily usage with the correct usage type and byte value
	if err := a.updateDailyUsage(userID, usageType, bytes); err != nil {
		return fmt.Errorf("failed to update daily storage usage: %w", err)
	}

	return nil
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
