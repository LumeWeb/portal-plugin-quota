package core

import (
	"context"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// LimitResolver provides unified limit resolution functionality across all quota policies
type LimitResolver interface {
	// ResolveEffectiveLimits resolves the effective limits for a user based on their configuration
	// This method consolidates the logic from getEffectiveLimits() and resolveEffectiveLimits()
	ResolveEffectiveLimits(ctx context.Context, config *models.UserQuotaConfig, policy models.EnforcementPolicy) (*EffectiveLimits, error)
}
