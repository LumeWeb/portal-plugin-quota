package core

import (
	"time"

	"gorm.io/gorm"
)

// GrantManager handles grant operations
type GrantManager interface {
	// CreateAllowanceGrant creates a new allowance grant for a user
	CreateAllowanceGrant(userID uint, grant *AllowanceGrant) error

	// GetActiveGrantsByType gets all active grants for a user of a specific type
	GetActiveGrantsByType(userID uint, grantType GrantType) ([]*AllowanceGrant, error)

	// GetActiveGrantsByTypeLocked gets all active grants for a user of a specific type with row-level locking
	GetActiveGrantsByTypeLocked(userID uint, grantType GrantType, tx *gorm.DB) ([]*AllowanceGrant, error)

	// GetActiveGrantsLocked gets all active grants for a user (all types) with row-level locking
	GetActiveGrantsLocked(userID uint) ([]*AllowanceGrant, error)

	// GetActiveGrants gets all active grants for a user (all types)
	GetActiveGrants(userID uint) ([]*AllowanceGrant, error)

	// CalculateAvailableBytes calculates total available bytes across all active grants of a type
	CalculateAvailableBytes(grants []*AllowanceGrant) uint64

	// ConsumeFromGrants consumes bytes from grants based on prioritization rules
	ConsumeFromGrants(userID uint, grantType GrantType, bytes uint64, usageDetailID uint) ([]*AllowanceConsumption, error)

	// DeactivateGrant deactivates a grant (doesn't delete, just marks inactive)
	DeactivateGrant(grantID uint) error

	// GetExpiringGrants gets grants expiring within a time window
	GetExpiringGrants(expiryWindow time.Duration) ([]*AllowanceGrant, error)

	// GetExpiringGrantsForUser gets grants expiring within a time window for a specific user
	GetExpiringGrantsForUser(userID uint, window time.Duration) ([]*AllowanceGrant, error)
}
