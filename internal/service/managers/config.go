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

// GetUserQuotaConfig retrieves the quota configuration for a user
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
