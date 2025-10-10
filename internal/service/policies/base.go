package policies

import (
	"fmt"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BasePolicyEnforcer provides common functionality for all policy enforcers
type BasePolicyEnforcer struct {
	ctx    core.Context
	db     *gorm.DB
	logger *core.Logger
}

// NewBasePolicyEnforcer creates a new base policy enforcer
func NewBasePolicyEnforcer(ctx core.Context) *BasePolicyEnforcer {
	return &BasePolicyEnforcer{
		ctx:    ctx,
		db:     ctx.DB(),
		logger: ctx.NamedLogger("quota.BasePolicyEnforcer"),
	}
}

// Common validation methods

// validateUserID validates that a user ID is valid
func (b *BasePolicyEnforcer) validateUserID(userID uint) error {
	if userID == 0 {
		return models.ErrInvalidUserID
	}
	return nil
}

// validateRequestedBytes validates that requested bytes is valid
// Note: 0 bytes is considered valid for quota checks (e.g., checking if any upload is allowed)
func (b *BasePolicyEnforcer) validateRequestedBytes(requestedBytes uint64) error {
	if requestedBytes == 0 {
		return models.ErrInvalidBytes
	}
	return nil
}

// validateBytes validates that bytes value is valid (not zero)
func (b *BasePolicyEnforcer) validateBytes(bytes uint64) error {
	if bytes == 0 {
		return models.ErrInvalidBytes
	}
	return nil
}

// Common database operations

// getUserQuotaConfig retrieves a user's quota configuration
func (b *BasePolicyEnforcer) getUserQuotaConfig(userID uint) (*models.UserQuotaConfig, error) {
	if err := b.validateUserID(userID); err != nil {
		return nil, err
	}

	var cfg models.UserQuotaConfig
	err := b.db.Where("user_id = ?", userID).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return default configuration if none exists
			return b.createDefaultQuotaConfig(userID)
		}
		return nil, err
	}

	return &cfg, nil
}

// createDefaultQuotaConfig creates a default quota configuration for a user
func (b *BasePolicyEnforcer) createDefaultQuotaConfig(userID uint) (*models.UserQuotaConfig, error) {
	// Get default enforcement policy from config
	quotaConfig := core.GetServiceConfig[*config.QuotaConfig](b.ctx, pluginCore.QUOTA_SERVICE)
	defaultPolicy := models.EnforcementPolicyHardLimits // Default fallback

	if quotaConfig != nil && quotaConfig.DefaultEnforcementPolicy != "" {
		policy := models.EnforcementPolicy(quotaConfig.DefaultEnforcementPolicy)
		// If the created policy is invalid, fall back to a default instead of returning an error
		if policy.IsValid() {
			defaultPolicy = policy
		}
	}

	cfg := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: defaultPolicy,
	}

	err := b.db.Create(cfg).Error
	if err != nil {
		return nil, fmt.Errorf("failed to create default quota config: %w", err)
	}

	return cfg, nil
}

// getCurrentUsage retrieves the current usage for a user
func (b *BasePolicyEnforcer) getCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	if err := b.validateUserID(userID); err != nil {
		return nil, err
	}

	// Get today's aggregated usage
	today := time.Now().Truncate(24 * time.Hour)
	var dailyQuota models.UserQuota
	err := b.db.Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	// If no record found, initialize with zero values
	if err == gorm.ErrRecordNotFound {
		dailyQuota = models.UserQuota{
			UserID:          userID,
			Date:            today,
			BytesUploaded:   0,
			BytesDownloaded: 0,
			BytesStored:     0,
		}
	}

	// Get usage by type for the current period
	usageByType := make(map[models.UsageType]uint64)

	// For simplicity, we'll use the daily quota values
	// In a real implementation, you might want to aggregate over a different period
	usageByType[models.UsageTypeUpload] = dailyQuota.BytesUploaded
	usageByType[models.UsageTypeDownload] = dailyQuota.BytesDownloaded
	usageByType[models.UsageTypeStorageAdd] = dailyQuota.BytesStored

	return &pluginCore.Usage{
		UserID:          userID,
		BytesUploaded:   dailyQuota.BytesUploaded,
		BytesDownloaded: dailyQuota.BytesDownloaded,
		BytesStored:     dailyQuota.BytesStored,
		LastUpdated:     time.Now(),
		UsageByType:     convertUsageTypeMap(usageByType),
	}, nil
}

// getUsageHistory retrieves usage history for a user
func (b *BasePolicyEnforcer) getUsageHistory(userID uint, period int, usageType models.UsageType) ([]*pluginCore.UsagePoint, error) {
	if err := b.validateUserID(userID); err != nil {
		return nil, err
	}

	// Calculate the start date based on the period
	startDate := time.Now().AddDate(0, 0, -period)

	var usageDetails []models.UserUsageDetail
	err := b.db.Where("user_id = ? AND type = ? AND timestamp >= ?", userID, usageType, startDate).
		Order("timestamp ASC").
		Find(&usageDetails).Error
	if err != nil {
		return nil, err
	}

	var usagePoints []*pluginCore.UsagePoint
	for _, detail := range usageDetails {
		usagePoints = append(usagePoints, &pluginCore.UsagePoint{
			Date:   detail.Timestamp,
			Bytes:  detail.Bytes,
			Type:   detail.Type,
			UserID: detail.UserID,
		})
	}

	return usagePoints, nil
}

// getDetailedUsage retrieves detailed usage records for a user within a time range
func (b *BasePolicyEnforcer) getDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	if err := b.validateUserID(userID); err != nil {
		return nil, err
	}

	var usageDetails []*models.UserUsageDetail
	err := b.db.Where("user_id = ? AND timestamp >= ? AND timestamp <= ?", userID, start, end).
		Order("timestamp DESC").
		Find(&usageDetails).Error
	if err != nil {
		return nil, err
	}

	return usageDetails, nil
}

// recordUserUsageDetail records a detailed usage record
func (b *BasePolicyEnforcer) recordUserUsageDetail(detail *models.UserUsageDetail) error {
	return b.db.Create(detail).Error
}

// updateDailyUsage updates the daily aggregated usage for a user
func (b *BasePolicyEnforcer) updateDailyUsage(userID uint, usageType models.UsageType, bytes uint64) error {
	if err := b.validateUserID(userID); err != nil {
		return err
	}

	today := time.Now().Truncate(24 * time.Hour)

	return b.db.Transaction(func(tx *gorm.DB) error {
		// Try to find existing daily quota record with row locking
		var dailyQuota models.UserQuota
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error

		if err == gorm.ErrRecordNotFound {
			// Create new daily quota record
			dailyQuota = models.UserQuota{
				UserID: userID,
				Date:   today,
			}

			// Set the appropriate field based on usage type
			switch usageType {
			case models.UsageTypeUpload:
				dailyQuota.BytesUploaded = bytes
			case models.UsageTypeDownload:
				dailyQuota.BytesDownloaded = bytes
			case models.UsageTypeStorageAdd:
				dailyQuota.BytesStored = bytes
			}

			return tx.Create(&dailyQuota).Error
		} else if err != nil {
			return err
		}

		// Update existing daily quota record
		switch usageType {
		case models.UsageTypeUpload:
			dailyQuota.BytesUploaded += bytes
		case models.UsageTypeDownload:
			dailyQuota.BytesDownloaded += bytes
		case models.UsageTypeStorageAdd:
			dailyQuota.BytesStored += bytes
		}

		return tx.Save(&dailyQuota).Error
	})
}

// Helper functions

// convertUsageTypeMap converts models.UsageType map to core.UsageType map
func convertUsageTypeMap(input map[models.UsageType]uint64) map[pluginCore.UsageType]uint64 {
	return lo.MapEntries(input, func(k models.UsageType, v uint64) (pluginCore.UsageType, uint64) {
		return pluginCore.UsageType(k), v
	})
}

// createQuotaCheckResult creates a standard quota check result
func (b *BasePolicyEnforcer) createQuotaCheckResult(allowed bool, reason pluginCore.QuotaCheckReason, policy models.EnforcementPolicy, details pluginCore.QuotaCheckDetails) pluginCore.QuotaCheckResult {
	details.Policy = pluginCore.EnforcementPolicy(policy)
	return pluginCore.QuotaCheckResult{
		Allowed: allowed,
		Reason:  reason,
		Details: details,
	}
}

// createSuccessResult creates a success quota check result
func (b *BasePolicyEnforcer) createSuccessResult(policy models.EnforcementPolicy) pluginCore.QuotaCheckResult {
	return b.createQuotaCheckResult(true, models.QuotaCheckReasonOK, policy, pluginCore.QuotaCheckDetails{
		Policy: policy,
	})
}

// createLimitExceededResult creates a limit exceeded quota check result
func (b *BasePolicyEnforcer) createLimitExceededResult(policy models.EnforcementPolicy, currentUsage, limit uint64) pluginCore.QuotaCheckResult {
	return b.createQuotaCheckResult(false, models.QuotaCheckReasonLimitExceeded, policy, pluginCore.QuotaCheckDetails{
		CurrentUsage: currentUsage,
		Limit:        &limit,
		Policy:       policy,
	})
}

// createWarningResult creates a warning quota check result (for threshold policy)
func (b *BasePolicyEnforcer) createWarningResult(policy models.EnforcementPolicy, currentUsage, threshold, limit uint64) pluginCore.QuotaCheckResult {
	return b.createQuotaCheckResult(true, models.QuotaCheckReasonWarningThreshold, policy, pluginCore.QuotaCheckDetails{
		CurrentUsage: currentUsage,
		Threshold:    &threshold,
		Limit:        &limit,
		Policy:       policy,
	})
}
