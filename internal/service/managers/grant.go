package managers

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/portal/db"
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
		logger: ctx.Logger(),
	}
}

// CreateAllowanceGrant creates a new allowance grant for a user
func (gm *GrantManagerDefault) CreateAllowanceGrant(ctx context.Context, userID uint, grant *pluginModels.AllowanceGrant) error {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.CreateAllowanceGrant")
	defer span.End()

	return db.RetryableTransaction(ctx, gm.db, func(tx *gorm.DB) *gorm.DB {
		if err := gm.CreateAllowanceGrantLocked(ctx, userID, grant, tx); err != nil {
			_ = tx.AddError(err)
		}
		return tx
	})
}

// CreateAllowanceGrantLocked creates a new allowance grant for a user within a transaction
func (gm *GrantManagerDefault) CreateAllowanceGrantLocked(ctx context.Context, userID uint, grant *pluginModels.AllowanceGrant, tx *gorm.DB) error {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.CreateAllowanceGrantLocked")
	defer span.End()

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
	dbConn := gm.db
	if tx != nil {
		dbConn = tx
	}

	// Create the grant in the database
	if err := dbConn.WithContext(ctx).Create(grant).Error; err != nil {
		return fmt.Errorf("failed to create allowance grant: %w", err)
	}

	return nil
}

// GetActiveGrantsByType gets all active grants for a user of a specific type
func (gm *GrantManagerDefault) GetActiveGrantsByType(ctx context.Context, userID uint, grantType pluginModels.GrantType) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.GetActiveGrantsByType")
	defer span.End()

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	if !grantType.IsValid() {
		return nil, pluginModels.ErrInvalidGrantType
	}

	// Use provided transaction if available, otherwise use default database connection
	dbConn := gm.db

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority
	query := dbConn.WithContext(ctx).Where("user_id = ? AND type = ? AND is_active = true", userID, grantType)

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
func (gm *GrantManagerDefault) GetActiveGrantsByTypeLocked(ctx context.Context, userID uint, grantType pluginModels.GrantType, tx *gorm.DB) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.GetActiveGrantsByTypeLocked")
	defer span.End()

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	if !grantType.IsValid() {
		return nil, pluginModels.ErrInvalidGrantType
	}

	// Use provided transaction if available, otherwise use default database connection
	dbConn := gm.db
	if tx != nil {
		dbConn = tx
	}

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority and row-level locking
	query := dbConn.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND type = ? AND is_active = true", userID, grantType)

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
func (gm *GrantManagerDefault) GetActiveGrants(ctx context.Context, userID uint) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.GetActiveGrants")
	defer span.End()

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority
	query := gm.db.WithContext(ctx).Where("user_id = ? AND is_active = true", userID)

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
func (gm *GrantManagerDefault) GetActiveGrantsLocked(ctx context.Context, userID uint, tx *gorm.DB) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.GetActiveGrantsLocked")
	defer span.End()

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	// Use provided transaction if available, otherwise use default database connection
	dbConn := gm.db
	if tx != nil {
		dbConn = tx
	}

	var grants []*pluginModels.AllowanceGrant
	now := time.Now().UTC()

	// Build query with SQL ordering by grant source priority and row-level locking
	query := dbConn.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND is_active = true", userID)

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
func (gm *GrantManagerDefault) ConsumeFromGrants(ctx context.Context, userID uint, grantType pluginModels.GrantType, bytes uint64, usageDetailID uint, tx *gorm.DB) ([]*pluginModels.AllowanceConsumption, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.ConsumeFromGrants")
	defer span.End()

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

	// Start a transaction if none was provided
	if tx == nil {
		err := db.RetryableTransaction(ctx, gm.db, func(tx *gorm.DB) *gorm.DB {
			if err := gm.consumeFromGrantsInTransaction(ctx, tx, userID, grantType, bytes, usageDetailID, &consumptions); err != nil {
				_ = tx.AddError(err)
			}
			return tx
		})
		if err != nil {
			return nil, err
		}
	} else {
		err := gm.consumeFromGrantsInTransaction(ctx, tx, userID, grantType, bytes, usageDetailID, &consumptions)
		if err != nil {
			return nil, err
		}
	}

	return consumptions, nil
}

// consumeFromGrantsInTransaction performs the actual grant consumption within a transaction
func (gm *GrantManagerDefault) consumeFromGrantsInTransaction(ctx context.Context, tx *gorm.DB, userID uint, grantType pluginModels.GrantType, bytes uint64, usageDetailID uint, consumptions *[]*pluginModels.AllowanceConsumption) error {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.consumeFromGrantsInTransaction")
	defer span.End()

	// Get active grants for user and type with row-level locks
	grants, err := gm.GetActiveGrantsByTypeLocked(ctx, userID, grantType, tx)
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
		result := tx.WithContext(ctx).Model(&pluginModels.AllowanceGrant{}).
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
		if err := tx.WithContext(ctx).Create(consumption).Error; err != nil {
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
func (gm *GrantManagerDefault) DeactivateGrant(ctx context.Context, grantID uint) error {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.DeactivateGrant")
	defer span.End()

	if grantID == 0 {
		return pluginModels.ErrInvalidGrantID
	}

	// Update the grant directly using raw SQL to bypass model validation
	result := gm.db.WithContext(ctx).Model(&pluginModels.AllowanceGrant{}).
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
func (gm *GrantManagerDefault) GetExpiringGrants(ctx context.Context, expiryWindow time.Duration) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.GetExpiringGrants")
	defer span.End()

	now := time.Now().UTC()
	cutoff := now.Add(expiryWindow)

	var grants []*pluginModels.AllowanceGrant
	err := gm.db.WithContext(ctx).Where("is_active = true AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > ?",
		cutoff, now).
		Order("expiry_date ASC").
		Find(&grants).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expiring grants: %w", err)
	}

	return grants, nil
}

// GetExpiringGrantsForUser gets grants expiring within a time window for a specific user
func (gm *GrantManagerDefault) GetExpiringGrantsForUser(ctx context.Context, userID uint, window time.Duration) ([]*pluginModels.AllowanceGrant, error) {
	ctx, span := core.TraceMethod(ctx, "GrantManagerDefault.GetExpiringGrantsForUser")
	defer span.End()

	if userID == 0 {
		return nil, pluginModels.ErrInvalidUserID
	}

	now := time.Now().UTC()
	cutoff := now.Add(window)

	var grants []*pluginModels.AllowanceGrant
	err := gm.db.WithContext(ctx).Where("user_id = ? AND is_active = true AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > ?",
		userID, cutoff, now).
		Order("expiry_date ASC").
		Find(&grants).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get expiring grants for user: %w", err)
	}

	return grants, nil
}
