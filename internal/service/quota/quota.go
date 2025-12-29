package quota

import (
	"context"
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/service/managers"
	"go.lumeweb.com/portal-plugin-quota/internal/service/policies"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type QuotaServiceDefault struct {
	*core.BaseComponent
	config          *config.QuotaConfig
	usageManager    pluginCore.UsageManager
	grantManager    pluginCore.GrantManager
	configManager   pluginCore.ConfigManager
	planManager     pluginCore.QuotaPlanManager
	limitResolver   pluginCore.LimitResolver
	usageAggregator pluginCore.UsageAggregator
}

var _ pluginCore.QuotaService = (*QuotaServiceDefault)(nil)

func NewQuotaService() (core.Service, []core.ContextBuilderOption, error) {
	service := &QuotaServiceDefault{}

	return service, core.ContextOptions(
		core.ContextWithStartupFunc(func(ctx core.Context) error {
			// Load service configuration
			service.config = core.GetServiceConfig[*config.QuotaConfig](ctx, pluginCore.QUOTA_SERVICE)

			// Initialize managers
			service.usageManager = managers.NewUsageManager(ctx)
			service.grantManager = managers.NewGrantManager(ctx)

			// Initialize limit resolver
			service.limitResolver = policies.NewLimitResolver(ctx, service)

			// Initialize plan manager
			service.planManager = policies.NewQuotaPlanManager(ctx, ctx.DB(), ctx.Logger())

			// Initialize policy enforcers
			policyEnforcers := make(map[models.EnforcementPolicy]pluginCore.PolicyEnforcer)
			policyEnforcers[models.EnforcementPolicyHardLimits] = policies.NewHardLimitsPolicyEnforcer(ctx, service)
			policyEnforcers[models.EnforcementPolicyThreshold] = policies.NewThresholdPolicyEnforcer(ctx, service)
			policyEnforcers[models.EnforcementPolicyUnlimited] = policies.NewUnlimitedPolicyEnforcer(ctx, service)
			policyEnforcers[models.EnforcementPolicyAllowance] = policies.NewAllowancePolicyEnforcer(ctx, service)

			// Initialize config manager with all required dependencies
			service.configManager = managers.NewConfigManager(ctx, service.limitResolver, service.planManager, policyEnforcers)

			// Register event listeners
			service.registerEventListeners()

			return nil
		}),
	), nil
}

func (s *QuotaServiceDefault) ID() string {
	return pluginCore.QUOTA_SERVICE
}

// Usage Recording
func (s *QuotaServiceDefault) RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.RecordUpload")
	defer span.End()

	if s.usageManager == nil {
		return fmt.Errorf("usage manager not initialized")
	}

	err := core.MetricTrack(
		OperationDuration.WithLabelValues(LabelOperationRecord),
		UploadRecorded.WithLabelValues(LabelStatusError),
		func() error {
			return s.usageManager.RecordUpload(ctx, userID, uploadID, bytes, ip)
		},
	)
	if err == nil {
		UploadRecorded.WithLabelValues(LabelStatusAllowed).Inc()
	}
	return err
}

func (s *QuotaServiceDefault) RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.RecordDownload")
	defer span.End()

	if s.usageManager == nil {
		return fmt.Errorf("usage manager not initialized")
	}

	err := core.MetricTrack(
		OperationDuration.WithLabelValues(LabelOperationRecord),
		DownloadRecorded.WithLabelValues(LabelStatusError),
		func() error {
			return s.usageManager.RecordDownload(ctx, userID, uploadID, bytes, ip)
		},
	)
	if err == nil {
		DownloadRecorded.WithLabelValues(LabelStatusAllowed).Inc()
	}
	return err
}

func (s *QuotaServiceDefault) RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.RecordStorageChange")
	defer span.End()

	if s.usageManager == nil {
		return fmt.Errorf("usage manager not initialized")
	}

	err := core.MetricTrack(
		OperationDuration.WithLabelValues(LabelOperationRecord),
		StorageRecorded.WithLabelValues(LabelStatusError),
		func() error {
			return s.usageManager.RecordStorageChange(ctx, userID, uploadID, bytes, ip)
		},
	)
	if err == nil {
		StorageRecorded.WithLabelValues(LabelStatusAllowed).Inc()
	}
	return err
}

// Quota Checking
func (s *QuotaServiceDefault) CheckUploadQuota(ctx context.Context, userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckUploadQuota")
	defer span.End()

	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}

	return core.MetricTrackResult(
		OperationDuration.WithLabelValues(LabelOperationCheck),
		UploadChecked.WithLabelValues(LabelStatusError),
		func() (pluginCore.QuotaCheckResult, error) {
			config, err := s.configManager.GetUserQuotaConfig(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
			}

			enforcer, err := s.configManager.GetPolicyEnforcer(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, requestedBytes)
			if err != nil {
				return result, err
			}

			if !result.Allowed {
				UploadChecked.WithLabelValues(LabelStatusDenied).Inc()
			} else {
				UploadChecked.WithLabelValues(LabelStatusAllowed).Inc()
			}

			return result, nil
		},
	)
}

func (s *QuotaServiceDefault) CheckDownloadQuota(ctx context.Context, userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckDownloadQuota")
	defer span.End()

	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}

	return core.MetricTrackResult(
		OperationDuration.WithLabelValues(LabelOperationCheck),
		DownloadChecked.WithLabelValues(LabelStatusError),
		func() (pluginCore.QuotaCheckResult, error) {
			config, err := s.configManager.GetUserQuotaConfig(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
			}

			enforcer, err := s.configManager.GetPolicyEnforcer(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
			}

			result, err := enforcer.CheckDownloadQuota(ctx, config, requestedBytes)
			if err != nil {
				return result, err
			}

			if !result.Allowed {
				DownloadChecked.WithLabelValues(LabelStatusDenied).Inc()
			} else {
				DownloadChecked.WithLabelValues(LabelStatusAllowed).Inc()
			}

			return result, nil
		},
	)
}

func (s *QuotaServiceDefault) CheckStorageQuota(ctx context.Context, userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckStorageQuota")
	defer span.End()

	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}

	return core.MetricTrackResult(
		OperationDuration.WithLabelValues(LabelOperationCheck),
		StorageChecked.WithLabelValues(LabelStatusError),
		func() (pluginCore.QuotaCheckResult, error) {
			config, err := s.configManager.GetUserQuotaConfig(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
			}

			enforcer, err := s.configManager.GetPolicyEnforcer(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
			}

			result, err := enforcer.CheckStorageQuota(ctx, config, requestedBytes)
			if err != nil {
				return result, err
			}

			if !result.Allowed {
				StorageChecked.WithLabelValues(LabelStatusDenied).Inc()
			} else {
				StorageChecked.WithLabelValues(LabelStatusAllowed).Inc()
			}

			return result, nil
		},
	)
}

// Usage Analytics
func (s *QuotaServiceDefault) GetCurrentUsage(ctx context.Context, userID uint) (*pluginCore.Usage, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetCurrentUsage")
	defer span.End()

	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetCurrentUsage(ctx, userID)
}

func (s *QuotaServiceDefault) GetUsageHistory(ctx context.Context, userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetUsageHistory")
	defer span.End()

	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetUsageHistory(ctx, userID, period, usageType)
}

func (s *QuotaServiceDefault) GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*pluginCore.UserUsageDetail, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetDetailedUsage")
	defer span.End()

	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetDetailedUsage(ctx, userID, start, end)
}

func (s *QuotaServiceDefault) GetTodayUsage(ctx context.Context, userID uint) (*pluginCore.Usage, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetTodayUsage")
	defer span.End()

	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetCurrentUsage(ctx, userID)
}

// Configuration Management
func (s *QuotaServiceDefault) SetQuotaConfig(ctx context.Context, userID uint, config *pluginCore.UserQuotaConfig) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.SetQuotaConfig")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	// Ensure the config has the correct user ID
	config.UserID = userID

	// Use upsert to handle both create and update cases
	err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("user_id = ?", userID).Assign(config).FirstOrCreate(config)
	})
	if err != nil {
		return fmt.Errorf("failed to set user quota config: %w", err)
	}

	return nil
}

func (s *QuotaServiceDefault) GetQuotaConfig(ctx context.Context, userID uint) (*pluginCore.UserQuotaConfig, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetQuotaConfig")
	defer span.End()

	if s.configManager == nil {
		return nil, fmt.Errorf("config manager not initialized")
	}

	return s.configManager.GetUserQuotaConfig(ctx, userID)
}

// Quota Plan Management
func (s *QuotaServiceDefault) CreateQuotaPlan(ctx context.Context, plan *models.QuotaPlan) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CreateQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	err := core.MetricTrack(
		nil,
		policies.PlanOperationsErr.WithLabelValues(policies.LabelPlanOperationCreate),
		func() error {
			if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
				return tx.Create(plan)
			}); err != nil {
				return fmt.Errorf("failed to create quota plan: %w", err)
			}
			policies.PlanOperations.WithLabelValues(policies.LabelPlanOperationCreate).Inc()
			return nil
		},
	)
	return err
}

func (s *QuotaServiceDefault) UpdateQuotaPlan(ctx context.Context, planID uint, plan *models.QuotaPlan) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.UpdateQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	err := core.MetricTrack(
		nil,
		policies.PlanOperationsErr.WithLabelValues(policies.LabelPlanOperationUpdate),
		func() error {
			if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
				return tx.Model(&models.QuotaPlan{}).Where("id = ?", planID).Updates(plan)
			}); err != nil {
				return fmt.Errorf("failed to update quota plan: %w", err)
			}
			policies.PlanOperations.WithLabelValues(policies.LabelPlanOperationUpdate).Inc()
			return nil
		},
	)
	return err
}

func (s *QuotaServiceDefault) DeleteQuotaPlan(ctx context.Context, planID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.DeleteQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	err := core.MetricTrack(
		nil,
		policies.PlanOperationsErr.WithLabelValues(policies.LabelPlanOperationDelete),
		func() error {
			if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
				return tx.Delete(&models.QuotaPlan{}, planID)
			}); err != nil {
				return fmt.Errorf("failed to delete quota plan: %w", err)
			}
			policies.PlanOperations.WithLabelValues(policies.LabelPlanOperationDelete).Inc()
			return nil
		},
	)
	return err
}

func (s *QuotaServiceDefault) GetQuotaPlan(ctx context.Context, planID uint) (*models.QuotaPlan, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var plan models.QuotaPlan
	result, err := core.MetricTrackResult(
		nil,
		policies.PlanOperationsErr.WithLabelValues(policies.LabelPlanOperationGet),
		func() (*models.QuotaPlan, error) {
			if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
				return tx.Where("id = ?", planID).First(&plan)
			}); err != nil {
				if err == gorm.ErrRecordNotFound {
					return nil, fmt.Errorf("quota plan not found: %d", planID)
				}
				return nil, fmt.Errorf("failed to get quota plan: %w", err)
			}
			policies.PlanOperations.WithLabelValues(policies.LabelPlanOperationGet).Inc()
			return &plan, nil
		},
	)
	return result, err
}

func (s *QuotaServiceDefault) ListQuotaPlans(ctx context.Context) ([]*models.QuotaPlan, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.ListQuotaPlans")
	defer span.End()

	if s.DB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var plans []*models.QuotaPlan
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Find(&plans)
	}); err != nil {
		return nil, fmt.Errorf("failed to list quota plans: %w", err)
	}

	return plans, nil
}

func (s *QuotaServiceDefault) SetDefaultQuotaPlan(ctx context.Context, planID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.SetDefaultQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	// Perform both updates atomically in a transaction
	return db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		// First, unset any existing default plan
		if err := tx.Model(&models.QuotaPlan{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return tx
		}

		// Then set the new default plan
		if err := tx.Model(&models.QuotaPlan{}).Where("id = ?", planID).Update("is_default", true).Error; err != nil {
			return tx
		}

		return nil
	})

}

func (s *QuotaServiceDefault) GetDefaultQuotaPlan(ctx context.Context) (*models.QuotaPlan, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetDefaultQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var plan models.QuotaPlan
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("is_default = ?", true).First(&plan)
	}); err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("no default quota plan found")
		}
		return nil, fmt.Errorf("failed to get default quota plan: %w", err)
	}

	return &plan, nil
}

func (s *QuotaServiceDefault) AssignUserToPlan(ctx context.Context, userID uint, planID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.AssignUserToPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	if userID == 0 {
		return fmt.Errorf("invalid user ID")
	}

	// Verify that the plan exists
	_, err := s.planManager.GetQuotaPlanByID(ctx, uint64(planID))
	if err != nil {
		return fmt.Errorf("failed to verify quota plan existence: %w", err)
	}

	// Perform both operations atomically in a transaction
	return db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		// First, ensure the user has a quota config
		result := tx.Where("user_id = ?", userID).FirstOrCreate(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		})
		if result.Error != nil {
			return tx
		}

		// Update the user's quota config with the plan ID
		if err := tx.Model(&models.UserQuotaConfig{}).Where("user_id = ?", userID).UpdateColumn("quota_plan_id", planID).Error; err != nil {
			return tx
		}

		return nil
	})

}

// RemoveUserFromPlan removes a user from their assigned quota plan
func (s *QuotaServiceDefault) RemoveUserFromPlan(ctx context.Context, userID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.RemoveUserFromPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	if userID == 0 {
		return fmt.Errorf("invalid user ID")
	}

	// Update the quota config to remove the plan ID (no-op if user doesn't exist)
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&models.UserQuotaConfig{}).Where("user_id = ?", userID).UpdateColumn("quota_plan_id", nil)
	}); err != nil {
		return fmt.Errorf("failed to remove user from plan: %w", err)
	}

	return nil
}

// Allowance Management
func (s *QuotaServiceDefault) AddAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.AddAllowance")
	defer span.End()

	// This method is kept for backward compatibility but delegates to the bonus allowance method
	return s.AddBonusAllowance(ctx, userID, storage, upload, download)
}

// AddBonusAllowance adds bonus allowance grants for a user
func (s *QuotaServiceDefault) AddBonusAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.AddBonusAllowance")
	defer span.End()

	return s.addAllowanceWithSource(ctx, userID, storage, upload, download, models.GrantSourceBonus)
}

// AddPromoAllowance adds promotional allowance grants for a user
func (s *QuotaServiceDefault) AddPromoAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.AddPromoAllowance")
	defer span.End()

	return s.addAllowanceWithSource(ctx, userID, storage, upload, download, models.GrantSourcePromo)
}

// AddSubscriptionAllowance adds subscription-based allowance grants for a user
func (s *QuotaServiceDefault) AddSubscriptionAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.AddSubscriptionAllowance")
	defer span.End()

	return s.addAllowanceWithSource(ctx, userID, storage, upload, download, models.GrantSourceSubscription)
}

// AddPAYGAddonAllowance adds pay-as-you-go addon allowance grants for a user
func (s *QuotaServiceDefault) AddPAYGAddonAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.AddPAYGAddonAllowance")
	defer span.End()

	return s.addAllowanceWithSource(ctx, userID, storage, upload, download, models.GrantSourcePAYGAddon)
}

// addAllowanceWithSource is a private helper method that creates allowance grants with a specific source
func (s *QuotaServiceDefault) addAllowanceWithSource(ctx context.Context, userID uint, storage, upload, download uint64, source models.GrantSource) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.addAllowanceWithSource")
	defer span.End()

	if s.grantManager == nil {
		return fmt.Errorf("grant manager not initialized")
	}

	if !source.IsValid() {
		return fmt.Errorf("invalid grant source: %s", source)
	}

	sourceLabel := string(source)

	// Perform all grant creations atomically in a transaction
	return db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		// Create storage allowance grant
		if storage > 0 {
			storageGrant := &models.AllowanceGrant{
				UserID: userID,
				Type:   models.GrantTypeStorage,
				Source: source,
				Bytes:  storage,
			}
			if err := s.grantManager.CreateAllowanceGrantLocked(ctx, userID, storageGrant, tx); err != nil {
				return tx
			}
			AllowanceAdded.WithLabelValues(sourceLabel).Inc()
		}

		// Create upload allowance grant
		if upload > 0 {
			uploadGrant := &models.AllowanceGrant{
				UserID: userID,
				Type:   models.GrantTypeUpload,
				Source: source,
				Bytes:  upload,
			}
			if err := s.grantManager.CreateAllowanceGrantLocked(ctx, userID, uploadGrant, tx); err != nil {
				return tx
			}
			AllowanceAdded.WithLabelValues(sourceLabel).Inc()
		}

		// Create download allowance grant
		if download > 0 {
			downloadGrant := &models.AllowanceGrant{
				UserID: userID,
				Type:   models.GrantTypeDownload,
				Source: source,
				Bytes:  download,
			}
			if err := s.grantManager.CreateAllowanceGrantLocked(ctx, userID, downloadGrant, tx); err != nil {
				return tx
			}
			AllowanceAdded.WithLabelValues(sourceLabel).Inc()
		}

		return nil
	})

}

func (s *QuotaServiceDefault) DeductAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.DeductAllowance")
	defer span.End()

	if s.grantManager == nil {
		return fmt.Errorf("grant manager not initialized")
	}

	// Handle all deductions in a single transaction for consistency
	return db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		// Deduct storage allowance
		if storage > 0 {
			detail := &models.UserUsageDetail{
				UserID:    userID,
				Type:      models.UsageTypeStorageRemove,
				Bytes:     storage,
				Timestamp: time.Now().UTC(),
			}
			if err := tx.Create(detail).Error; err != nil {
				return tx
			}

			_, err := s.grantManager.ConsumeFromGrants(ctx, userID, models.GrantTypeStorage, storage, detail.ID, tx)
			if err != nil {
				return tx
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
				return tx
			}

			_, err := s.grantManager.ConsumeFromGrants(ctx, userID, models.GrantTypeUpload, upload, detail.ID, tx)
			if err != nil {
				return tx
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
				return tx
			}

			_, err := s.grantManager.ConsumeFromGrants(ctx, userID, models.GrantTypeDownload, download, detail.ID, tx)
			if err != nil {
				return tx
			}
		}

		return nil
	})

}

func (s *QuotaServiceDefault) GetAllowanceBalance(ctx context.Context, userID uint) (*pluginCore.AllowanceBalance, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetAllowanceBalance")
	defer span.End()

	if s.configManager == nil {
		return nil, fmt.Errorf("config manager not initialized")
	}

	grants, err := s.configManager.GetUserAllowanceGrants(ctx, userID)
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

	AllowanceBalance.WithLabelValues(LabelTypeStorage).Set(float64(balance.StorageRemaining))
	AllowanceBalance.WithLabelValues(LabelTypeUpload).Set(float64(balance.UploadRemaining))
	AllowanceBalance.WithLabelValues(LabelTypeDownload).Set(float64(balance.DownloadRemaining))

	return balance, nil
}

func (s *QuotaServiceDefault) ResetAllowance(ctx context.Context, userID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.ResetAllowance")
	defer span.End()

	if s.grantManager == nil {
		return fmt.Errorf("grant manager not initialized")
	}

	// Get all active grants for the user
	grants, err := s.grantManager.GetActiveGrants(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user grants: %w", err)
	}

	// Deactivate all grants
	for _, grant := range grants {
		if err := s.grantManager.DeactivateGrant(ctx, grant.ID); err != nil {
			s.Logger().Warn("Failed to deactivate grant", zap.Uint("grantID", grant.ID), zap.Error(err))
			// Continue with other grants even if one fails
		}
	}

	return nil
}

// System Management
// TODO: Implement reconciliation logic
func (s *QuotaServiceDefault) Reconcile(ctx context.Context) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.Reconcile")
	defer span.End()

	// Implementation would depend on specific reconciliation needs
	// This is a placeholder for now
	s.Logger().Info("Reconciliation started")

	// In a real implementation, this might:
	// 1. Check for inconsistencies between usage details and daily quotas
	// 2. Update any grants that need expiration handling
	// 3. Clean up any orphaned records

	return nil
}

func (s *QuotaServiceDefault) CleanupOldRecords(ctx context.Context, retentionDays int) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CleanupOldRecords")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	cutoffDate := time.Now().UTC().AddDate(0, 0, -retentionDays)

	// Delete old usage details
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("timestamp < ?", cutoffDate).Delete(&models.UserUsageDetail{})
	}); err != nil {
		return fmt.Errorf("failed to cleanup old usage details: %w", err)
	}

	// Delete old allowance consumptions
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Where("consumption_date < ?", cutoffDate).Delete(&models.AllowanceConsumption{})
	}); err != nil {
		return fmt.Errorf("failed to cleanup old allowance consumptions: %w", err)
	}

	s.Logger().Info("Old records cleanup completed", zap.Int("retentionDays", retentionDays))
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

// GetConfig implements core.Configurable
func (s *QuotaServiceDefault) GetConfig() (any, error) {
	return &config.QuotaConfig{}, nil
}
