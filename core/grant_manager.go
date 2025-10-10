package core

import (
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// GrantManager handles grant operations
type GrantManager interface {
	// CreateAllowanceGrant creates a new allowance grant for a user
	CreateAllowanceGrant(userID uint, grant *models.AllowanceGrant) error

	// GetActiveGrantsByType gets all active grants for a user of a specific type
	GetActiveGrantsByType(userID uint, grantType models.GrantType) ([]*models.AllowanceGrant, error)

	// GetActiveGrants gets all active grants for a user (all types)
	GetActiveGrants(userID uint) ([]*models.AllowanceGrant, error)

	// CalculateAvailableBytes calculates total available bytes across all active grants of a type
	CalculateAvailableBytes(grants []*models.AllowanceGrant) uint64

	// ConsumeFromGrants consumes bytes from grants based on prioritization rules
	ConsumeFromGrants(userID uint, grantType models.GrantType, bytes uint64) ([]*models.AllowanceConsumption, error)

	// DeactivateGrant deactivates a grant (doesn't delete, just marks inactive)
	DeactivateGrant(grantID uint) error

	// GetExpiringGrants gets grants expiring within a time window
	GetExpiringGrants(expiryWindow time.Duration) ([]*models.AllowanceGrant, error)

	// GetExpiringGrantsForUser gets grants expiring within a time window for a specific user
	GetExpiringGrantsForUser(userID uint, window time.Duration) ([]*models.AllowanceGrant, error)
}
