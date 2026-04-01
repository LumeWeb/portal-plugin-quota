package quota

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	quotaLock "go.lumeweb.com/portal-plugin-quota/internal/lock"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/service/managers"
	"go.lumeweb.com/portal-plugin-quota/internal/service/policies"
	"go.lumeweb.com/portal/core"
	portalModels "go.lumeweb.com/portal/db/models"
	"go.lumeweb.com/portal/db"
	"go.lumeweb.com/queryutil"
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
	lockManager     quotaLock.LockManager
	uploadService   core.UploadService
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
			service.lockManager = quotaLock.NewLockManager(ctx)

			// Initialize limit resolver
			service.limitResolver = policies.NewLimitResolver(ctx, service)

			// Initialize plan manager
			service.planManager = policies.NewQuotaPlanManager(ctx, ctx.DB(), ctx.Logger())

			// Initialize upload service
			uploadSvc := core.GetService[core.UploadService](ctx, core.UPLOAD_SERVICE)
			if uploadSvc != nil {
				service.uploadService = uploadSvc
			}

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

// checkQuotaWithLock is a helper function that wraps quota checking with lock acquisition
// and metric tracking to reduce code duplication across quota check methods.
func (s *QuotaServiceDefault) checkQuotaWithLock(ctx context.Context, userID uint, requestedBytes uint64,
	checkFunc func(ctx context.Context, enforcer pluginCore.PolicyEnforcer, config *models.UserQuotaConfig, bytes uint64) (pluginCore.QuotaCheckResult, error),
	metric *prometheus.CounterVec) (pluginCore.QuotaCheckResult, error) {

	if s.configManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("config manager not initialized")
	}
	if s.lockManager == nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("lock manager not initialized")
	}

	// Acquire lock for this user to prevent race conditions
	lock, err := s.lockManager.AcquireLock(ctx, userID)
	if err != nil {
		return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to acquire quota lock: %w", err)
	}
	defer lock.Release()

	return core.MetricTrackResult(
		OperationDuration.WithLabelValues(LabelOperationCheck),
		metric.WithLabelValues(LabelStatusError),
		func() (pluginCore.QuotaCheckResult, error) {
			config, err := s.configManager.GetUserQuotaConfig(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get user quota config: %w", err)
			}

			enforcer, err := s.configManager.GetPolicyEnforcer(ctx, userID)
			if err != nil {
				return pluginCore.QuotaCheckResult{}, fmt.Errorf("failed to get policy enforcer: %w", err)
			}

			result, err := checkFunc(ctx, enforcer, config, requestedBytes)
			if err != nil {
				return result, err
			}

			if !result.Allowed {
				metric.WithLabelValues(LabelStatusDenied).Inc()
			} else {
				metric.WithLabelValues(LabelStatusAllowed).Inc()
			}

			return result, nil
		},
	)
}

func (s *QuotaServiceDefault) CheckUploadQuota(ctx context.Context, userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckUploadQuota")
	defer span.End()

	return s.checkQuotaWithLock(ctx, userID, requestedBytes,
		func(ctx context.Context, enforcer pluginCore.PolicyEnforcer, config *models.UserQuotaConfig, bytes uint64) (pluginCore.QuotaCheckResult, error) {
			return enforcer.CheckUploadQuota(ctx, config, bytes)
		},
		UploadChecked,
	)
}

func (s *QuotaServiceDefault) CheckDownloadQuota(ctx context.Context, userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckDownloadQuota")
	defer span.End()

	return s.checkQuotaWithLock(ctx, userID, requestedBytes,
		func(ctx context.Context, enforcer pluginCore.PolicyEnforcer, config *models.UserQuotaConfig, bytes uint64) (pluginCore.QuotaCheckResult, error) {
			return enforcer.CheckDownloadQuota(ctx, config, bytes)
		},
		DownloadChecked,
	)
}

func (s *QuotaServiceDefault) CheckStorageQuota(ctx context.Context, userID uint, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckStorageQuota")
	defer span.End()

	return s.checkQuotaWithLock(ctx, userID, requestedBytes,
		func(ctx context.Context, enforcer pluginCore.PolicyEnforcer, config *models.UserQuotaConfig, bytes uint64) (pluginCore.QuotaCheckResult, error) {
			return enforcer.CheckStorageQuota(ctx, config, bytes)
		},
		StorageChecked,
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

func (s *QuotaServiceDefault) GetUsageHistoryDateRange(ctx context.Context, userID uint, usageType pluginCore.UsageType, startTime, endTime time.Time) ([]*pluginCore.UsagePoint, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetUsageHistoryDateRange")
	defer span.End()

	if s.usageManager == nil {
		return nil, fmt.Errorf("usage manager not initialized")
	}

	return s.usageManager.GetUsageHistoryDateRange(ctx, userID, usageType, startTime, endTime)
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

	// Block creation with IsDefault=true to prevent duplicate key violations
	// Use SetDefaultQuotaPlan to change which plan is default after creation
	if plan.IsDefault {
		return fmt.Errorf("cannot create plan with IsDefault=true - set an existing plan as default using SetDefaultQuotaPlan")
	}

	// Check if a plan with the same name already exists
	// Note: We perform this pre-flight check as an optimization for early feedback,
	// but we also handle UNIQUE constraint violations from the database during creation
	// to prevent race conditions (TOCTOU) where concurrent requests bypass this check.
	existingPlan, err := s.planManager.GetQuotaPlanByName(ctx, plan.Name)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) && !errors.Is(err, models.ErrQuotaPlanNotFound) {
		// Real database error (connection, timeout, etc.) - fail fast
		return fmt.Errorf("failed to check for existing plan: %w", err)
	}
	if existingPlan != nil {
		// Plan already exists - return conflict error
		return models.ErrQuotaPlanNameExists
	}

	err = core.MetricTrack(
		nil,
		policies.PlanOperationsErr.WithLabelValues(policies.LabelPlanOperationCreate),
		func() error {
			if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
				return tx.Create(plan)
			}); err != nil {
				// Check for unique constraint violation and return proper error
				if isUniqueConstraintError(err) {
					return models.ErrQuotaPlanNameExists
				}
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

	// Fetch the existing plan to check if IsDefault is being changed
	var existing models.QuotaPlan
	if err := s.DB().Where("id = ?", planID).First(&existing).Error; err != nil {
		return fmt.Errorf("failed to fetch existing quota plan: %w", err)
	}

	// Block all attempts to change IsDefault through UpdateQuotaPlan
	// Only SetDefaultQuotaPlan can change default status to prevent duplicate key violations
	if existing.IsDefault != plan.IsDefault {
		return fmt.Errorf("cannot change default quota plan status through update - use SetDefaultQuotaPlan to change the default plan")
	}

	err := core.MetricTrack(
		nil,
		policies.PlanOperationsErr.WithLabelValues(policies.LabelPlanOperationUpdate),
		func() error {
			if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
				// Fetch the existing plan to ensure we update the right record
				var existing models.QuotaPlan
				if result := tx.Where("id = ?", planID).First(&existing); result.Error != nil {
					return result
				}

				// Update the existing plan's fields with new values
				// Note: IsDefault is not updated here unless handled by SetDefaultQuotaPlan
				existing.Name = plan.Name
				existing.Description = plan.Description
				existing.WindowType = plan.WindowType
				existing.WindowDuration = plan.WindowDuration
				existing.WindowStartHour = plan.WindowStartHour
				existing.WindowTimezone = plan.WindowTimezone
				existing.StorageLimitBytes = plan.StorageLimitBytes
				existing.UploadLimitBytes = plan.UploadLimitBytes
				existing.DownloadLimitBytes = plan.DownloadLimitBytes
				existing.StorageThreshold = plan.StorageThreshold
				existing.UploadThreshold = plan.UploadThreshold
				existing.DownloadThreshold = plan.DownloadThreshold
				// existing.IsDefault intentionally not set here - must use SetDefaultQuotaPlan
				existing.IsActive = plan.IsActive

				// Save the existing record to trigger BeforeSave and BeforeUpdate hooks
				return tx.Save(&existing)
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

	// Check if any users are assigned to this plan
	var assignedUserCount int64
	if err := s.DB().WithContext(ctx).Model(&models.UserQuotaConfig{}).
		Where("quota_plan_id = ?", planID).
		Count(&assignedUserCount).Error; err != nil {
		return fmt.Errorf("failed to check plan assignments: %w", err)
	}

	if assignedUserCount > 0 {
		return fmt.Errorf("plan %d has %d users assigned, cannot delete", planID, assignedUserCount)
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

// ListQuotaPlans retrieves a paginated and filtered list of quota plans
func (s *QuotaServiceDefault) ListQuotaPlans(ctx context.Context, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*models.QuotaPlan, int64, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.ListQuotaPlans")
	defer span.End()

	if s.DB() == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	var plans []*models.QuotaPlan
	var total int64

	query := s.DB().Model(&models.QuotaPlan{})

	// Apply filters, sorts and pagination using queryutil helpers
	query = queryutil.ApplyFilters(query, filters, nil)
	query = queryutil.ApplySort(query, sorts)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count quota plans: %w", err)
	}

	query = queryutil.ApplyPagination(query, pagination)

	if err := query.Find(&plans).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch quota plans: %w", err)
	}

	return plans, total, nil
}

func (s *QuotaServiceDefault) SetDefaultQuotaPlan(ctx context.Context, planID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.SetDefaultQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	// Perform both updates atomically in a transaction
	// Order is critical: unset old default first, then set new one
	// This ensures never more than one default, avoiding constraint violations
	return db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		// First, fetch and unset the current default plan (if any)
		// Use Unscoped() to include soft-deleted records, which prevents
		// duplicate key violations when trying to set a new default plan
		// after the old default has been soft-deleted
		var currentDefault models.QuotaPlan
		if tx.Unscoped().Where("is_default = ?", true).First(&currentDefault).Error == nil {
			// Also use Unscoped() when saving to ensure we can update soft-deleted records
			currentDefault.IsDefault = false
			if result := tx.Unscoped().Save(&currentDefault); result.Error != nil {
				return result
			}
		}

		// Then, fetch and set the new default plan
		// Must fetch the full model to trigger validation hooks
		var newDefault models.QuotaPlan
		if result := tx.Where("id = ? AND is_active = ?", planID, true).First(&newDefault); result.Error != nil {
			return result
		}

		newDefault.IsDefault = true
		return tx.Save(&newDefault)
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
			return result
		}

		// Update the user's quota config with the plan ID
		return tx.Model(&models.UserQuotaConfig{}).Where("user_id = ?", userID).UpdateColumn("quota_plan_id", planID)
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

// ListUserQuotaConfigs retrieves a paginated and filtered list of user quota configurations
func (s *QuotaServiceDefault) ListUserQuotaConfigs(ctx context.Context, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*models.UserQuotaConfig, int64, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.ListUserQuotaConfigs")
	defer span.End()

	if s.DB() == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	var configs []*models.UserQuotaConfig
	var total int64

	query := s.DB().WithContext(ctx).Model(&models.UserQuotaConfig{})

	// Apply filters, sorts and pagination using queryutil helpers
	query = queryutil.ApplyFilters(query, filters, nil)
	query = queryutil.ApplySort(query, sorts)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count user quota configs: %w", err)
	}

	query = queryutil.ApplyPagination(query, pagination)

	if err := query.Find(&configs).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to fetch user quota configs: %w", err)
	}

	return configs, total, nil
}

// UpdateUserQuotaConfig updates a user's quota configuration and returns the updated config
func (s *QuotaServiceDefault) UpdateUserQuotaConfig(ctx context.Context, userID uint, update *pluginCore.UserQuotaConfigUpdate) (*models.UserQuotaConfig, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.UpdateUserQuotaConfig")
	defer span.End()

	if s.DB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if userID == 0 {
		return nil, fmt.Errorf("invalid user ID")
	}

	// Build update map with only provided fields
	updates := make(map[string]interface{})

	if update.EnforcementPolicy != nil {
		updates["enforcement_policy"] = *update.EnforcementPolicy
	}
	if update.QuotaPlanID != nil {
		updates["quota_plan_id"] = *update.QuotaPlanID
	}
	if update.WindowType != nil {
		updates["window_type"] = *update.WindowType
	}
	if update.WindowDuration != nil {
		updates["window_duration"] = update.WindowDuration
	}
	if update.WindowStartHour != nil {
		updates["window_start_hour"] = update.WindowStartHour
	}
	if update.WindowTimezone != nil {
		updates["window_timezone"] = update.WindowTimezone
	}
	if update.StorageLimitBytes != nil {
		updates["storage_limit_bytes"] = *update.StorageLimitBytes
	}
	if update.UploadLimitBytes != nil {
		updates["upload_limit_bytes"] = *update.UploadLimitBytes
	}
	if update.DownloadLimitBytes != nil {
		updates["download_limit_bytes"] = *update.DownloadLimitBytes
	}
	if update.StorageThreshold != nil {
		updates["storage_threshold"] = update.StorageThreshold
	}
	if update.UploadThreshold != nil {
		updates["upload_threshold"] = update.UploadThreshold
	}
	if update.DownloadThreshold != nil {
		updates["download_threshold"] = update.DownloadThreshold
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("no fields to update")
	}

	var updatedConfig models.UserQuotaConfig

	// Ensure the user has a quota config first, then apply updates
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		var config models.UserQuotaConfig
		result := tx.Where("user_id = ?", userID).FirstOrCreate(&config, &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		})
		if result.Error != nil {
			return result
		}

		// Apply the updates to the found/created config
		if result = tx.Model(&config).Updates(updates); result.Error != nil {
			return result
		}

		// Reload the config to return the updated state
		return tx.Where("user_id = ?", userID).First(&updatedConfig)
	}); err != nil {
		return nil, fmt.Errorf("failed to update user quota config: %w", err)
	}

	return &updatedConfig, nil
}

// ResetUserQuotaPlan resets a user's quota plan assignment to NULL
func (s *QuotaServiceDefault) ResetUserQuotaPlan(ctx context.Context, userID uint) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.ResetUserQuotaPlan")
	defer span.End()

	if s.DB() == nil {
		return fmt.Errorf("database not initialized")
	}

	if userID == 0 {
		return fmt.Errorf("invalid user ID")
	}

	// Update the quota config to remove the plan ID
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		return tx.Model(&models.UserQuotaConfig{}).Where("user_id = ?", userID).UpdateColumn("quota_plan_id", nil)
	}); err != nil {
		return fmt.Errorf("failed to reset user quota plan: %w", err)
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
				_ = tx.AddError(err)
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
				_ = tx.AddError(err)
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
				_ = tx.AddError(err)
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
				_ = tx.AddError(err)
				return tx
			}

			_, err := s.grantManager.ConsumeFromGrants(ctx, userID, models.GrantTypeStorage, storage, detail.ID, tx)
			if err != nil {
				_ = tx.AddError(err)
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
				_ = tx.AddError(err)
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
				_ = tx.AddError(err)
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
func (s *QuotaServiceDefault) GetSystemStats(ctx context.Context) (*pluginCore.SystemStats, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.GetSystemStats")
	defer span.End()

	if s.DB() == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var totalUsers, activeUsers, totalPlans, activePlans, totalGrants, activeGrants int64

	// Count users
	db := s.DB()
	if err := db.Model(&models.UserQuotaConfig{}).Count(&totalUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count total users: %w", err)
	}
	if err := db.Model(&models.UserQuotaConfig{}).Where("id > 0").Count(&activeUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to count active users: %w", err)
	}

	// Count plans
	if err := db.Model(&models.QuotaPlan{}).Count(&totalPlans).Error; err != nil {
		return nil, fmt.Errorf("failed to count total plans: %w", err)
	}
	if err := db.Model(&models.QuotaPlan{}).Where("is_active = ?", true).Count(&activePlans).Error; err != nil {
		return nil, fmt.Errorf("failed to count active plans: %w", err)
	}

	// Count grants
	if err := db.Model(&models.AllowanceGrant{}).Count(&totalGrants).Error; err != nil {
		return nil, fmt.Errorf("failed to count total grants: %w", err)
	}
	if err := db.Model(&models.AllowanceGrant{}).Where("is_active = ?", true).Count(&activeGrants).Error; err != nil {
		return nil, fmt.Errorf("failed to count active grants: %w", err)
	}

	// Calculate current usage
	var storageUsage, uploadUsage, downloadUsage int64
	if err := db.Model(&models.UserQuota{}).Select("COALESCE(SUM(bytes_uploaded), 0)").Scan(&uploadUsage).Error; err != nil {
		return nil, fmt.Errorf("failed to sum upload usage: %w", err)
	}
	if err := db.Model(&models.UserQuota{}).Select("COALESCE(SUM(bytes_downloaded), 0)").Scan(&downloadUsage).Error; err != nil {
		return nil, fmt.Errorf("failed to sum download usage: %w", err)
	}
	if err := db.Model(&models.UserQuota{}).Select("COALESCE(SUM(bytes_stored), 0)").Scan(&storageUsage).Error; err != nil {
		return nil, fmt.Errorf("failed to sum storage usage: %w", err)
	}

	stats := &pluginCore.SystemStats{
		TotalUsers:   totalUsers,
		ActiveUsers:  activeUsers,
		TotalPlans:   totalPlans,
		ActivePlans:  activePlans,
		TotalGrants:  totalGrants,
		ActiveGrants: activeGrants,
		CurrentUsage: pluginCore.Usage{
			BytesUploaded:   uint64(uploadUsage),
			BytesDownloaded: uint64(downloadUsage),
			BytesStored:     uint64(storageUsage),
		},
		TotalUsageBytes: uint64(storageUsage) + uint64(uploadUsage) + uint64(downloadUsage),
	}

	return stats, nil
}

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

// CheckGroupQuotaIteration performs iterative filtering to determine if a group of users
// can collectively handle a quota-based operation with anonymous distribution.
//
// The algorithm:
// 1. Calculates shared bytes for current user count
// 2. Checks each user's quota availability
// 3. Filters out users without sufficient quota
// 4. Re-calculates share with remaining users
// 5. Repeats until stabilized or empty
//
// Parameters:
//   - users: Initial list of user IDs to check
//   - requiredBytes: Total bytes to be shared across users
//   - checkQuota: Function that checks if a user has sufficient quota for a given byte amount
//   - precision: Decimal precision for shared bytes calculation
//
// Returns:
//   - bool: True if some users can collectively handle the operation, false otherwise
//   - error: Error from quota checking (nil if no errors occurred)
func CheckGroupQuotaIteration(
	users []uint,
	requiredBytes uint64,
	checkQuota func(uint, uint64) (bool, error),
	precision int,
) (bool, error) {
	currentUsers := users

	for {
		userCount := uint(len(currentUsers))
		sharedBytes := pluginCore.CalculateSharedBytes(requiredBytes, userCount, precision)

		// Check each user and collect those with sufficient quota
		sufficientUsers := make([]uint, 0, len(currentUsers))
		for _, userID := range currentUsers {
			hasSufficient, err := checkQuota(userID, sharedBytes)
			if err != nil {
				return false, err
			}

			if hasSufficient {
				sufficientUsers = append(sufficientUsers, userID)
			}
		}

		// No users can handle their share → cannot serve
		if len(sufficientUsers) == 0 {
			return false, nil
		}

		// All current users have sufficient quota → stabilized
		if len(sufficientUsers) == len(currentUsers) {
			return true, nil
		}

		// Need to re-calculate with filtered users
		currentUsers = sufficientUsers
	}
}

// CheckCIDGroupQuotaAvailability checks if the group of users pinning content can collectively handle an operation
// on content identified by CID, following anonymous distribution logic with iterative filtering.
//
// This method:
// 1. Resolves the CID to get users who have pinned this content
// 2. Iteratively calculates per-user cost using anonymous distribution logic
// 3. Filters out users who lack sufficient quota for their share
// 4. Re-calculates share with remaining users, repeating until:
//    - No users remain (cannot serve) → returns false
//    - All remaining users have sufficient quota (stabilized) → returns true
//
// Algorithm rationale:
// - As users are excluded due to insufficient quota, the per-user cost increases
// - This may cause more users to be excluded, requiring multiple iterations
// - The process stabilizes when either no users can serve or all remaining users can
func (s *QuotaServiceDefault) CheckCIDGroupQuotaAvailability(ctx context.Context, cid core.StorageHash, requiredBytes uint64, usageType pluginCore.UsageType) (bool, error) {
	_, span := core.TraceMethod(ctx, "QuotaServiceDefault.CheckCIDGroupQuotaAvailability")
	defer span.End()

	// Try to resolve CID to upload ID
	uploadID, err := s.resolveCIDToUploadID(ctx, cid)
	if err != nil {
		s.Logger().Warn("Failed to resolve CID to upload ID",
			zap.Stringer("hash", cid),
			zap.Error(err))
		return false, fmt.Errorf("failed to resolve CID: %w", err)
	}

	if uploadID == 0 {
		s.Logger().Debug("No upload found for CID",
			zap.Stringer("hash", cid))
		return false, nil
	}

	// Get all users who have pinned this upload
	pinningUsers, err := s.getPinningUsersForUpload(ctx, uploadID)
	if err != nil {
		return false, fmt.Errorf("failed to get pinning users: %w", err)
	}

	if len(pinningUsers) == 0 {
		s.Logger().Debug("No users pinning the content",
			zap.Stringer("hash", cid),
			zap.Uint("uploadID", uploadID))
		return false, nil
	}

	// Use iterative filtering algorithm to check if group can handle the operation
	available, err := CheckGroupQuotaIteration(pinningUsers, requiredBytes, func(userID uint, bytes uint64) (bool, error) {
		return s.checkUserQuotaForAction(ctx, userID, bytes, usageType)
	}, s.config.SharedUsagePrecision)

	if err != nil {
		s.Logger().Error("Group quota check failed",
			zap.Stringer("hash", cid),
			zap.Error(err))
		return false, fmt.Errorf("group quota check failed: %w", err)
	}

	if available {
		s.Logger().Debug("Group can handle CID operation",
			zap.Stringer("hash", cid),
			zap.Int("initialPinners", len(pinningUsers)),
			zap.Uint64("requiredBytes", requiredBytes),
			zap.String("usageType", string(usageType)))
	} else {
		s.Logger().Debug("Group cannot handle CID operation",
			zap.Stringer("hash", cid),
			zap.Int("initialPinners", len(pinningUsers)),
			zap.Uint64("requiredBytes", requiredBytes),
			zap.String("usageType", string(usageType)))
	}

	return available, nil

	return false, nil
}

// resolveCIDToUploadID attempts to resolve a StorageHash to an upload ID using the portal's upload service.
func (s *QuotaServiceDefault) resolveCIDToUploadID(ctx context.Context, storageHash core.StorageHash) (uint, error) {
	_, span := core.TraceMethod(ctx, "QuotaServiceDefault.resolveCIDToUploadID")
	defer span.End()

	// Try to get the upload by its hash
	upload, err := s.uploadService.GetUpload(ctx, storageHash)
	if err != nil {
		if errors.Is(err, core.ErrUploadNotFound) {
			s.Logger().Debug("No upload found for CID",
				zap.Stringer("hash", storageHash))
			return 0, nil
		}
		s.Logger().Warn("Failed to get upload by hash",
			zap.Stringer("hash", storageHash),
			zap.Error(err))
		return 0, fmt.Errorf("failed to get upload by hash: %w", err)
	}

	if upload == nil {
		s.Logger().Debug("Upload is nil for hash",
			zap.Stringer("hash", storageHash))
		return 0, nil
	}

	s.Logger().Debug("Successfully resolved hash to upload ID",
		zap.Stringer("hash", storageHash),
		zap.Uint("uploadID", upload.ID))

	return upload.ID, nil
}

// getPinningUsersForUpload returns all user IDs that have pinned the given upload
// Copied from UsageManager but added here for CID availability checking
func (s *QuotaServiceDefault) getPinningUsersForUpload(ctx context.Context, uploadID uint) ([]uint, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.getPinningUsersForUpload")
	defer span.End()

	usageManager := s.GetUsageManager()
	if usageManager == nil {
		return nil, fmt.Errorf("usage manager not available")
	}

	// Access pin service through the core
	pinService := core.GetService[core.PinService](s.Context(), core.PIN_SERVICE)
	if pinService == nil {
		return nil, fmt.Errorf("pin service not available")
	}

	pins, err := pinService.GetPinsByUploadID(ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pins for upload: %w", err)
	}

	return lo.Uniq(lo.Map(pins, func(pin *portalModels.Pin, _ int) uint {
		return pin.UserID
	})), nil
}

// checkUserQuotaForAction checks if a user has sufficient quota for a specific action
func (s *QuotaServiceDefault) checkUserQuotaForAction(ctx context.Context, userID uint, requiredBytes uint64, usageType pluginCore.UsageType) (bool, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.checkUserQuotaForAction")
	defer span.End()

	// Check based on usage type
	switch usageType {
	case pluginCore.UsageTypeUpload:
		return s.checkUploadQuotaWithUsage(ctx, userID, requiredBytes)
	case pluginCore.UsageTypeDownload:
		return s.checkDownloadQuotaWithUsage(ctx, userID, requiredBytes)
	case pluginCore.UsageTypeStorageAdd:
		return s.checkStorageQuotaWithUsage(ctx, userID, requiredBytes)
	default:
		return false, fmt.Errorf("unsupported usage type: %s", usageType)
	}
}

// checkUploadQuotaWithUsage checks if user has sufficient upload quota given current usage
func (s *QuotaServiceDefault) checkUploadQuotaWithUsage(ctx context.Context, userID uint, requiredBytes uint64) (bool, error) {
	result, err := s.CheckUploadQuota(ctx, userID, requiredBytes)
	if err != nil {
		return false, fmt.Errorf("failed to check upload quota: %w", err)
	}
	return result.Allowed, nil
}

// checkDownloadQuotaWithUsage checks if user has sufficient download quota given current usage
func (s *QuotaServiceDefault) checkDownloadQuotaWithUsage(ctx context.Context, userID uint, requiredBytes uint64) (bool, error) {
	result, err := s.CheckDownloadQuota(ctx, userID, requiredBytes)
	if err != nil {
		return false, fmt.Errorf("failed to check download quota: %w", err)
	}
	return result.Allowed, nil
}

// checkStorageQuotaWithUsage checks if user has sufficient storage quota given current usage
func (s *QuotaServiceDefault) checkStorageQuotaWithUsage(ctx context.Context, userID uint, requiredBytes uint64) (bool, error) {
	result, err := s.CheckStorageQuota(ctx, userID, requiredBytes)
	if err != nil {
		return false, fmt.Errorf("failed to check storage quota: %w", err)
	}
	return result.Allowed, nil
}

func (s *QuotaServiceDefault) CleanupOldRecords(ctx context.Context, retentionDays int) (int64, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.CleanupOldRecords")
	defer span.End()

	if s.DB() == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	cutoffDate := time.Now().UTC().AddDate(0, 0, -retentionDays)

	var totalDeleted int64

	// Delete old usage details
	usageDeleted := int64(0)
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		result := tx.Where("timestamp < ?", cutoffDate).Delete(&models.UserUsageDetail{})
		usageDeleted = result.RowsAffected
		return tx
	}); err != nil {
		return 0, fmt.Errorf("failed to cleanup old usage details: %w", err)
	}
	totalDeleted += usageDeleted

	// Delete old allowance consumptions
	consumptionDeleted := int64(0)
	if err := db.RetryableComponentTransaction(s, ctx, func(tx *gorm.DB) *gorm.DB {
		result := tx.Where("consumption_date < ?", cutoffDate).Delete(&models.AllowanceConsumption{})
		consumptionDeleted = result.RowsAffected
		return tx
	}); err != nil {
		return 0, fmt.Errorf("failed to cleanup old allowance consumptions: %w", err)
	}
	totalDeleted += consumptionDeleted

	s.Logger().Info("Old records cleanup completed", zap.Int("retentionDays", retentionDays), zap.Int64("totalDeleted", totalDeleted))
	return totalDeleted, nil
}

// Manager Getters
func (s *QuotaServiceDefault) GetUsageManager() pluginCore.UsageManager {
	return s.usageManager
}

func (s *QuotaServiceDefault) GetGrantManager() pluginCore.GrantManager {
	return s.grantManager
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

// isUniqueConstraintError checks if an error is a unique constraint violation
// for the quota_plan name field. This handles both MySQL and SQLite errors.
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}

	// Check the error message for unique constraint indicators
	errStr := err.Error()

	// MySQL unique constraint violation
	// Error 1062: Duplicate entry
	// Error 1586: Duplicate entry
	if strings.Contains(errStr, "Duplicate entry") &&
		strings.Contains(errStr, "key") &&
		strings.Contains(errStr, "quota_plans") {
		return true
	}

	// SQLite unique constraint violation
	if strings.Contains(errStr, "UNIQUE constraint failed") {
		return true
	}

	// GORM's built-in duplicate key error
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}

	// Additional check for MySQL ErrDuplicateEntry
	// This requires checking for the MySQL driver error
	// Note: This is a simplified check; in production you might want to
	// use type assertions to check for specific driver errors

	return false
}
