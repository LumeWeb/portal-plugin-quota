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
	StorageLimit       *uint64
	UploadDailyLimit   *uint64
	DownloadDailyLimit *uint64
	UploadTotalLimit   *uint64
	DownloadTotalLimit *uint64
	StorageThreshold   *uint64
	UploadThreshold    *uint64
	DownloadThreshold  *uint64
	
	// Relationships
	AllowanceGrants []*AllowanceGrant `gorm:"foreignKey:UserID"`
}

// BeforeCreate validates the UserQuotaConfig model before creation
func (u *UserQuotaConfig) BeforeCreate(_ *gorm.DB) error {
	return u.validate()
}

// BeforeUpdate validates the UserQuotaConfig model before update
func (u *UserQuotaConfig) BeforeUpdate(_ *gorm.DB) error {
	return u.validate()
}

// validate performs validation checks on the UserQuotaConfig fields
func (u *UserQuotaConfig) validate() error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if !u.EnforcementPolicy.IsValid() {
		return ErrInvalidEnforcementPolicy
	}


	// Validate custom limits if set
	if u.StorageLimit != nil && *u.StorageLimit < 0 {
		return ErrInvalidStorageLimit
	}

	if u.UploadDailyLimit != nil && *u.UploadDailyLimit < 0 {
		return ErrInvalidUploadDailyLimit
	}

	if u.DownloadDailyLimit != nil && *u.DownloadDailyLimit < 0 {
		return ErrInvalidDownloadDailyLimit
	}

	if u.UploadTotalLimit != nil && *u.UploadTotalLimit < 0 {
		return ErrInvalidUploadTotalLimit
	}

	if u.DownloadTotalLimit != nil && *u.DownloadTotalLimit < 0 {
		return ErrInvalidDownloadTotalLimit
	}

	// Validate thresholds are <= corresponding limits if both are set
	if u.StorageLimit != nil && u.StorageThreshold != nil && *u.StorageThreshold > *u.StorageLimit {
		return ErrInvalidStorageThreshold
	}

	if u.UploadDailyLimit != nil && u.UploadThreshold != nil && *u.UploadThreshold > *u.UploadDailyLimit {
		return ErrInvalidUploadThreshold
	}

	if u.DownloadDailyLimit != nil && u.DownloadThreshold != nil && *u.DownloadThreshold > *u.DownloadDailyLimit {
		return ErrInvalidDownloadThreshold
	}

	return nil
}
