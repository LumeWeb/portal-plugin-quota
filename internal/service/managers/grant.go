package managers

import (
	"fmt"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GrantManagerDefault handles grant operations
type GrantManagerDefault struct {
	ctx    core.Context
	db     *gorm.DB
	logger *core.Logger
}

// NewGrantManager creates a new grant manager
func NewGrantManager(ctx core.Context) pluginCore.GrantManager {
	return &GrantManagerDefault{
		ctx:    ctx,
		db:     ctx.DB(),
		logger: ctx.NamedLogger("quota.GrantManager"),
	}
}

// CreateAllowanceGrant creates a new allowance grant for a user
func (gm *GrantManagerDefault) CreateAllowanceGrant(userID uint, grant *pluginModels.AllowanceGrant) error {
	return gm.CreateAllowanceGrantLocked(userID, grant, nil)
}

// CreateAllowanceGrantLocked creates a new allowance grant for a user within a transaction
func (gm *GrantManagerDefault) CreateAllowanceGrantLocked(userID uint, grant *pluginModels.AllowanceGrant, tx *gorm.DB) error {
	if userID == 0 {
		return pluginModels.ErrInvalidUserID
	}

	if grant == nil {
		return fmt.Errorf("grant cannot be nil")
	}

	// Set the user ID on the grant
	grant.UserID = userID

	// Set IsActive to true by default if not specified
	if !grant.IsActive {
		grant.IsActive = true
	}

	// Use provided transaction or default database connection
	db := gm.db
	if tx != nil {
		db = tx
	}

	// Create the grant in the database
	if err := db.Create(grant).Error; err != nil {
		return fmt.Errorf("failed to create allowance grant: %w", err)
	}

	return nil
}

// GetActiveGrantsByType gets all active grants for a user of a specific type
func (gm *GrantManagerDefault) GetActiveGrantsByType(userID uint, grantType pluginModels.GrantType) ([]*pluginModels.AllowanceGrant, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	if !grantType.IsValid() {
		return nil, pluginModels.ErrInvalidGrantType
	}

	// Use provided transaction if available, otherwise use default database connection
	db := gm.db

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority
	query := db.Where("user_id = ? AND type = ? AND is_active = true", userID, grantType)

	// Filter out expired grants in SQL
	query = query.Where("(expiry_date IS NULL OR expiry_date > ?)", now)

	// Order by source priority using CASE statement
	query = query.Order(gm.getSourcePriorityOrderClause() + " DESC")

	err := query.Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active grants by type: %w", err)
	}

	return grants, nil
}

// GetActiveGrantsByTypeLocked gets all active grants for a user of a specific type with row-level locking
func (gm *GrantManagerDefault) GetActiveGrantsByTypeLocked(userID uint, grantType pluginModels.GrantType, tx *gorm.DB) ([]*pluginModels.AllowanceGrant, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	if !grantType.IsValid() {
		return nil, pluginModels.ErrInvalidGrantType
	}

	// Use provided transaction if available, otherwise use default database connection
	db := gm.db
	if tx != nil {
		db = tx
	}

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority and row-level locking
	query := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND type = ? AND is_active = true", userID, grantType)

	// Filter out expired grants in SQL
	query = query.Where("(expiry_date IS NULL OR expiry_date > ?)", now)

	// Order by source priority using CASE statement
	query = query.Order(gm.getSourcePriorityOrderClause() + " DESC")

	err := query.Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active grants by type: %w", err)
	}

	return grants, nil
}

// GetActiveGrants gets all active grants for a user (all types)
func (gm *GrantManagerDefault) GetActiveGrants(userID uint) ([]*pluginModels.AllowanceGrant, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority
	query := gm.db.Where("user_id = ? AND is_active = true", userID)

	// Filter out expired grants in SQL
	query = query.Where("(expiry_date IS NULL OR expiry_date > ?)", now)

	// Order by source priority using CASE statement
	query = query.Order(gm.getSourcePriorityOrderClause() + " DESC")

	err := query.Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active grants: %w", err)
	}

	return grants, nil
}

// GetActiveGrantsLocked gets all active grants for a user (all types) with row-level locking
func (gm *GrantManagerDefault) GetActiveGrantsLocked(userID uint, tx *gorm.DB) ([]*pluginModels.AllowanceGrant, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	// Use provided transaction if available, otherwise use default database connection
	db := gm.db
	if tx != nil {
		db = tx
	}

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority and row-level locking
	query := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND is_active = true", userID)

	// Filter out expired grants in SQL
	query = query.Where("(expiry_date IS NULL OR expiry_date > ?)", now)

	// Order by source priority using CASE statement
	query = query.Order(gm.getSourcePriorityOrderClause() + " DESC")

	err := query.Find(&grants).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active grants: %w", err)
	}

	return grants, nil
}

// CalculateAvailableBytes calculates total available bytes across all active grants of a type
func (gm *GrantManagerDefault) CalculateAvailableBytes(grants []*pluginModels.AllowanceGrant) uint64 {
	return lo.SumBy(grants, func(grant *pluginModels.AllowanceGrant) uint64 {
		return grant.BytesRemaining
	})
}

// ConsumeFromGrants consumes bytes from grants based on prioritization rules
func (gm *GrantManagerDefault) ConsumeFromGrants(userID uint, grantType pluginModels.GrantType, bytes uint64, usageDetailID uint, tx *gorm.DB) ([]*pluginModels.AllowanceConsumption, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	if !grantType.IsValid() {
		return nil, pluginModels.ErrInvalidGrantType
	}

	if bytes == 0 {
		return []*pluginModels.AllowanceConsumption{}, nil
	}

	var consumptions []*pluginModels.AllowanceConsumption

	// Use provided transaction or create a new one
	db := gm.db
	if tx != nil {
		db = tx
	}

	// Start a transaction if none was provided
	if tx == nil {
		err := db.Transaction(func(tx *gorm.DB) error {
			return gm.consumeFromGrantsInTransaction(tx, userID, grantType, bytes, usageDetailID, &consumptions)
		})
		if err != nil {
			return nil, err
		}
	} else {
		err := gm.consumeFromGrantsInTransaction(tx, userID, grantType, bytes, usageDetailID, &consumptions)
		if err != nil {
			return nil, err
		}
	}

	return consumptions, nil
}

// consumeFromGrantsInTransaction performs the actual grant consumption within a transaction
func (gm *GrantManagerDefault) consumeFromGrantsInTransaction(tx *gorm.DB, userID uint, grantType pluginModels.GrantType, bytes uint64, usageDetailID uint, consumptions *[]*pluginModels.AllowanceConsumption) error {
	// Get active grants for user and type with row-level locks
	grants, err := gm.GetActiveGrantsByTypeLocked(userID, grantType, tx)
	if err != nil {
		return fmt.Errorf("failed to get active grants: %w", err)
	}

	// Check if we have enough total allowance
	totalAvailable := gm.CalculateAvailableBytes(grants)
	if totalAvailable < bytes {
		return pluginModels.ErrInsufficientAllowance
	}

	// Consume bytes from grants in priority order
	remainingBytes := bytes
	for _, grant := range grants {
		if remainingBytes <= 0 {
			break
		}

		// Calculate how much we can consume from this grant
		consumeAmount := remainingBytes
		if consumeAmount > grant.BytesRemaining {
			consumeAmount = grant.BytesRemaining
		}

		// Skip creating consumption records for 0 byte consumption
		if consumeAmount == 0 {
			continue
		}

		// Create consumption record
		consumption := &pluginModels.AllowanceConsumption{
			GrantID:         grant.ID,
			UsageDetailID:   usageDetailID,
			BytesConsumed:   consumeAmount,
			ConsumptionDate: time.Now().UTC(),
		}
		*consumptions = append(*consumptions, consumption)

		// Update grant atomically using a single SQL update with WHERE guard
		// This prevents negative bytes_remaining values and detects concurrent modifications
		result := tx.Model(&pluginModels.AllowanceGrant{}).
			Where("id = ? AND bytes_remaining >= ?", grant.ID, consumeAmount).
			UpdateColumns(map[string]interface{}{
				"bytes_used":      gorm.Expr("bytes_used + ?", consumeAmount),
				"bytes_remaining": gorm.Expr("bytes_remaining - ?", consumeAmount),
				"updated_at":      time.Now().UTC(),
			})

		if result.Error != nil {
			return fmt.Errorf("failed to update grant: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return fmt.Errorf("grant %d was concurrently modified", grant.ID)
		}

		// Save consumption record
		if err := tx.Create(consumption).Error; err != nil {
			return fmt.Errorf("failed to create consumption record: %w", err)
		}

		// Reduce remaining bytes
		remainingBytes -= consumeAmount
	}

	return nil
}

// getSourcePriorityOrderClause generates a SQL CASE statement for ordering by source priority
func (gm *GrantManagerDefault) getSourcePriorityOrderClause() string {
	return fmt.Sprintf(`CASE 
		WHEN source = '%s' THEN %d
		WHEN source = '%s' THEN %d
		WHEN source = '%s' THEN %d
		WHEN source = '%s' THEN %d
		ELSE 0
	END`,
		pluginModels.GrantSourcePAYGAddon, pluginModels.GrantSourcePAYGAddon.GetGrantPriority(),
		pluginModels.GrantSourcePromo, pluginModels.GrantSourcePromo.GetGrantPriority(),
		pluginModels.GrantSourceBonus, pluginModels.GrantSourceBonus.GetGrantPriority(),
		pluginModels.GrantSourceSubscription, pluginModels.GrantSourceSubscription.GetGrantPriority())
}

// DeactivateGrant deactivates a grant (doesn't delete, just marks inactive)
func (gm *GrantManagerDefault) DeactivateGrant(grantID uint) error {
	if grantID == 0 {
		return pluginModels.ErrInvalidGrantID
	}

	// Update the grant directly using raw SQL to bypass model validation
	result := gm.db.Model(&pluginModels.AllowanceGrant{}).
		Where("id = ?", grantID).
		UpdateColumns(map[string]interface{}{"is_active": false, "updated_at": time.Now().UTC()})

	if result.Error != nil {
		return fmt.Errorf("failed to deactivate grant: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("grant not found: %d", grantID)
	}

	return nil
}

// GetExpiringGrants gets grants expiring within a time window
func (gm *GrantManagerDefault) GetExpiringGrants(expiryWindow time.Duration) ([]*pluginModels.AllowanceGrant, error) {
	now := time.Now().UTC()
	cutoff := now.Add(expiryWindow)

	var grants []*pluginModels.AllowanceGrant
	err := gm.db.Where("is_active = true AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > ?",
		cutoff, now).
		Order("expiry_date ASC").
		Find(&grants).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expiring grants: %w", err)
	}

	return grants, nil
}

// GetExpiringGrantsForUser gets grants expiring within a time window for a specific user
func (gm *GrantManagerDefault) GetExpiringGrantsForUser(userID uint, window time.Duration) ([]*pluginModels.AllowanceGrant, error) {
	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	now := time.Now().UTC()
	cutoff := now.Add(window)

	var grants []*pluginModels.AllowanceGrant
	err := gm.db.Where("user_id = ? AND is_active = true AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > ?",
		userID, cutoff, now).
		Order("expiry_date ASC").
		Find(&grants).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expiring grants for user: %w", err)
	}

	return grants, nil
}
