package core

import (
	"context"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// ConfigManager handles user quota configuration resolution
type ConfigManager interface {
	ResolveEffectiveLimits(ctx context.Context, userID uint) (*EffectiveLimits, error)
	GetUserQuotaConfig(ctx context.Context, userID uint) (*models.UserQuotaConfig, error)
	GetPolicyEnforcer(ctx context.Context, userID uint) (PolicyEnforcer, error)
	GetUserAllowanceGrants(ctx context.Context, userID uint) ([]*models.AllowanceGrant, error)
	GetUserAllowanceGrantsByType(ctx context.Context, userID uint, grantType models.GrantType) ([]*models.AllowanceGrant, error)
}
