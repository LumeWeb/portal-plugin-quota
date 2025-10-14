package models

import (
	"time"

	"go.uber.org/zap"
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

// TableName sets the table name for UserQuota
func (UserQuota) TableName() string {
	return "user_quotas"
}

// BeforeCreate validates the UserQuota model before creation
func (u *UserQuota) BeforeCreate(tx *gorm.DB) error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if u.Date.IsZero() {
		return ErrInvalidDate
	}

	return nil
}

// BeforeUpdate validates the UserQuota model before update
func (u *UserQuota) BeforeUpdate(tx *gorm.DB) error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if u.Date.IsZero() {
		return ErrInvalidDate
	}

	return nil
}
