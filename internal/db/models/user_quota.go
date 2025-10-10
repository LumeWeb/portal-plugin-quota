package models

import (
	"time"

	"gorm.io/gorm"
)

// UserQuota - Aggregated daily quota usage
type UserQuota struct {
	gorm.Model
	UserID          uint      `gorm:"uniqueIndex:user_date"`
	Date            time.Time `gorm:"index;uniqueIndex:user_date"`
	BytesUploaded   uint64
	BytesDownloaded uint64
	BytesStored     uint64
}

// BeforeCreate validates the UserQuota model before creation
func (u *UserQuota) BeforeCreate(_ *gorm.DB) error {
	return u.validate()
}

// BeforeUpdate validates the UserQuota model before update
func (u *UserQuota) BeforeUpdate(_ *gorm.DB) error {
	return u.validate()
}

// validate performs validation checks on the UserQuota fields
func (u *UserQuota) validate() error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if u.Date.IsZero() {
		return ErrInvalidDate
	}

	return nil
}
