package core

import "go.lumeweb.com/portal-plugin-quota/internal/db/models"

// ConfigManager handles user quota configuration resolution
type ConfigManager interface {
	ResolveEffectiveLimits(userID uint) (*EffectiveLimits, error)
	GetUserQuotaConfig(userID uint) (*models.UserQuotaConfig, error)
	GetPolicyEnforcer(userID uint) (PolicyEnforcer, error)
	GetUserAllowanceGrants(userID uint) ([]*models.AllowanceGrant, error)
	GetUserAllowanceGrantsByType(userID uint, grantType models.GrantType) ([]*models.AllowanceGrant, error)
}
