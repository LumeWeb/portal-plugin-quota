package policies

import (
	"errors"
	"fmt"

	"github.com/docker/go-units"
	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DefaultLimitResolver implements the LimitResolver interface
type DefaultLimitResolver struct {
	ctx          core.Context
	db           *gorm.DB
	logger       *core.Logger
	quotaService pluginCore.QuotaService
}

// NewLimitResolver creates a new default limit resolver
func NewLimitResolver(ctx core.Context, quotaService pluginCore.QuotaService) *DefaultLimitResolver {
	return &DefaultLimitResolver{
		ctx:          ctx,
		db:           ctx.DB(),
		logger:       ctx.NamedLogger("quota.LimitResolver"),
		quotaService: quotaService,
	}
}

// ResolveEffectiveLimits resolves the effective limits for a user based on their configuration
func (r *DefaultLimitResolver) ResolveEffectiveLimits(config *models.UserQuotaConfig, policy models.EnforcementPolicy) (*pluginCore.EffectiveLimits, error) {
	if config == nil {
		return nil, fmt.Errorf("quota config is nil")
	}

	limits := &pluginCore.EffectiveLimits{
		UserID:            config.UserID,
		EnforcementPolicy: pluginCore.EnforcementPolicy(policy),
	}

	// Get quota plan if assigned
	var plan *models.QuotaPlan
	var err error
	if config.QuotaPlanID != nil {
		plan, err = r.quotaService.GetQuotaPlanManager().GetQuotaPlanByID(*config.QuotaPlanID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve quota plan: %w", err)
		}
	} else {
		// Try to get default plan
		plan, err = r.quotaService.GetQuotaPlanManager().GetDefaultQuotaPlan()
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("failed to retrieve default quota plan: %w", err)
			}
			// No default plan found, continue with nil plan
		}
	}

	// Apply plan limits first (if available)
	if plan != nil {
		// Check if plan is active
		if plan.IsActive != nil && !*plan.IsActive {
			return nil, fmt.Errorf("quota plan is inactive")
		}
		if err := r.applyPlanLimits(limits, plan); err != nil {
			return nil, err
		}
		limits.QuotaPlanID = lo.ToPtr(uint64(plan.ID))
	}

	// Override with user-specific limits
	if err := r.applyUserLimits(limits, config); err != nil {
		return nil, err
	}

	// Apply policy-specific validation
	if policy == models.EnforcementPolicyHardLimits {
		return r.validateHardLimits(limits, config)
	}

	// Validate threshold vs limit for threshold policy
	if policy == models.EnforcementPolicyThreshold {
		if limits.UploadThreshold != nil && limits.UploadDailyLimit != nil {
			if err := r.ValidateThresholdVsLimit(int64(*limits.UploadThreshold), int64(*limits.UploadDailyLimit), "upload threshold"); err != nil {
				return nil, err
			}
		}
		if limits.DownloadThreshold != nil && limits.DownloadDailyLimit != nil {
			if err := r.ValidateThresholdVsLimit(int64(*limits.DownloadThreshold), int64(*limits.DownloadDailyLimit), "download threshold"); err != nil {
				return nil, err
			}
		}
		if limits.StorageThreshold != nil && limits.StorageLimit != nil {
			if err := r.ValidateThresholdVsLimit(int64(*limits.StorageThreshold), int64(*limits.StorageLimit), "storage threshold"); err != nil {
				return nil, err
			}
		}
	}

	return limits, nil
}

// ValidateThresholdVsLimit ensures threshold cannot exceed limit
func (r *DefaultLimitResolver) ValidateThresholdVsLimit(thresholdValue, limitValue int64, thresholdType string) error {
	// If limit is disabled (0), skip threshold validation
	if limitValue == 0 {
		return nil
	}

	if thresholdValue > limitValue && limitValue != -1 {
		return fmt.Errorf("threshold cannot exceed limit")
	}
	return nil
}

// ValidateThresholdValue validates that a threshold value is reasonable
func (r *DefaultLimitResolver) ValidateThresholdValue(thresholdValue int64, thresholdType string) error {
	// Allow -1 (unlimited), 0 (always warn), and positive values
	if thresholdValue < -1 {
		return fmt.Errorf("invalid %s value", thresholdType)
	}

	// Check if the value is unreasonably large (10 PiB should be enough for most use cases)
	if thresholdValue > 0 && thresholdValue > int64(10*units.PiB) {
		return fmt.Errorf("threshold value %d is unreasonably large", thresholdValue)
	}

	return nil
}

// ApplyLimit converts and validates database limit values to core limit values
func (r *DefaultLimitResolver) ApplyLimit(dest **uint64, source int64, limitName string, options ...pluginCore.LimitOption) error {
	config := &pluginCore.LimitConfig{}
	for _, opt := range options {
		opt(config)
	}

	// Allow -1 (unlimited), 0 (disabled), and positive values
	if source < -1 {
		return fmt.Errorf("invalid %s: %d (must be -1, 0, or positive)", limitName, source)
	}

	// Check if the value is unreasonably large (1 PiB should be enough for most use cases)
	if source > 0 && source > int64(units.PiB) {
		return fmt.Errorf("limit value %d is unreasonably large", source)
	}

	var convertedValue *uint64
	if source == -1 || (config.TreatZeroAsNil && source == 0) {
		convertedValue = nil // unlimited or disabled (treated as nil)
	} else {
		converted := uint64(source)
		convertedValue = &converted
	}

	*dest = convertedValue
	return nil
}

// applyPlanLimits applies limits from a quota plan
func (r *DefaultLimitResolver) applyPlanLimits(limits *pluginCore.EffectiveLimits, plan *models.QuotaPlan) error {
	// Apply basic limits
	if err := r.ApplyLimit(&limits.StorageLimit, plan.StorageLimit, "storage limit in quota plan", pluginCore.WithTreatZeroAsNil()); err != nil {
		return err
	}
	if err := r.ApplyLimit(&limits.UploadDailyLimit, plan.UploadDailyLimit, "upload daily limit in quota plan", pluginCore.WithTreatZeroAsNil()); err != nil {
		return err
	}
	if err := r.ApplyLimit(&limits.DownloadDailyLimit, plan.DownloadDailyLimit, "download daily limit in quota plan", pluginCore.WithTreatZeroAsNil()); err != nil {
		return err
	}
	if err := r.ApplyLimit(&limits.UploadTotalLimit, plan.UploadTotalLimit, "upload total limit in quota plan", pluginCore.WithTreatZeroAsNil()); err != nil {
		return err
	}
	if err := r.ApplyLimit(&limits.DownloadTotalLimit, plan.DownloadTotalLimit, "download total limit in quota plan", pluginCore.WithTreatZeroAsNil()); err != nil {
		return err
	}

	// Apply thresholds (for threshold policy)
	if plan.StorageThreshold != nil {
		if err := r.ValidateThresholdValue(*plan.StorageThreshold, "storage threshold"); err != nil {
			return err
		}
		limits.StorageThreshold = r.convertLimitValue(*plan.StorageThreshold)
		limits.HasStorageThresholdConfig = true
	}
	if plan.UploadThreshold != nil {
		if err := r.ValidateThresholdValue(*plan.UploadThreshold, "upload threshold"); err != nil {
			return err
		}
		limits.UploadThreshold = r.convertLimitValue(*plan.UploadThreshold)
		limits.HasUploadThresholdConfig = true
	}
	if plan.DownloadThreshold != nil {
		if err := r.ValidateThresholdValue(*plan.DownloadThreshold, "download threshold"); err != nil {
			return err
		}
		limits.DownloadThreshold = r.convertLimitValue(*plan.DownloadThreshold)
		limits.HasDownloadThresholdConfig = true
	}

	// Mark which limits were configured (only if they were actually applied)
	if limits.StorageLimit != nil {
		limits.HasStorageLimitConfig = true
	}
	if limits.UploadDailyLimit != nil {
		limits.HasUploadDailyLimitConfig = true
	}
	if limits.DownloadDailyLimit != nil {
		limits.HasDownloadDailyLimitConfig = true
	}
	if limits.UploadTotalLimit != nil {
		limits.HasUploadTotalLimitConfig = true
	}
	if limits.DownloadTotalLimit != nil {
		limits.HasDownloadTotalLimitConfig = true
	}
	return nil
}

// applyUserLimits applies user-specific limits that override plan limits
func (r *DefaultLimitResolver) applyUserLimits(limits *pluginCore.EffectiveLimits, config *models.UserQuotaConfig) error {
	if config.StorageLimit != nil {
		if err := r.ApplyLimit(&limits.StorageLimit, *config.StorageLimit, "storage limit in user config"); err != nil {
			return err
		}
		limits.HasStorageLimitConfig = true
	}
	if config.UploadDailyLimit != nil {
		if err := r.ApplyLimit(&limits.UploadDailyLimit, *config.UploadDailyLimit, "upload daily limit in user config"); err != nil {
			return err
		}
		limits.HasUploadDailyLimitConfig = true
	}
	if config.DownloadDailyLimit != nil {
		if err := r.ApplyLimit(&limits.DownloadDailyLimit, *config.DownloadDailyLimit, "download daily limit in user config"); err != nil {
			return err
		}
		limits.HasDownloadDailyLimitConfig = true
	}
	if config.UploadTotalLimit != nil {
		if err := r.ApplyLimit(&limits.UploadTotalLimit, *config.UploadTotalLimit, "upload total limit in user config"); err != nil {
			return err
		}
		limits.HasUploadTotalLimitConfig = true
	}
	if config.DownloadTotalLimit != nil {
		if err := r.ApplyLimit(&limits.DownloadTotalLimit, *config.DownloadTotalLimit, "download total limit in user config"); err != nil {
			return err
		}
		limits.HasDownloadTotalLimitConfig = true
	}
	if config.StorageThreshold != nil {
		if err := r.ValidateThresholdValue(*config.StorageThreshold, "storage threshold"); err != nil {
			return err
		}
		limits.StorageThreshold = r.convertLimitValue(*config.StorageThreshold)
		limits.HasStorageThresholdConfig = true
	}
	if config.UploadThreshold != nil {
		if err := r.ValidateThresholdValue(*config.UploadThreshold, "upload threshold"); err != nil {
			return err
		}
		limits.UploadThreshold = r.convertLimitValue(*config.UploadThreshold)
		limits.HasUploadThresholdConfig = true
	}
	if config.DownloadThreshold != nil {
		if err := r.ValidateThresholdValue(*config.DownloadThreshold, "download threshold"); err != nil {
			return err
		}
		limits.DownloadThreshold = r.convertLimitValue(*config.DownloadThreshold)
		limits.HasDownloadThresholdConfig = true
	}
	return nil
}

// validateHardLimits performs additional validation for hard limits policy
func (r *DefaultLimitResolver) validateHardLimits(limits *pluginCore.EffectiveLimits, config *models.UserQuotaConfig) (*pluginCore.EffectiveLimits, error) {
	// Check if any limits are configured
	hasLimits := limits.HasAnyLimits()

	if !hasLimits {
		// Check if there's a default plan that could provide limits
		_, err := r.quotaService.GetQuotaPlanManager().GetDefaultQuotaPlan()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("no limits configured for hard limits policy")
			}
			return nil, fmt.Errorf("failed to retrieve default quota plan: %w", err)
		}
		// If we found a default plan, we should have limits from it
		if !limits.HasAnyLimits() {
			return nil, fmt.Errorf("no limits configured for hard limits policy")
		}
	}

	return limits, nil
}

// convertLimitValue converts database int64 limit values to core *uint64 values
func (r *DefaultLimitResolver) convertLimitValue(value int64) *uint64 {
	if value == -1 {
		return nil // unlimited
	}
	converted := uint64(value)
	return &converted
}

// applyLimit is a helper method that uses the ApplyLimit method with default options
func (r *DefaultLimitResolver) applyLimit(dest **uint64, source int64, limitName string, options ...pluginCore.LimitOption) {
	// Default options for limit resolution
	defaultOptions := []pluginCore.LimitOption{pluginCore.WithAllowUnlimited()}
	allOptions := append(defaultOptions, options...)

	if err := r.ApplyLimit(dest, source, limitName, allOptions...); err != nil {
		// Log error but don't fail - this should be caught during validation
		r.logger.Warn("Failed to apply limit",
			zap.String("limitName", limitName),
			zap.Int64("value", source),
			zap.Error(err))
	}
}
