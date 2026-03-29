package policies

import (
	"context"
	"errors"
	"fmt"

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
		}
	}

	// Start with plan limits as base
	if plan != nil {
		if plan.IsActive != nil && !*plan.IsActive {
			return nil, fmt.Errorf("quota plan is inactive")
		}
		r.applyPlanLimits(limits, plan, policy)
		planID := uint64(plan.ID)
		limits.QuotaPlanID = &planID
		LimitResolved.WithLabelValues(LabelLimitSourcePlan).Inc()
	}

	// Override with user-specific limits
	r.applyUserLimits(limits, config)
	if limits.HasAnyLimits() {
		LimitResolved.WithLabelValues(LabelLimitSourceUser).Inc()
	}

	// Apply policy-specific validation
	if policy == models.EnforcementPolicyHardLimits {
		return r.validateHardLimits(ctx, limits)
	}

	return limits, nil
}

// applyPlanLimits applies limits from a quota plan
func (r *DefaultLimitResolver) applyPlanLimits(limits *pluginCore.EffectiveLimits, plan *models.QuotaPlan, policy models.EnforcementPolicy) {
	// Build single window from plan configuration
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(plan.WindowType.String()),
		Duration:  plan.WindowDuration,
		StartHour: plan.WindowStartHour,
		Timezone:  plan.WindowTimezone,
	}

	// Apply window-based storage limit
	if plan.StorageLimitBytes > 0 {
		limits.StorageLimitConfig = &pluginCore.Limit{
			Bytes:     plan.StorageLimitBytes,
			Window:    window,
			Priority:  0,
		}
		limits.HasStorageLimitConfig = true
	}

	// Apply window-based upload limit
	if plan.UploadLimitBytes > 0 {
		limits.UploadLimitConfig = &pluginCore.Limit{
			Bytes:     plan.UploadLimitBytes,
			Window:    window,
			Priority:  0,
		}
		limits.HasUploadLimitConfig = true
	}

	// Apply window-based download limit
	if plan.DownloadLimitBytes > 0 {
		limits.DownloadLimitConfig = &pluginCore.Limit{
			Bytes:     plan.DownloadLimitBytes,
			Window:    window,
			Priority:  0,
		}
		limits.HasDownloadLimitConfig = true
	}

	// Apply thresholds (for THRESHOLD policy)
	if policy == models.EnforcementPolicyThreshold {
		if plan.StorageThreshold != nil {
			v := uint64(*plan.StorageThreshold)
			limits.StorageThreshold = &v
			limits.HasStorageThresholdConfig = true
		}
		if plan.UploadThreshold != nil {
			v := uint64(*plan.UploadThreshold)
			limits.UploadThreshold = &v
			limits.HasUploadThresholdConfig = true
		}
		if plan.DownloadThreshold != nil {
			v := uint64(*plan.DownloadThreshold)
			limits.DownloadThreshold = &v
			limits.HasDownloadThresholdConfig = true
		}
	}
}

// applyUserLimits applies user-specific limits that override plan limits
func (r *DefaultLimitResolver) applyUserLimits(limits *pluginCore.EffectiveLimits, config *models.UserQuotaConfig) {
	// Build single window from user configuration
	// User has priority, so we always use their window if configured
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}

	// Apply window-based storage limit (user overrides plan if configured)
	if config.StorageLimitBytes > 0 {
		limits.StorageLimitConfig = &pluginCore.Limit{
			Bytes:     config.StorageLimitBytes,
			Window:    window,
			Priority:  10,
		}
		limits.HasStorageLimitConfig = true
	}

	// Apply window-based upload limit (user overrides plan if configured)
	if config.UploadLimitBytes > 0 {
		limits.UploadLimitConfig = &pluginCore.Limit{
			Bytes:     config.UploadLimitBytes,
			Window:    window,
			Priority:  10,
		}
		limits.HasUploadLimitConfig = true
	}

	// Apply window-based download limit (user overrides plan if configured)
	if config.DownloadLimitBytes > 0 {
		limits.DownloadLimitConfig = &pluginCore.Limit{
			Bytes:     config.DownloadLimitBytes,
			Window:    window,
			Priority:  10,
		}
		limits.HasDownloadLimitConfig = true
	}

	// Apply thresholds
	if config.StorageThreshold != nil {
		v := uint64(*config.StorageThreshold)
		limits.StorageThreshold = &v
		limits.HasStorageThresholdConfig = true
	}
	if config.UploadThreshold != nil {
		v := uint64(*config.UploadThreshold)
		limits.UploadThreshold = &v
		limits.HasUploadThresholdConfig = true
	}
	if config.DownloadThreshold != nil {
		v := uint64(*config.DownloadThreshold)
		limits.DownloadThreshold = &v
		limits.HasDownloadThresholdConfig = true
	}
}

// validateHardLimits performs additional validation for hard limits policy
func (r *DefaultLimitResolver) validateHardLimits(ctx context.Context, limits *pluginCore.EffectiveLimits) (*pluginCore.EffectiveLimits, error) {
	ctx, span := core.TraceMethod(ctx, "DefaultLimitResolver.validateHardLimits")
	defer span.End()

	if !limits.HasAnyLimits() {
		return nil, fmt.Errorf("no limits configured for hard limits policy")
	}

	return limits, nil
}
