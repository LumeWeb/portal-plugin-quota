package quota

import (
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/service/managers"
	"go.lumeweb.com/portal-plugin-quota/internal/service/policies"
	portalCore "go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type QuotaServiceDefault struct {
	ctx             portalCore.Context
	logger          *portalCore.Logger
	config          *config.QuotaConfig
	usageManager    pluginCore.UsageManager
	grantManager    pluginCore.GrantManager
	configManager   pluginCore.ConfigManager
	planManager     pluginCore.QuotaPlanManager
	limitResolver   pluginCore.LimitResolver
	usageAggregator pluginCore.UsageAggregator
}

var _ pluginCore.QuotaService = (*QuotaServiceDefault)(nil)

func NewQuotaService() (portalCore.Service, []portalCore.ContextBuilderOption, error) {
	service := &QuotaServiceDefault{}

	return service, portalCore.ContextOptions(
		portalCore.ContextWithStartupFunc(func(ctx portalCore.Context) error {
			service.ctx = ctx
			service.logger = ctx.ServiceLogger(service)

			// Load service configuration
			service.config = portalCore.GetServiceConfig[*config.QuotaConfig](ctx, pluginCore.QUOTA_SERVICE)

			// Initialize managers
			service.usageManager = managers.NewUsageManager(ctx)
			service.grantManager = managers.NewGrantManager(ctx)

			// Initialize limit resolver
			service.limitResolver = policies.NewLimitResolver(ctx, service)

			// Initialize plan manager
			service.planManager = policies.NewQuotaPlanManager(ctx.DB(), ctx.Logger())

			// Initialize policy enforcers
			policyEnforcers := make(map[models.EnforcementPolicy]pluginCore.PolicyEnforcer)
			policyEnforcers[models.EnforcementPolicyHardLimits] = policies.NewHardLimitsPolicyEnforcer(ctx, service)
			policyEnforcers[models.EnforcementPolicyThreshold] = policies.NewThresholdPolicyEnforcer(ctx, service)
			policyEnforcers[models.EnforcementPolicyUnlimited] = policies.NewUnlimitedPolicyEnforcer(ctx, service)
			policyEnforcers[models.EnforcementPolicyAllowance] = policies.NewAllowancePolicyEnforcer(ctx, service)

			// Initialize config manager with all required dependencies
			service.configManager = managers.NewConfigManager(ctx, service.limitResolver, service.planManager, policyEnforcers)

			return nil
		}),
	), nil
}

func (s *QuotaServiceDefault) ID() string {
	return pluginCore.QUOTA_SERVICE
}

// Usage Recording
func (s *QuotaServiceDefault) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if s.usageManager == nil {
		return fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.RecordUpload(userID, uploadID, bytes, ip)
}

func (s *QuotaServiceDefault) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if s.usageManager == nil {
		return fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.RecordDownload(userID, uploadID, bytes, ip)
}

func (s *QuotaServiceDefault) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if s.usageManager == nil {
		return fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
}

// Quota Checking
func (s *QuotaServiceDefault) CheckUploadQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}

	config, err := s.configManager.GetUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
	}

	enforcer, err := s.configManager.GetPolicyEnforcer(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
	}

	return enforcer.CheckUploadQuota(config, requestedBytes)
}

func (s *QuotaServiceDefault) CheckDownloadQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}

	config, err := s.configManager.GetUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
	}

	enforcer, err := s.configManager.GetPolicyEnforcer(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
	}

	return enforcer.CheckDownloadQuota(config, requestedBytes)
}

func (s *QuotaServiceDefault) CheckStorageQuota(userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}

	config, err := s.configManager.GetUserQuotaConfig(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
	}

	enforcer, err := s.configManager.GetPolicyEnforcer(userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
	}

	return enforcer.CheckStorageQuota(config, requestedBytes)
}

// Usage Analytics
func (s *QuotaServiceDefault) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetCurrentUsage(userID)
}

func (s *QuotaServiceDefault) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetUsageHistory(userID, period, usageType)
}

func (s *QuotaServiceDefault) GetDetailedUsage(userID uint, start, end time.Time) ([]*pluginCore.UserUsageDetail, error) {
	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetDetailedUsage(userID, start, end)
}

func (s *QuotaServiceDefault) GetTodayUsage(userID uint) (*pluginCore.Usage, error) {
	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetCurrentUsage(userID)
}

// Configuration Management
func (s *QuotaServiceDefault) SetQuotaConfig(userID uint, config *pluginCore.UserQuotaConfig) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	// Ensure the config has the correct user ID
	config.UserID = userID

	// Use upsert to handle both create and update cases
	result := s.ctx.DB().Where("user_id = ?", userID).Assign(config).FirstOrCreate(config)
	if result.Error != nil {
		return fmt.Errorf("failed to set user quota config: %w", result.Error)
	}

	return nil
}

func (s *QuotaServiceDefault) GetQuotaConfig(userID uint) (*pluginCore.UserQuotaConfig, error) {
	if s.configManager == nil {
		return nil, fmt.Errorf("config manager not initialized")
	}

	return s.configManager.GetUserQuotaConfig(userID)
}

// Quota Plan Management
func (s *QuotaServiceDefault) CreateQuotaPlan(plan *models.QuotaPlan) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	db := s.ctx.DB()
	if err := db.Create(plan).Error; err != nil {
		return fmt.Errorf("failed to create quota plan: %w", err)
	}

	return nil
}

func (s *QuotaServiceDefault) UpdateQuotaPlan(planID uint, plan *models.QuotaPlan) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	db := s.ctx.DB()
	if err := db.Model(&models.QuotaPlan{}).Where("id = ?", planID).Updates(plan).Error; err != nil {
		return fmt.Errorf("failed to update quota plan: %w", err)
	}

	return nil
}

func (s *QuotaServiceDefault) DeleteQuotaPlan(planID uint) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	db := s.ctx.DB()
	if err := db.Delete(&models.QuotaPlan{}, planID).Error; err != nil {
		return fmt.Errorf("failed to delete quota plan: %w", err)
	}

	return nil
}

func (s *QuotaServiceDefault) GetQuotaPlan(planID uint) (*models.QuotaPlan, error) {
	if s.ctx == nil || s.ctx.DB() == nil {
		return nil, fmt.Errorf("service context or database not initialized")
	}

	var plan models.QuotaPlan
	db := s.ctx.DB()
	if err := db.Where("id = ?", planID).First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("quota plan not found: %d", planID)
		}
		return nil, fmt.Errorf("failed to get quota plan: %w", err)
	}

	return &plan, nil
}

func (s *QuotaServiceDefault) ListQuotaPlans() ([]*models.QuotaPlan, error) {
	if s.ctx == nil || s.ctx.DB() == nil {
		return nil, fmt.Errorf("service context or database not initialized")
	}

	var plans []*models.QuotaPlan
	db := s.ctx.DB()
	if err := db.Find(&plans).Error; err != nil {
		return nil, fmt.Errorf("failed to list quota plans: %w", err)
	}

	return plans, nil
}

func (s *QuotaServiceDefault) SetDefaultQuotaPlan(planID uint) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	db := s.ctx.DB()

	// Perform both updates atomically in a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// First, unset any existing default plan
		if err := tx.Model(&models.QuotaPlan{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return fmt.Errorf("failed to unset existing default plan: %w", err)
		}

		// Then set the new default plan
		if err := tx.Model(&models.QuotaPlan{}).Where("id = ?", planID).Update("is_default", true).Error; err != nil {
			return fmt.Errorf("failed to set default quota plan: %w", err)
		}

		return nil
	})
}

func (s *QuotaServiceDefault) GetDefaultQuotaPlan() (*models.QuotaPlan, error) {
	if s.ctx == nil || s.ctx.DB() == nil {
		return nil, fmt.Errorf("service context or database not initialized")
	}

	var plan models.QuotaPlan
	db := s.ctx.DB()
	if err := db.Where("is_default = ?", true).First(&plan).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no default quota plan found")
		}
		return nil, fmt.Errorf("failed to get default quota plan: %w", err)
	}

	return &plan, nil
}

func (s *QuotaServiceDefault) AssignUserToPlan(userID uint, planID uint) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	if userID == 0 {
		return fmt.Errorf("invalid user ID")
	}

	// Verify that the plan exists
	_, err := s.planManager.GetQuotaPlanByID(uint64(planID))
	if err != nil {
		return fmt.Errorf("failed to verify quota plan existence: %w", err)
	}

	db := s.ctx.DB()

	// Perform both operations atomically in a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// First, ensure the user has a quota config
		result := tx.Where("user_id = ?", userID).FirstOrCreate(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		})
		if result.Error != nil {
			return fmt.Errorf("failed to get or create user quota config: %w", result.Error)
		}

		// Update the user's quota config with the plan ID
		if err := tx.Model(&models.UserQuotaConfig{}).Where("user_id = ?", userID).UpdateColumn("quota_plan_id", planID).Error; err != nil {
			return fmt.Errorf("failed to assign user to plan: %w", err)
		}

		return nil
	})
}

// RemoveUserFromPlan removes a user from their assigned quota plan
func (s *QuotaServiceDefault) RemoveUserFromPlan(userID uint) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	if userID == 0 {
		return fmt.Errorf("invalid user ID")
	}

	db := s.ctx.DB()

	// Perform the operation in a transaction
	return db.Transaction(func(tx *gorm.DB) error {
		// First, ensure the user has a quota config
		result := tx.Where("user_id = ?", userID).FirstOrCreate(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		})
		if result.Error != nil {
			return fmt.Errorf("failed to get or create user quota config: %w", result.Error)
		}

		// Update the user's quota config to remove the plan ID
		if err := tx.Model(&models.UserQuotaConfig{}).Where("user_id = ?", userID).UpdateColumn("quota_plan_id", nil).Error; err != nil {
			return fmt.Errorf("failed to remove user from plan: %w", err)
		}

		return nil
	})
}

// Allowance Management
func (s *QuotaServiceDefault) AddAllowance(userID uint, storage, upload, download uint64) error {
	// This method is kept for backward compatibility but delegates to the bonus allowance method
	return s.AddBonusAllowance(userID, storage, upload, download)
}

// AddBonusAllowance adds bonus allowance grants for a user
func (s *QuotaServiceDefault) AddBonusAllowance(userID uint, storage, upload, download uint64) error {
	return s.addAllowanceWithSource(userID, storage, upload, download, models.GrantSourceBonus)
}

// AddPromoAllowance adds promotional allowance grants for a user
func (s *QuotaServiceDefault) AddPromoAllowance(userID uint, storage, upload, download uint64) error {
	return s.addAllowanceWithSource(userID, storage, upload, download, models.GrantSourcePromo)
}

// AddSubscriptionAllowance adds subscription-based allowance grants for a user
func (s *QuotaServiceDefault) AddSubscriptionAllowance(userID uint, storage, upload, download uint64) error {
	return s.addAllowanceWithSource(userID, storage, upload, download, models.GrantSourceSubscription)
}

// AddPAYGAddonAllowance adds pay-as-you-go addon allowance grants for a user
func (s *QuotaServiceDefault) AddPAYGAddonAllowance(userID uint, storage, upload, download uint64) error {
	return s.addAllowanceWithSource(userID, storage, upload, download, models.GrantSourcePAYGAddon)
}

// addAllowanceWithSource is a private helper method that creates allowance grants with a specific source
func (s *QuotaServiceDefault) addAllowanceWithSource(userID uint, storage, upload, download uint64, source models.GrantSource) error {
	if s.grantManager == nil {
		return fmt.Errorf("grant manager not initialized")
	}

	if !source.IsValid() {
		return fmt.Errorf("invalid grant source: %s", source)
	}

	// Perform all grant creations atomically in a transaction
	return s.ctx.DB().Transaction(func(tx *gorm.DB) error {
		// Create storage allowance grant
		if storage > 0 {
			storageGrant := &models.AllowanceGrant{
				UserID: userID,
				Type:   models.GrantTypeStorage,
				Source: source,
				Bytes:  storage,
			}
			if err := s.grantManager.CreateAllowanceGrantLocked(userID, storageGrant, tx); err != nil {
				return fmt.Errorf("failed to create storage allowance grant: %w", err)
			}
		}

		// Create upload allowance grant
		if upload > 0 {
			uploadGrant := &models.AllowanceGrant{
				UserID: userID,
				Type:   models.GrantTypeUpload,
				Source: source,
				Bytes:  upload,
			}
			if err := s.grantManager.CreateAllowanceGrantLocked(userID, uploadGrant, tx); err != nil {
				return fmt.Errorf("failed to create upload allowance grant: %w", err)
			}
		}

		// Create download allowance grant
		if download > 0 {
			downloadGrant := &models.AllowanceGrant{
				UserID: userID,
				Type:   models.GrantTypeDownload,
				Source: source,
				Bytes:  download,
			}
			if err := s.grantManager.CreateAllowanceGrantLocked(userID, downloadGrant, tx); err != nil {
				return fmt.Errorf("failed to create download allowance grant: %w", err)
			}
		}

		return nil
	})
}

func (s *QuotaServiceDefault) DeductAllowance(userID uint, storage, upload, download uint64) error {
	if s.grantManager == nil {
		return fmt.Errorf("grant manager not initialized")
	}

	// Handle all deductions in a single transaction for consistency
	return s.ctx.DB().Transaction(func(tx *gorm.DB) error {
		// Deduct storage allowance
		if storage > 0 {
			detail := &models.UserUsageDetail{
				UserID:    userID,
				Type:      models.UsageTypeStorageRemove,
				Bytes:     storage,
				Timestamp: time.Now().UTC(),
			}
			if err := tx.Create(detail).Error; err != nil {
				return fmt.Errorf("failed to create storage usage detail: %w", err)
			}

			_, err := s.grantManager.ConsumeFromGrants(userID, models.GrantTypeStorage, storage, detail.ID, tx)
			if err != nil {
				return fmt.Errorf("failed to deduct storage allowance: %w", err)
			}
		}

		// Deduct upload allowance
		if upload > 0 {
			detail := &models.UserUsageDetail{
				UserID:    userID,
				Type:      models.UsageTypeUpload,
				Bytes:     upload,
				Timestamp: time.Now().UTC(),
			}
			if err := tx.Create(detail).Error; err != nil {
				return fmt.Errorf("failed to create upload usage detail: %w", err)
			}

			_, err := s.grantManager.ConsumeFromGrants(userID, models.GrantTypeUpload, upload, detail.ID, tx)
			if err != nil {
				return fmt.Errorf("failed to deduct upload allowance: %w", err)
			}
		}

		// Deduct download allowance
		if download > 0 {
			detail := &models.UserUsageDetail{
				UserID:    userID,
				Type:      models.UsageTypeDownload,
				Bytes:     download,
				Timestamp: time.Now().UTC(),
			}
			if err := tx.Create(detail).Error; err != nil {
				return fmt.Errorf("failed to create download usage detail: %w", err)
			}

			_, err := s.grantManager.ConsumeFromGrants(userID, models.GrantTypeDownload, download, detail.ID, tx)
			if err != nil {
				return fmt.Errorf("failed to deduct download allowance: %w", err)
			}
		}

		return nil
	})
}

func (s *QuotaServiceDefault) GetAllowanceBalance(userID uint) (*pluginCore.AllowanceBalance, error) {
	if s.configManager == nil {
		return nil, fmt.Errorf("config manager not initialized")
	}

	grants, err := s.configManager.GetUserAllowanceGrants(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user allowance grants: %w", err)
	}

	// Calculate balances by type
	balance := &pluginCore.AllowanceBalance{}

	for _, grant := range grants {
		switch grant.Type {
		case models.GrantTypeStorage:
			balance.StorageAllowance += grant.Bytes
			balance.StorageUsed += grant.BytesUsed
			balance.StorageRemaining += grant.BytesRemaining
		case models.GrantTypeUpload:
			balance.UploadAllowance += grant.Bytes
			balance.UploadUsed += grant.BytesUsed
			balance.UploadRemaining += grant.BytesRemaining
		case models.GrantTypeDownload:
			balance.DownloadAllowance += grant.Bytes
			balance.DownloadUsed += grant.BytesUsed
			balance.DownloadRemaining += grant.BytesRemaining
		}
	}

	return balance, nil
}

func (s *QuotaServiceDefault) ResetAllowance(userID uint) error {
	if s.grantManager == nil {
		return fmt.Errorf("grant manager not initialized")
	}

	// Get all active grants for the user
	grants, err := s.grantManager.GetActiveGrants(userID)
	if err != nil {
		return fmt.Errorf("failed to get user grants: %w", err)
	}

	// Deactivate all grants
	for _, grant := range grants {
		if err := s.grantManager.DeactivateGrant(grant.ID); err != nil {
			s.logger.Warn("Failed to deactivate grant", zap.Uint("grantID", grant.ID), zap.Error(err))
			// Continue with other grants even if one fails
		}
	}

	return nil
}

// System Management
// TODO: Implement reconciliation logic
func (s *QuotaServiceDefault) Reconcile() error {
	// Implementation would depend on specific reconciliation needs
	// This is a placeholder for now
	s.logger.Info("Reconciliation started")

	// In a real implementation, this might:
	// 1. Check for inconsistencies between usage details and daily quotas
	// 2. Update any grants that need expiration handling
	// 3. Clean up any orphaned records

	return nil
}

func (s *QuotaServiceDefault) CleanupOldRecords(retentionDays int) error {
	if s.ctx == nil || s.ctx.DB() == nil {
		return fmt.Errorf("service context or database not initialized")
	}

	db := s.ctx.DB()
	cutoffDate := time.Now().UTC().AddDate(0, 0, -retentionDays)

	// Delete old usage details
	if err := db.Where("timestamp < ?", cutoffDate).Delete(&models.UserUsageDetail{}).Error; err != nil {
		return fmt.Errorf("failed to cleanup old usage details: %w", err)
	}

	// Delete old allowance consumptions
	if err := db.Where("consumption_date < ?", cutoffDate).Delete(&models.AllowanceConsumption{}).Error; err != nil {
		return fmt.Errorf("failed to cleanup old allowance consumptions: %w", err)
	}

	s.logger.Info("Old records cleanup completed", zap.Int("retentionDays", retentionDays))
	return nil
}

// Manager Getters
func (s *QuotaServiceDefault) GetUsageManager() pluginCore.UsageManager {
	return s.usageManager
}

func (s *QuotaServiceDefault) GetGrantManager() pluginCore.GrantManager {
	return s.grantManager
}

func (s *QuotaServiceDefault) GetUsageAggregator() pluginCore.UsageAggregator {
	return s.usageAggregator
}

func (s *QuotaServiceDefault) GetQuotaPlanManager() pluginCore.QuotaPlanManager {
	return s.planManager
}

func (s *QuotaServiceDefault) GetConfigManager() pluginCore.ConfigManager {
	return s.configManager
}

// Config method for service registration
func (s *QuotaServiceDefault) Config() (any, error) {
	return &config.QuotaConfig{}, nil
}
