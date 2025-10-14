package models

import (
	"gorm.io/gorm"
)

// UserQuotaConfig - Per-user quota configuration
type UserQuotaConfig struct {
	gorm.Model
	UserID             uint              `gorm:"uniqueIndex"`
	EnforcementPolicy  EnforcementPolicy `gorm:"index"`
	QuotaPlanID        *uint64
	StorageLimit       *int64
	UploadDailyLimit   *int64
	DownloadDailyLimit *int64
	UploadTotalLimit   *int64
	DownloadTotalLimit *int64
	StorageThreshold   *int64
	UploadThreshold    *int64
	DownloadThreshold  *int64
	
	// Relationships
	AllowanceGrants []*AllowanceGrant `gorm:"foreignKey:UserID;references:UserID"`
}

// TableName sets the table name for UserQuotaConfig
func (UserQuotaConfig) TableName() string {
	return "user_quota_configs"
}

// BeforeCreate validates the UserQuotaConfig model before creation
func (u *UserQuotaConfig) BeforeCreate(tx *gorm.DB) error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if !u.EnforcementPolicy.IsValid() {
		return ErrInvalidEnforcementPolicy
	}

	return nil
}

// BeforeUpdate validates the UserQuotaConfig model before update
func (u *UserQuotaConfig) BeforeUpdate(tx *gorm.DB) error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if !u.EnforcementPolicy.IsValid() {
		return ErrInvalidEnforcementPolicy
	}

	return nil
}
