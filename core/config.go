package core

import (
	"context"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// ConfigManager handles user quota configuration resolution
type ConfigManager interface {
	ResolveEffectiveLimits(ctx context.Context, userID uint) (*EffectiveLimits, error)
	ResolveEffectiveLimitsBatch(ctx context.Context, userIDs []uint) (map[uint]*EffectiveLimits, error)
	// ResolveEffectiveLimitsBatchReadOnly is like ResolveEffectiveLimitsBatch but does NOT
	// create default configs for users lacking one. Users without existing configs are omitted
	// from the result map. Use this from read-only paths (e.g. health checks).
	ResolveEffectiveLimitsBatchReadOnly(ctx context.Context, userIDs []uint) (map[uint]*EffectiveLimits, error)
	GetUserQuotaConfig(ctx context.Context, userID uint) (*models.UserQuotaConfig, error)
	GetPolicyEnforcer(ctx context.Context, userID uint) (PolicyEnforcer, error)
	GetUserAllowanceGrants(ctx context.Context, userID uint) ([]*models.AllowanceGrant, error)
	GetUserAllowanceGrantsByType(ctx context.Context, userID uint, grantType models.GrantType) ([]*models.AllowanceGrant, error)
}
