package models

import (
	"gorm.io/gorm"
)

// QuotaPlan - Reusable quota configuration templates (for subscription-style models)
type QuotaPlan struct {
	gorm.Model
	Name                   string    `gorm:"uniqueIndex"`
	Description            string
	StorageLimit           uint64
	UploadDailyLimit       uint64
	DownloadDailyLimit     uint64
	UploadTotalLimit       uint64
	DownloadTotalLimit     uint64
	StorageThreshold       *uint64
	UploadThreshold        *uint64
	DownloadThreshold      *uint64
	IsDefault              bool
	IsActive               bool
}

// BeforeCreate validates the QuotaPlan model before creation
func (q *QuotaPlan) BeforeCreate(_ *gorm.DB) error {
	return q.validate()
}

// BeforeUpdate validates the QuotaPlan model before update
func (q *QuotaPlan) BeforeUpdate(_ *gorm.DB) error {
	return q.validate()
}

// BeforeDelete validates the QuotaPlan model before deletion
func (q *QuotaPlan) BeforeDelete(tx *gorm.DB) error {
	// Prevent deletion if this is the default plan
	if q.IsDefault {
		return ErrCannotDeleteDefaultPlan
	}

	// Check if any UserQuotaConfig references this plan
	var count int64
	err := tx.Model(&UserQuotaConfig{}).Where("quota_plan_id = ?", q.ID).Count(&count).Error
	if err != nil {
		return err
	}

	if count > 0 {
		return ErrCannotDeleteReferencedPlan
	}

	return nil
}

// validate performs validation checks on the QuotaPlan fields
func (q *QuotaPlan) validate() error {
	if q.Name == "" {
		return ErrInvalidPlanName
	}

	// Validate that limit fields are greater than 0
	if q.StorageLimit <= 0 {
		return ErrInvalidStorageLimit
	}

	if q.UploadDailyLimit <= 0 {
		return ErrInvalidUploadDailyLimit
	}

	if q.DownloadDailyLimit <= 0 {
		return ErrInvalidDownloadDailyLimit
	}

	if q.UploadTotalLimit <= 0 {
		return ErrInvalidUploadTotalLimit
	}

	if q.DownloadTotalLimit <= 0 {
		return ErrInvalidDownloadTotalLimit
	}
	
	// Validate thresholds are <= corresponding limits if both are set
	if q.StorageThreshold != nil && *q.StorageThreshold > q.StorageLimit {
		return ErrInvalidStorageThreshold
	}

	if q.UploadThreshold != nil && *q.UploadThreshold > q.UploadDailyLimit {
		return ErrInvalidUploadThreshold
	}

	if q.DownloadThreshold != nil && *q.DownloadThreshold > q.DownloadDailyLimit {
		return ErrInvalidDownloadThreshold
	}

	return nil
}
