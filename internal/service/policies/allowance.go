package policies

import (
	"errors"
	"fmt"
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
func (a *AllowancePolicyEnforcer) CheckUploadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
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
	grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(config.UserID, models.GrantTypeUpload)
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
}

// CheckDownloadQuota checks if a download operation is allowed under allowance policy
func (a *AllowancePolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
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
	grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(config.UserID, models.GrantTypeDownload)
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
		return a.createQuotaCheckResult(false, models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, details), nil
	}

	return a.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyAllowance, details), nil
}

// CheckStorageQuota checks if a storage operation is allowed under allowance policy
func (a *AllowancePolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
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
	grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(config.UserID, models.GrantTypeStorage)
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
		return a.createQuotaCheckResult(false, models.QuotaCheckReasonAllowanceDepleted, models.EnforcementPolicyAllowance, details), nil
	}

	return a.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyAllowance, details), nil
}

// RecordUpload records an upload operation and consumes allowance
func (a *AllowancePolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := a.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}

	// Get active grants for upload
	grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(userID, models.GrantTypeUpload)
	if err != nil {
		return fmt.Errorf("failed to get active upload grants: %w", err)
	}

	// Calculate available bytes from grants
	availableBytes := a.quotaService.GetGrantManager().CalculateAvailableBytes(grants)

	// Check if we have enough allowance
	if bytes > availableBytes {
		return fmt.Errorf("insufficient upload allowance: requested %d, available %d", bytes, availableBytes)
	}

	// Consume allowance from grants
	_, err = a.quotaService.GetGrantManager().ConsumeFromGrants(userID, models.GrantTypeUpload, bytes)
	if err != nil {
		return fmt.Errorf("failed to consume upload allowance: %w", err)
	}

	return a.delegateRecordUpload(userID, uploadID, bytes, ip)
}

// RecordDownload records a download operation and consumes allowance
func (a *AllowancePolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := a.validateUserID(userID); err != nil {
		return err
	}
	if err := a.validateBytes(bytes); err != nil {
		return err
	}

	// Get active grants for download
	grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(userID, models.GrantTypeDownload)
	if err != nil {
		return fmt.Errorf("failed to get active download grants: %w", err)
	}

	// Calculate available bytes from grants
	availableBytes := a.quotaService.GetGrantManager().CalculateAvailableBytes(grants)

	// Check if we have enough allowance
	if bytes > availableBytes {
		return fmt.Errorf("insufficient download allowance: requested %d, available %d", bytes, availableBytes)
	}

	// Consume allowance from grants
	_, err = a.quotaService.GetGrantManager().ConsumeFromGrants(userID, models.GrantTypeDownload, bytes)
	if err != nil {
		return fmt.Errorf("failed to consume download allowance: %w", err)
	}

	// Delegate to UsageManager for actual recording
	return a.quotaService.GetUsageManager().RecordDownload(userID, uploadID, bytes, ip)
}

// RecordStorageChange records a storage change operation and consumes allowance
func (a *AllowancePolicyEnforcer) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := a.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// For storage changes, we only consume allowance when adding storage (positive bytes)
	if bytes > 0 {
		// Get active grants for storage
		grants, err := a.quotaService.GetGrantManager().GetActiveGrantsByType(userID, models.GrantTypeStorage)
		if err != nil {
			return fmt.Errorf("failed to get active storage grants: %w", err)
		}

		// Calculate available bytes from grants
		availableBytes := a.quotaService.GetGrantManager().CalculateAvailableBytes(grants)

		// Check if we have enough allowance
		if uint64(bytes) > availableBytes {
			return fmt.Errorf("insufficient storage allowance: requested %d, available %d", bytes, availableBytes)
		}

		// Consume allowance from grants
		_, err = a.quotaService.GetGrantManager().ConsumeFromGrants(userID, models.GrantTypeStorage, uint64(bytes))
		if err != nil {
			return fmt.Errorf("failed to consume storage allowance: %w", err)
		}
	}

	// Delegate to UsageManager for actual recording
	return a.quotaService.GetUsageManager().RecordStorageChange(userID, uploadID, bytes, ip)
}

// GetDetailedUsage delegates to the usage manager
func (a *AllowancePolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return a.quotaService.GetUsageManager().GetDetailedUsage(userID, start, end)
}

// GetCurrentUsage delegates to the usage manager
func (a *AllowancePolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return a.quotaService.GetUsageManager().GetCurrentUsage(userID)
}

// GetUsageHistory delegates to the usage manager
func (a *AllowancePolicyEnforcer) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	return a.quotaService.GetUsageManager().GetUsageHistory(userID, period, usageType)
}
