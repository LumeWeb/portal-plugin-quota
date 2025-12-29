package policies

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/go-units"
	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
)

// DefaultLimitResolver implements the LimitResolver interface
type DefaultLimitResolver struct {
	*core.BaseComponent
	quotaService pluginCore.QuotaService
}

// NewLimitResolver creates a new default limit resolver
func NewLimitResolver(ctx core.Context, quotaService pluginCore.QuotaService) *DefaultLimitResolver {
	return &DefaultLimitResolver{
		BaseComponent: core.NewBaseComponent(ctx),
		quotaService:  quotaService,
	}
}

// ResolveEffectiveLimits resolves the effective limits for a user based on their configuration
func (r *DefaultLimitResolver) ResolveEffectiveLimits(ctx context.Context, config *models.UserQuotaConfig, policy models.EnforcementPolicy) (*pluginCore.EffectiveLimits, error) {
	ctx, span := core.TraceMethod(ctx, "DefaultLimitResolver.ResolveEffectiveLimits")
	defer span.End()

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
		plan, err = r.quotaService.GetQuotaPlanManager().GetQuotaPlanByID(ctx, *config.QuotaPlanID)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve quota plan: %w", err)
		}
	} else {
		// Try to get default plan
		plan, err = r.quotaService.GetQuotaPlanManager().GetDefaultQuotaPlan(ctx)
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
		LimitResolved.WithLabelValues(LabelLimitSourcePlan).Inc()
	}

	// Override with user-specific limits
	if err := r.applyUserLimits(limits, config); err != nil {
		return nil, err
	}
	if limits.HasAnyLimits() {
		LimitResolved.WithLabelValues(LabelLimitSourceUser).Inc()
	}

	// Apply policy-specific validation
	if policy == models.EnforcementPolicyHardLimits {
		return r.validateHardLimits(ctx, limits, config)
	}

	// Validate threshold vs limit for threshold policy
	if policy == models.EnforcementPolicyThreshold {
		if limits.UploadThreshold != nil && limits.UploadDailyLimit != nil {
			if err := r.ValidateThresholdVsLimit(ctx, int64(*limits.UploadThreshold), int64(*limits.UploadDailyLimit), "upload threshold"); err != nil {
				return nil, err
			}
		}
		if limits.DownloadThreshold != nil && limits.DownloadDailyLimit != nil {
			if err := r.ValidateThresholdVsLimit(ctx, int64(*limits.DownloadThreshold), int64(*limits.DownloadDailyLimit), "download threshold"); err != nil {
				return nil, err
			}
		}
		if limits.StorageThreshold != nil && limits.StorageLimit != nil {
			if err := r.ValidateThresholdVsLimit(ctx, int64(*limits.StorageThreshold), int64(*limits.StorageLimit), "storage threshold"); err != nil {
				return nil, err
			}
		}
	}

	return limits, nil
}

// ValidateThresholdVsLimit ensures threshold cannot exceed limit
func (r *DefaultLimitResolver) ValidateThresholdVsLimit(ctx context.Context, thresholdValue, limitValue int64, thresholdType string) error {
	ctx, span := core.TraceMethod(ctx, "DefaultLimitResolver.ValidateThresholdVsLimit")
	defer span.End()

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
func (r *DefaultLimitResolver) ValidateThresholdValue(ctx context.Context, thresholdValue int64, thresholdType string) error {
	ctx, span := core.TraceMethod(ctx, "DefaultLimitResolver.ValidateThresholdValue")
	defer span.End()

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
func (r *DefaultLimitResolver) ApplyLimit(ctx context.Context, dest **uint64, source int64, limitName string, options ...pluginCore.LimitOption) error {
	ctx, span := core.TraceMethod(ctx, "DefaultLimitResolver.ApplyLimit")
	defer span.End()

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
		convertedValue = nil // -1 and 0 (when TreatZeroAsNil is true) are treated as unlimited (nil)
	} else {
		converted := uint64(source)
		convertedValue = &converted
	}

	*dest = convertedValue
	return nil
}

// applyPlanLimits applies limits from a quota plan
// 0 values are treated as disabled limits (not as unlimited)
func (r *DefaultLimitResolver) applyPlanLimits(limits *pluginCore.EffectiveLimits, plan *models.QuotaPlan) error {
	// Apply basic limits with special handling for zero values in plans
	if err := r.applyPlanLimit(&limits.StorageLimit, plan.StorageLimit, "storage limit in quota plan"); err != nil {
		return err
	}
	if err := r.applyPlanLimit(&limits.UploadDailyLimit, plan.UploadDailyLimit, "upload daily limit in quota plan"); err != nil {
		return err
	}
	if err := r.applyPlanLimit(&limits.DownloadDailyLimit, plan.DownloadDailyLimit, "download daily limit in quota plan"); err != nil {
		return err
	}
	if err := r.applyPlanLimit(&limits.UploadTotalLimit, plan.UploadTotalLimit, "upload total limit in quota plan"); err != nil {
		return err
	}
	if err := r.applyPlanLimit(&limits.DownloadTotalLimit, plan.DownloadTotalLimit, "download total limit in quota plan"); err != nil {
		return err
	}

	// Apply thresholds (for threshold policy)
	if plan.StorageThreshold != nil {
		if err := r.ValidateThresholdValue(context.Background(), *plan.StorageThreshold, "storage threshold"); err != nil {
			return err
		}
		limits.StorageThreshold = r.convertLimitValue(*plan.StorageThreshold)
		limits.HasStorageThresholdConfig = true
	}
	if plan.UploadThreshold != nil {
		if err := r.ValidateThresholdValue(context.Background(), *plan.UploadThreshold, "upload threshold"); err != nil {
			return err
		}
		limits.UploadThreshold = r.convertLimitValue(*plan.UploadThreshold)
		limits.HasUploadThresholdConfig = true
	}
	if plan.DownloadThreshold != nil {
		if err := r.ValidateThresholdValue(context.Background(), *plan.DownloadThreshold, "download threshold"); err != nil {
			return err
		}
		limits.DownloadThreshold = r.convertLimitValue(*plan.DownloadThreshold)
		limits.HasDownloadThresholdConfig = true
	}

	// Mark which limits were configured (based on whether the plan had them set, regardless of value)
	// Since these are int64 fields in the plan, we assume they were explicitly set when the plan was defined
	limits.HasStorageLimitConfig = true
	limits.HasUploadDailyLimitConfig = true
	limits.HasDownloadDailyLimitConfig = true
	limits.HasUploadTotalLimitConfig = true
	limits.HasDownloadTotalLimitConfig = true
	return nil
}

// applyPlanLimit applies a single limit from a quota plan with special handling for zero values
// For quota plans: 0 = disabled (nil), -1 = unlimited (nil), positive = actual limit
func (r *DefaultLimitResolver) applyPlanLimit(dest **uint64, source int64, limitName string) error {
	// Allow -1 (unlimited), 0 (disabled), and positive values
	if source < -1 {
		return fmt.Errorf("invalid %s: %d (must be -1, 0, or positive)", limitName, source)
	}

	// Check if the value is unreasonably large (1 PiB should be enough for most use cases)
	if source > 0 && source > int64(units.PiB) {
		return fmt.Errorf("limit value %d is unreasonably large", source)
	}

	var convertedValue *uint64
	if source == -1 || source == 0 {
		convertedValue = nil // -1 and 0 are both treated as nil for quota plans
	} else {
		converted := uint64(source)
		convertedValue = &converted
	}

	*dest = convertedValue
	return nil
}

// applyUserLimits applies user-specific limits that override plan limits
// 0 values are treated as disabled limits
func (r *DefaultLimitResolver) applyUserLimits(limits *pluginCore.EffectiveLimits, config *models.UserQuotaConfig) error {
	if config.StorageLimit != nil {
		if err := r.ApplyLimit(context.Background(), &limits.StorageLimit, *config.StorageLimit, "storage limit in user config"); err != nil {
			return err
		}
		limits.HasStorageLimitConfig = true
	}
	if config.UploadDailyLimit != nil {
		if err := r.ApplyLimit(context.Background(), &limits.UploadDailyLimit, *config.UploadDailyLimit, "upload daily limit in user config"); err != nil {
			return err
		}
		limits.HasUploadDailyLimitConfig = true
	}
	if config.DownloadDailyLimit != nil {
		if err := r.ApplyLimit(context.Background(), &limits.DownloadDailyLimit, *config.DownloadDailyLimit, "download daily limit in user config"); err != nil {
			return err
		}
		limits.HasDownloadDailyLimitConfig = true
	}
	if config.UploadTotalLimit != nil {
		if err := r.ApplyLimit(context.Background(), &limits.UploadTotalLimit, *config.UploadTotalLimit, "upload total limit in user config"); err != nil {
			return err
		}
		limits.HasUploadTotalLimitConfig = true
	}
	if config.DownloadTotalLimit != nil {
		if err := r.ApplyLimit(context.Background(), &limits.DownloadTotalLimit, *config.DownloadTotalLimit, "download total limit in user config"); err != nil {
			return err
		}
		limits.HasDownloadTotalLimitConfig = true
	}
	if config.StorageThreshold != nil {
		if err := r.ValidateThresholdValue(context.Background(), *config.StorageThreshold, "storage threshold"); err != nil {
			return err
		}
		limits.StorageThreshold = r.convertLimitValue(*config.StorageThreshold)
		limits.HasStorageThresholdConfig = true
	}
	if config.UploadThreshold != nil {
		if err := r.ValidateThresholdValue(context.Background(), *config.UploadThreshold, "upload threshold"); err != nil {
			return err
		}
		limits.UploadThreshold = r.convertLimitValue(*config.UploadThreshold)
		limits.HasUploadThresholdConfig = true
	}
	if config.DownloadThreshold != nil {
		if err := r.ValidateThresholdValue(context.Background(), *config.DownloadThreshold, "download threshold"); err != nil {
			return err
		}
		limits.DownloadThreshold = r.convertLimitValue(*config.DownloadThreshold)
		limits.HasDownloadThresholdConfig = true
	}
	return nil
}

// validateHardLimits performs additional validation for hard limits policy
func (r *DefaultLimitResolver) validateHardLimits(ctx context.Context, limits *pluginCore.EffectiveLimits, config *models.UserQuotaConfig) (*pluginCore.EffectiveLimits, error) {
	ctx, span := core.TraceMethod(ctx, "DefaultLimitResolver.validateHardLimits")
	defer span.End()

	// Check if any limits are configured
	hasLimits := limits.HasAnyLimits()

	if !hasLimits {
		// Check if there's a default plan that could provide limits
		_, err := r.quotaService.GetQuotaPlanManager().GetDefaultQuotaPlan(ctx)
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
