package models

import (
	"time"

	"gorm.io/gorm"
)

// AllowanceConsumption - Track how grants are consumed
type AllowanceConsumption struct {
	gorm.Model
	GrantID         uint      `gorm:"index"`
	UsageDetailID   uint      `gorm:"index"`
	BytesConsumed   uint64    // How much was consumed from this grant
	ConsumptionDate time.Time `gorm:"index"`

	// Relationships
	Grant       *AllowanceGrant  `gorm:"foreignKey:GrantID"`
	UsageDetail *UserUsageDetail `gorm:"foreignKey:UsageDetailID"`
}

// BeforeCreate validates the AllowanceConsumption model before creation
func (a *AllowanceConsumption) BeforeCreate(_ *gorm.DB) error {
	return a.validate()
}

// BeforeUpdate validates the AllowanceConsumption model before update
func (a *AllowanceConsumption) BeforeUpdate(_ *gorm.DB) error {
	return a.validate()
}

// validate performs validation checks on the AllowanceConsumption fields
func (a *AllowanceConsumption) validate() error {
	if a.GrantID <= 0 {
		return ErrInvalidGrantID
	}

	if a.UsageDetailID <= 0 {
		return ErrInvalidUsageDetailID
	}

	if a.BytesConsumed <= 0 {
		return ErrInvalidBytesConsumed
	}

	if a.ConsumptionDate.IsZero() {
		return ErrInvalidConsumptionDate
	}

	return nil
}
