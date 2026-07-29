package managers

import (
	"context"
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ConfigManager handles the resolution of user quota configurations based on policy and plan references
type ConfigManager struct {
	*core.BaseComponent
	config          *config.QuotaConfig
	limitResolver   pluginCore.LimitResolver
	planManager     pluginCore.QuotaPlanManager
	policyEnforcers map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(ctx core.Context, limitResolver pluginCore.LimitResolver, planManager pluginCore.QuotaPlanManager, policyEnforcers map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer) *ConfigManager {
	quotaConfig := core.GetServiceConfig[*config.QuotaConfig](ctx, pluginCore.QUOTA_SERVICE)

	return &ConfigManager{
		BaseComponent:   core.NewBaseComponent(ctx),
		config:          quotaConfig,
		limitResolver:   limitResolver,
		planManager:     planManager,
		policyEnforcers: policyEnforcers,
	}
}

// ResolveEffectiveLimits resolves the effective limits for a user based on their configuration
func (cm *ConfigManager) ResolveEffectiveLimits(ctx context.Context, userID uint) (*pluginCore.EffectiveLimits, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.ResolveEffectiveLimits")
	defer span.End()

	cm.Logger().Debug("Resolving effective limits for user", zap.Uint("userID", userID))

	// Get user's quota config
	cfg, err := cm.GetUserQuotaConfig(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quota config: %w", err)
	}

	// Resolve limits using the limit resolver
	limits, err := cm.limitResolver.ResolveEffectiveLimits(ctx, cfg, pluginModels.EnforcementPolicy(cfg.EnforcementPolicy))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve effective limits: %w", err)
	}

	cm.Logger().Debug("Effective limits resolved successfully", zap.Uint("userID", userID))
	return limits, nil
}

// ResolveEffectiveLimitsBatch resolves effective limits for multiple users in a single pass.
// Returns a map of userID → *EffectiveLimits. Users that fail to resolve are omitted from the map.
func (cm *ConfigManager) ResolveEffectiveLimitsBatch(ctx context.Context, userIDs []uint) (map[uint]*pluginCore.EffectiveLimits, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.ResolveEffectiveLimitsBatch")
	defer span.End()

	if len(userIDs) == 0 {
		return map[uint]*pluginCore.EffectiveLimits{}, nil
	}

	// Fetch all user quota configs in a single query
	var configs []*pluginModels.UserQuotaConfig
	err := cm.DB().WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&configs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch user quota configs: %w", err)
	}

	// Build a lookup of existing configs
	configMap := make(map[uint]*pluginModels.UserQuotaConfig, len(configs))
	for _, cfg := range configs {
		configMap[cfg.UserID] = cfg
	}

	// Batch-create default configs for missing users
	var missingIDs []uint
	for _, userID := range userIDs {
		if _, exists := configMap[userID]; !exists {
			missingIDs = append(missingIDs, userID)
		}
	}
	if len(missingIDs) > 0 {
		// Build default configs using the same logic as createDefaultUserQuotaConfig,
		// but resolve the default plan once for all users.
		defaultPlan, planErr := cm.planManager.GetDefaultQuotaPlan(ctx)

		var defaultPlanID *uint64
		if planErr == nil && defaultPlan != nil {
			pid := uint64(defaultPlan.ID)
			defaultPlanID = &pid
		}

		enforcementPolicy := pluginModels.EnforcementPolicyHardLimits
		if cm.config != nil && cm.config.DefaultEnforcementPolicy != "" {
			enforcementPolicy = pluginModels.EnforcementPolicy(cm.config.DefaultEnforcementPolicy)
		}

		defaults := make([]*pluginModels.UserQuotaConfig, 0, len(missingIDs))
		for _, userID := range missingIDs {
			defaults = append(defaults, &pluginModels.UserQuotaConfig{
					UserID:            userID,
					EnforcementPolicy: enforcementPolicy,
					QuotaPlanID:       defaultPlanID,
				})
		}
		if createErr := cm.DB().WithContext(ctx).Create(&defaults).Error; createErr != nil {
			// Best-effort: if batch create fails (e.g. race), fall back to per-user resolution
			cm.Logger().Warn("batch default config creation failed, falling back to per-user",
				zap.Error(createErr))
		} else {
			// Re-fetch all configs now that defaults exist
			configs = nil
			if refetchErr := cm.DB().WithContext(ctx).Where("user_id IN ?", userIDs).Find(&configs).Error; refetchErr != nil {
				return nil, fmt.Errorf("failed to refetch user quota configs: %w", refetchErr)
			}
			configMap = make(map[uint]*pluginModels.UserQuotaConfig, len(configs))
			for _, cfg := range configs {
				configMap[cfg.UserID] = cfg
			}
		}
	}

	// Resolve limits in-memory, log+continue on per-user failures
	result := make(map[uint]*pluginCore.EffectiveLimits, len(userIDs))
	for _, userID := range userIDs {
		cfg, exists := configMap[userID]
		if !exists {
			// Config still missing after batch create — try single-user path as last resort
			limits, err := cm.ResolveEffectiveLimits(ctx, userID)
			if err != nil {
				cm.Logger().Warn("omitting user from batch limits: single-user resolution failed",
					zap.Uint("userID", userID), zap.Error(err))
				continue
			}
			result[userID] = limits
			continue
		}

		limits, err := cm.limitResolver.ResolveEffectiveLimits(ctx, cfg, pluginModels.EnforcementPolicy(cfg.EnforcementPolicy))
		if err != nil {
			cm.Logger().Warn("omitting user from batch limits: resolver failed",
				zap.Uint("userID", userID), zap.Error(err))
			continue
		}
		result[userID] = limits
	}

	return result, nil
}

// ResolveEffectiveLimitsBatchReadOnly resolves effective limits for multiple users
// without creating default configs for users that lack one. Users without existing
// configs are omitted from the result map. Suitable for read-only paths like health checks.
func (cm *ConfigManager) ResolveEffectiveLimitsBatchReadOnly(ctx context.Context, userIDs []uint) (map[uint]*pluginCore.EffectiveLimits, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.ResolveEffectiveLimitsBatchReadOnly")
	defer span.End()

	if len(userIDs) == 0 {
		return map[uint]*pluginCore.EffectiveLimits{}, nil
	}

	// Fetch all user quota configs in a single query (no creation)
	var configs []*pluginModels.UserQuotaConfig
	err := cm.DB().WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&configs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to batch fetch user quota configs: %w", err)
	}

	configMap := make(map[uint]*pluginModels.UserQuotaConfig, len(configs))
	for _, cfg := range configs {
		configMap[cfg.UserID] = cfg
	}

	// Resolve limits in-memory, log+continue on per-user failures
	result := make(map[uint]*pluginCore.EffectiveLimits, len(configs))
	for _, userID := range userIDs {
		cfg, exists := configMap[userID]
		if !exists {
			cm.Logger().Debug("omitting user from read-only batch limits: no config",
				zap.Uint("userID", userID))
			continue
		}

		limits, err := cm.limitResolver.ResolveEffectiveLimits(ctx, cfg, pluginModels.EnforcementPolicy(cfg.EnforcementPolicy))
		if err != nil {
			cm.Logger().Warn("omitting user from read-only batch limits: resolver failed",
				zap.Uint("userID", userID), zap.Error(err))
			continue
		}
		result[userID] = limits
	}

	return result, nil
}

// GetExcludedFromHealthReports returns the set of userIDs excluded from CID pin
// health aggregates. A user is excluded if either their UserQuotaConfig has
// ExcludedFromHealthReports=true (per-user override) OR their assigned QuotaPlan
// has ExcludedFromHealthReports=true (plan-level). Users without a config or plan
// are not excluded.
func (cm *ConfigManager) GetExcludedFromHealthReports(ctx context.Context, userIDs []uint) (map[uint]bool, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.GetExcludedFromHealthReports")
	defer span.End()

	excluded := make(map[uint]bool)
	if len(userIDs) == 0 {
		return excluded, nil
	}

	// Fetch user configs — check per-user flag and collect plan IDs
	var configs []*pluginModels.UserQuotaConfig
	err := cm.DB().WithContext(ctx).
		Where("user_id IN ?", userIDs).
		Find(&configs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user configs for health exclusion: %w", err)
	}

	// Collect unique non-nil plan IDs (only for users not already excluded per-user)
	planIDs := make(map[uint64]bool)
	userToPlan := make(map[uint]uint64)
	for _, cfg := range configs {
		if cfg.ExcludedFromHealthReports {
			excluded[cfg.UserID] = true
			continue
		}
		if cfg.QuotaPlanID != nil {
			planIDs[*cfg.QuotaPlanID] = true
			userToPlan[cfg.UserID] = *cfg.QuotaPlanID
		}
	}

	if len(planIDs) == 0 {
		return excluded, nil
	}

	// Batch-fetch plans with ExcludedFromHealthReports=true
	ids := make([]uint64, 0, len(planIDs))
	for id := range planIDs {
		ids = append(ids, id)
	}

	var excludedPlans []pluginModels.QuotaPlan
	err = cm.DB().WithContext(ctx).
		Where("id IN ? AND excluded_from_health_reports = ?", ids, true).
		Find(&excludedPlans).Error
	if err != nil {
		return nil, fmt.Errorf("failed to fetch excluded plans: %w", err)
	}

	excludedPlanIDs := make(map[uint64]bool, len(excludedPlans))
	for _, p := range excludedPlans {
		excludedPlanIDs[uint64(p.ID)] = true
	}

	// Map remaining users to excluded status via plan
	for userID, planID := range userToPlan {
		if excludedPlanIDs[planID] {
			excluded[userID] = true
		}
	}

	return excluded, nil
}

func (cm *ConfigManager) GetUserQuotaConfig(ctx context.Context, userID uint) (*pluginModels.UserQuotaConfig, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.GetUserQuotaConfig")
	defer span.End()

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	cm.Logger().Debug("Retrieving user quota config", zap.Uint("userID", userID))

	// Create default config if not found
	defaultConfig, createErr := cm.createDefaultUserQuotaConfig(ctx, userID)
	if createErr != nil {
		return nil, fmt.Errorf("failed to create default user quota config: %w", createErr)
	}

	// Use FirstOrCreate to atomically handle the get-or-create logic.
	// This prevents a race condition where two concurrent requests for a new user
	// both try to create the config, causing one to fail.
	// Use Attrs to set defaults on create without adding them to the WHERE clause,
	// so that an existing config with different field values is still found.
	err := db.RetryableTransaction(ctx, cm.DB(), func(tx *gorm.DB) *gorm.DB {
		return tx.Where("user_id = ?", userID).Attrs(defaultConfig).FirstOrCreate(defaultConfig)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find or create default user quota config: %w", err)
	}

	return defaultConfig, nil
}

// createDefaultUserQuotaConfig creates a default quota configuration for a user
// Attempts to assign the default quota plan, falls back to system defaults if no plan exists
func (cm *ConfigManager) createDefaultUserQuotaConfig(ctx context.Context, userID uint) (*pluginModels.UserQuotaConfig, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.createDefaultUserQuotaConfig")
	defer span.End()

	// Set default enforcement policy from config
	enforcementPolicy := pluginModels.EnforcementPolicyHardLimits
	if cm.config != nil && cm.config.DefaultEnforcementPolicy != "" {
		enforcementPolicy = pluginModels.EnforcementPolicy(cm.config.DefaultEnforcementPolicy)
	}

	// Try to get the default quota plan
	defaultPlan, err := cm.planManager.GetDefaultQuotaPlan(ctx)
	if err != nil {
		// No default plan exists, create config with system defaults
		return &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: enforcementPolicy,
		}, nil
	}

	// Default plan exists, assign user to it
	planID := uint64(defaultPlan.ID)
	return &pluginModels.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: enforcementPolicy,
		QuotaPlanID:       &planID,
	}, nil
}

// GetPolicyEnforcer gets the appropriate policy enforcer for a user
func (cm *ConfigManager) GetPolicyEnforcer(ctx context.Context, userID uint) (pluginCore.PolicyEnforcer, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.GetPolicyEnforcer")
	defer span.End()

	cm.Logger().Debug("Getting policy enforcer for user", zap.Uint("userID", userID))

	cfg, err := cm.GetUserQuotaConfig(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quota config: %w", err)
	}

	enforcer, exists := cm.policyEnforcers[cfg.EnforcementPolicy]
	if !exists {
		return nil, fmt.Errorf("no policy enforcer found for policy: %s", cfg.EnforcementPolicy)
	}

	cm.Logger().Debug("Policy enforcer retrieved", zap.Uint("userID", userID), zap.String("policy", string(cfg.EnforcementPolicy)))
	return enforcer, nil
}

// GetUserAllowanceGrants gets all active allowance grants for a user (for ALLOWANCE policy)
func (cm *ConfigManager) GetUserAllowanceGrants(ctx context.Context, userID uint) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.GetUserAllowanceGrants")
	defer span.End()

	cm.Logger().Debug("Getting user allowance grants", zap.Uint("userID", userID))

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	var grants []*pluginModels.AllowanceGrant
	err := cm.DB().Where("user_id = ? AND is_active = true AND (expiry_date IS NULL OR expiry_date > ?)",
		userID, time.Now().UTC()).Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user allowance grants: %w", err)
	}

	cm.Logger().Debug("User allowance grants retrieved", zap.Uint("userID", userID), zap.Int("count", len(grants)))
	return grants, nil
}

// GetUserAllowanceGrantsByType gets active grants by type for a user
func (cm *ConfigManager) GetUserAllowanceGrantsByType(ctx context.Context, userID uint, grantType pluginModels.GrantType) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "ConfigManager.GetUserAllowanceGrantsByType")
	defer span.End()

	cm.Logger().Debug("Getting user allowance grants by type", zap.Uint("userID", userID), zap.String("type", string(grantType)))

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	var grants []*pluginModels.AllowanceGrant
	err := cm.DB().Where("user_id = ? AND type = ? AND is_active = true AND (expiry_date IS NULL OR expiry_date > ?)",
		userID, grantType, time.Now().UTC()).Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user allowance grants by type: %w", err)
	}

	cm.Logger().Debug("User allowance grants by type retrieved", zap.Uint("userID", userID), zap.String("type", string(grantType)), zap.Int("count", len(grants)))
	return grants, nil
}
