package core

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// GrantManager handles grant operations
type GrantManager interface {
	// CreateAllowanceGrant creates a new allowance grant for a user
	CreateAllowanceGrant(ctx context.Context, userID uint, grant *AllowanceGrant) error

	// CreateAllowanceGrantLocked creates a new allowance grant for a user within a transaction
	CreateAllowanceGrantLocked(ctx context.Context, userID uint, grant *AllowanceGrant, tx *gorm.DB) error

	// GetActiveGrantsByType gets all active grants for a user of a specific type
	GetActiveGrantsByType(ctx context.Context, userID uint, grantType GrantType) ([]*AllowanceGrant, error)

	// GetActiveGrantsByTypeLocked gets all active grants for a user of a specific type with row-level locking
	GetActiveGrantsByTypeLocked(ctx context.Context, userID uint, grantType GrantType, tx *gorm.DB) ([]*AllowanceGrant, error)

	// GetActiveGrantsLocked gets all active grants for a user (all types) with row-level locking
	GetActiveGrantsLocked(ctx context.Context, userID uint, tx *gorm.DB) ([]*AllowanceGrant, error)

	// GetActiveGrants gets all active grants for a user (all types)
	GetActiveGrants(ctx context.Context, userID uint) ([]*AllowanceGrant, error)

	// CalculateAvailableBytes calculates total available bytes across all active grants of a type
	CalculateAvailableBytes(grants []*AllowanceGrant) uint64

	// ConsumeFromGrants consumes bytes from grants based on prioritization rules
	ConsumeFromGrants(ctx context.Context, userID uint, grantType GrantType, bytes uint64, usageDetailID uint, tx *gorm.DB) ([]*AllowanceConsumption, error)

	// DeactivateGrant deactivates a grant (doesn't delete, just marks inactive)
	DeactivateGrant(ctx context.Context, grantID uint) error

	// GetExpiringGrants gets grants expiring within a time window
	GetExpiringGrants(ctx context.Context, expiryWindow time.Duration) ([]*AllowanceGrant, error)

	// GetExpiringGrantsForUser gets grants expiring within a time window for a specific user
	GetExpiringGrantsForUser(ctx context.Context, userID uint, window time.Duration) ([]*AllowanceGrant, error)
}
