package policies

import (
	"errors"
	"fmt"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// QuotaPlanManagerDefault implements QuotaPlanManager interface
type QuotaPlanManagerDefault struct {
	db     *gorm.DB
	logger *core.Logger
}

// NewQuotaPlanManager creates a new quota plan manager default implementation
func NewQuotaPlanManager(db *gorm.DB, logger *core.Logger) *QuotaPlanManagerDefault {
	return &QuotaPlanManagerDefault{
		db:     db,
		logger: logger,
	}
}

// GetQuotaPlanByID retrieves a quota plan by its ID
func (q *QuotaPlanManagerDefault) GetQuotaPlanByID(id uint64) (*models.QuotaPlan, error) {
	q.logger.Debug("GetQuotaPlanByID: retrieving quota plan by ID", zap.Uint64("id", id))

	var plan models.QuotaPlan
	err := q.db.First(&plan, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			q.logger.Debug("GetQuotaPlanByID: quota plan not found", zap.Uint64("id", id))
			return nil, fmt.Errorf("%w: %d", models.ErrQuotaPlanNotFound, id)
		}
		q.logger.Error("GetQuotaPlanByID: failed to retrieve quota plan",
			zap.Uint64("id", id),
			zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve quota plan: %w", err)
	}

	q.logger.Debug("GetQuotaPlanByID: quota plan retrieved successfully",
		zap.Uint64("id", id),
		zap.String("name", plan.Name))

	return &plan, nil
}

// GetDefaultQuotaPlan retrieves the default active quota plan
func (q *QuotaPlanManagerDefault) GetDefaultQuotaPlan() (*models.QuotaPlan, error) {
	q.logger.Debug("GetDefaultQuotaPlan: retrieving default active quota plan")

	var plan models.QuotaPlan
	err := q.db.Where("is_default = true AND is_active = true").First(&plan).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			q.logger.Debug("GetDefaultQuotaPlan: default active quota plan not found")
			return nil, models.ErrQuotaPlanNotFound
		}
		q.logger.Error("GetDefaultQuotaPlan: failed to retrieve default active quota plan", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve default quota plan: %w", err)
	}

	q.logger.Debug("GetDefaultQuotaPlan: default active quota plan retrieved successfully",
		zap.Uint("id", plan.ID),
		zap.String("name", plan.Name))

	return &plan, nil
}
