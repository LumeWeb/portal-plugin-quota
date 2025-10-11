package policies

import (
	"fmt"
	"time"

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

	// Aggregate usage across all records for this user
	var totalUploaded, totalDownloaded, totalStored uint64

	// Get total bytes uploaded
	err := b.db.Model(&models.UserQuota{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(bytes_uploaded), 0)").
		Scan(&totalUploaded).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get total uploaded bytes: %w", err)
	}

	// Get total bytes downloaded
	err = b.db.Model(&models.UserQuota{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(bytes_downloaded), 0)").
		Scan(&totalDownloaded).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get total downloaded bytes: %w", err)
	}

	// Get total bytes stored
	err = b.db.Model(&models.UserQuota{}).
		Where("user_id = ?", userID).
		Select("COALESCE(SUM(bytes_stored), 0)").
		Scan(&totalStored).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get total stored bytes: %w", err)
	}

	// Get usage by type across all time
	usageByType, err := b.getUsageByType(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage by type: %w", err)
	}

	// Get the last updated timestamp
	var lastUpdated time.Time
	err = b.db.Model(&models.UserQuota{}).
		Where("user_id = ?", userID).
		Order("date DESC").
		Limit(1).
		Pluck("date", &lastUpdated).
		Error
	if err != nil {
		// If no records exist, use current date
		lastUpdated = time.Now().Truncate(24 * time.Hour)
	}

	return &pluginCore.Usage{
		UserID:          userID,
		BytesUploaded:   totalUploaded,
		BytesDownloaded: totalDownloaded,
		BytesStored:     totalStored,
		LastUpdated:     lastUpdated,
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
func (b *BasePolicyEnforcer) updateDailyUsage(userID uint, usageType models.UsageType, bytes int64) error {
	if err := b.validateUserID(userID); err != nil {
		return err
	}

	today := time.Now().Truncate(24 * time.Hour)

	// Create the daily quota record with initial values
	dailyQuota := models.UserQuota{
		UserID: userID,
		Date:   today,
	}

	// Set the appropriate field based on usage type
	switch usageType {
	case models.UsageTypeUpload:
		if bytes > 0 {
			dailyQuota.BytesUploaded = uint64(bytes)
		}
	case models.UsageTypeDownload:
		if bytes > 0 {
			dailyQuota.BytesDownloaded = uint64(bytes)
		}
	case models.UsageTypeStorageAdd:
		if bytes > 0 {
			dailyQuota.BytesStored = uint64(bytes)
		}
	case models.UsageTypeStorageRemove:
		// For storage removal, we start with 0 and apply negative delta
		if bytes < 0 {
			dailyQuota.BytesStored = 0
		}
	}

	// Use upsert to handle concurrent access atomically
	return b.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "date"}},
		DoUpdates: clause.Assignments(b.getUpdateAssignments(usageType, bytes)),
	}).Create(&dailyQuota).Error
}

// getUpdateAssignments returns the assignments for updating quota values atomically
func (b *BasePolicyEnforcer) getUpdateAssignments(usageType models.UsageType, bytes int64) map[string]interface{} {
	assignments := make(map[string]interface{})

	switch usageType {
	case models.UsageTypeUpload:
		if bytes > 0 {
			assignments["bytes_uploaded"] = gorm.Expr("bytes_uploaded + ?", bytes)
		}
	case models.UsageTypeDownload:
		if bytes > 0 {
			assignments["bytes_downloaded"] = gorm.Expr("bytes_downloaded + ?", bytes)
		}
	case models.UsageTypeStorageAdd:
		if bytes > 0 {
			assignments["bytes_stored"] = gorm.Expr("bytes_stored + ?", bytes)
		}
	case models.UsageTypeStorageRemove:
		if bytes < 0 {
			// Apply signed delta and clamp to 0 minimum
			assignments["bytes_stored"] = gorm.Expr("CASE WHEN bytes_stored + ? < 0 THEN 0 ELSE bytes_stored + ? END", bytes, bytes)
		}
	}

	return assignments
}

// Helper functions

// convertUsageTypeMap converts models.UsageType map to core.UsageType map
func convertUsageTypeMap(input map[models.UsageType]uint64) map[pluginCore.UsageType]uint64 {
	output := make(map[pluginCore.UsageType]uint64)
	for k, v := range input {
		output[pluginCore.UsageType(k)] = v
	}
	return output
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
func (b *BasePolicyEnforcer) createLimitExceededResult(policy models.EnforcementPolicy, currentUsage uint64, limit uint64) pluginCore.QuotaCheckResult {
	return b.createQuotaCheckResult(false, models.QuotaCheckReasonLimitExceeded, policy, pluginCore.QuotaCheckDetails{
		CurrentUsage: currentUsage,
		Limit:        &limit,
		Policy:       policy,
	})
}

// getUsageByType retrieves usage by type for a user across all time
func (b *BasePolicyEnforcer) getUsageByType(userID uint) (map[models.UsageType]uint64, error) {
	var usageDetails []models.UserUsageDetail
	err := b.db.Where("user_id = ?", userID).Find(&usageDetails).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get usage details: %w", err)
	}

	usageByType := make(map[models.UsageType]uint64)
	for _, detail := range usageDetails {
		usageByType[detail.Type] += detail.Bytes
	}

	return usageByType, nil
}

// createWarningResult creates a warning quota check result (for threshold policy)
func (b *BasePolicyEnforcer) createWarningResult(policy models.EnforcementPolicy, currentUsage uint64, threshold, limit uint64) pluginCore.QuotaCheckResult {
	return b.createQuotaCheckResult(true, models.QuotaCheckReasonWarningThreshold, policy, pluginCore.QuotaCheckDetails{
		CurrentUsage: currentUsage,
		Threshold:    &threshold,
		Limit:        &limit,
		Policy:       policy,
	})
}
