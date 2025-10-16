package managers

import (
	"fmt"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	portalCore "go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ConfigManager handles the resolution of user quota configurations based on policy and plan references
type ConfigManager struct {
	ctx             portalCore.Context
	db              *gorm.DB
	logger          *portalCore.Logger
	config          *config.QuotaConfig
	limitResolver   pluginCore.LimitResolver
	planManager     pluginCore.QuotaPlanManager
	policyEnforcers map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(ctx portalCore.Context, limitResolver pluginCore.LimitResolver, planManager pluginCore.QuotaPlanManager, policyEnforcers map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer) *ConfigManager {
	quotaConfig := portalCore.GetServiceConfig[*config.QuotaConfig](ctx, pluginCore.QUOTA_SERVICE)

	return &ConfigManager{
		ctx:             ctx,
		db:              ctx.DB(),
		logger:          ctx.NamedLogger("quota.ConfigManager"),
		config:          quotaConfig,
		limitResolver:   limitResolver,
		planManager:     planManager,
		policyEnforcers: policyEnforcers,
	}
}

// ResolveEffectiveLimits resolves the effective limits for a user based on their configuration
func (cm *ConfigManager) ResolveEffectiveLimits(userID uint) (*pluginCore.EffectiveLimits, error) {
	cm.logger.Debug("Resolving effective limits for user", zap.Uint("userID", userID))

	// Get user's quota config
	cfg, err := cm.GetUserQuotaConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quota config: %w", err)
	}

	// Resolve limits using the limit resolver
	limits, err := cm.limitResolver.ResolveEffectiveLimits(cfg, pluginModels.EnforcementPolicy(cfg.EnforcementPolicy))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve effective limits: %w", err)
	}

	cm.logger.Debug("Effective limits resolved successfully", zap.Uint("userID", userID))
	return limits, nil
}

// GetUserQuotaConfig retrieves the quota configuration for a user
func (cm *ConfigManager) GetUserQuotaConfig(userID uint) (*pluginModels.UserQuotaConfig, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	cm.logger.Debug("Retrieving user quota config", zap.Uint("userID", userID))

	var config pluginModels.UserQuotaConfig
	err := cm.db.Where("user_id = ?", userID).First(&config).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			cm.logger.Debug("User quota config not found, creating default", zap.Uint("userID", userID))
			// Create default config if not found
			defaultConfig, createErr := cm.createDefaultUserQuotaConfig(userID)
			if createErr != nil {
				return nil, fmt.Errorf("failed to create default user quota config: %w", createErr)
			}

			// Save the default config to the database
			if err := cm.db.Create(defaultConfig).Error; err != nil {
				return nil, fmt.Errorf("failed to save default user quota config: %w", err)
			}

			return defaultConfig, nil
		}
		return nil, fmt.Errorf("failed to retrieve user quota config: %w", err)
	}

	return &config, nil
}

// createDefaultUserQuotaConfig creates a default quota configuration for a user
// Attempts to assign the default quota plan, falls back to system defaults if no plan exists
func (cm *ConfigManager) createDefaultUserQuotaConfig(userID uint) (*pluginModels.UserQuotaConfig, error) {
	// Set default enforcement policy from config
	enforcementPolicy := pluginModels.EnforcementPolicyHardLimits
	if cm.config != nil && cm.config.DefaultEnforcementPolicy != "" {
		enforcementPolicy = pluginModels.EnforcementPolicy(cm.config.DefaultEnforcementPolicy)
	}

	// Try to get the default quota plan
	defaultPlan, err := cm.planManager.GetDefaultQuotaPlan()
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
func (cm *ConfigManager) GetPolicyEnforcer(userID uint) (pluginCore.PolicyEnforcer, error) {
	cm.logger.Debug("Getting policy enforcer for user", zap.Uint("userID", userID))

	cfg, err := cm.GetUserQuotaConfig(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user quota config: %w", err)
	}

	enforcer, exists := cm.policyEnforcers[cfg.EnforcementPolicy]
	if !exists {
		return nil, fmt.Errorf("no policy enforcer found for policy: %s", cfg.EnforcementPolicy)
	}

	cm.logger.Debug("Policy enforcer retrieved", zap.Uint("userID", userID), zap.String("policy", string(cfg.EnforcementPolicy)))
	return enforcer, nil
}

// GetUserAllowanceGrants gets all active allowance grants for a user (for ALLOWANCE policy)
func (cm *ConfigManager) GetUserAllowanceGrants(userID uint) ([]*pluginModels.AllowanceGrant, error) {
	cm.logger.Debug("Getting user allowance grants", zap.Uint("userID", userID))

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	var grants []*pluginModels.AllowanceGrant
	err := cm.db.Where("user_id = ? AND is_active = true AND (expiry_date IS NULL OR expiry_date > ?)",
		userID, time.Now().UTC()).Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user allowance grants: %w", err)
	}

	cm.logger.Debug("User allowance grants retrieved", zap.Uint("userID", userID), zap.Int("count", len(grants)))
	return grants, nil
}

// GetUserAllowanceGrantsByType gets active grants by type for a user
func (cm *ConfigManager) GetUserAllowanceGrantsByType(userID uint, grantType pluginModels.GrantType) ([]*pluginModels.AllowanceGrant, error) {
	cm.logger.Debug("Getting user allowance grants by type", zap.Uint("userID", userID), zap.String("type", string(grantType)))

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	var grants []*pluginModels.AllowanceGrant
	err := cm.db.Where("user_id = ? AND type = ? AND is_active = true AND (expiry_date IS NULL OR expiry_date > ?)",
		userID, grantType, time.Now().UTC()).Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve user allowance grants by type: %w", err)
	}

	cm.logger.Debug("User allowance grants by type retrieved", zap.Uint("userID", userID), zap.String("type", string(grantType)), zap.Int("count", len(grants)))
	return grants, nil
}

// GetConfigManager returns the ConfigManager instance itself
func (cm *ConfigManager) GetConfigManager() *ConfigManager {
	return cm
}
