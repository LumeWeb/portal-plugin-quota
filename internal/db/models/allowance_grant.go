package models

import (
	"time"

	"gorm.io/gorm"
)

// AllowanceGrant - Individual allowance grants for users
type AllowanceGrant struct {
	gorm.Model
	UserID         uint        `gorm:"index"`
	Type           GrantType   `gorm:"index"` // STORAGE, UPLOAD, DOWNLOAD
	Source         GrantSource `gorm:"index"` // SUBSCRIPTION, PAYG_ADDON, BONUS, PROMO
	Bytes          uint64      // Total bytes granted
	BytesUsed      uint64      // Bytes consumed so far
	BytesRemaining uint64      // Calculated field: Bytes - BytesUsed
	ExpiryDate     *time.Time  `gorm:"index"` // When grant expires (nil = never)
	IsActive       bool        `gorm:"index"` // Whether grant is currently active

	// Relationships
	UserQuotaConfig *UserQuotaConfig `gorm:"foreignKey:UserID;references:UserID"`
}

// TableName sets the table name for AllowanceGrant
func (AllowanceGrant) TableName() string {
	return "allowance_grants"
}

// BeforeSave computes BytesRemaining before persistence
func (a *AllowanceGrant) BeforeSave(_ *gorm.DB) error {
	// Compute BytesRemaining as Bytes - BytesUsed
	a.BytesRemaining = a.Bytes - a.BytesUsed
	
	// Guard against underflow (when BytesUsed > Bytes) - clamp to 0
	if a.BytesRemaining > a.Bytes {
		a.BytesRemaining = 0
	}
	
	return nil
}

// BeforeCreate validates the AllowanceGrant model before creation
func (a *AllowanceGrant) BeforeCreate(_ *gorm.DB) error {
	return a.validateOnCreate()
}

// BeforeUpdate validates the AllowanceGrant model before update
func (a *AllowanceGrant) BeforeUpdate(_ *gorm.DB) error {
	return a.validate()
}

// validate performs validation checks on the AllowanceGrant fields
func (a *AllowanceGrant) validate() error {
	if a.UserID <= 0 {
		return ErrInvalidUserID
	}

	if !a.Type.IsValid() {
		return ErrInvalidGrantType
	}

	if !a.Source.IsValid() {
		return ErrInvalidGrantSource
	}

	if a.Bytes <= 0 {
		return ErrInvalidBytes
	}

	if a.BytesUsed > a.Bytes {
		return ErrInvalidBytesUsed
	}


	return nil
}

// validateOnCreate performs validation checks on the AllowanceGrant fields including expiry date validation
func (a *AllowanceGrant) validateOnCreate() error {
	if err := a.validate(); err != nil {
		return err
	}

	// Validate that ExpiryDate is in the future if set (only on create)
	if a.ExpiryDate != nil && a.ExpiryDate.Before(time.Now()) {
		return ErrInvalidExpiryDateOnCreate
	}

	return nil
}
