package models

import (
	"gorm.io/gorm"
)

// QuotaPlan - Reusable quota configuration templates (for subscription-style models)
type QuotaPlan struct {
	gorm.Model
	Name                   string    `gorm:"uniqueIndex"`
	Description            string
	StorageLimit           int64
	UploadDailyLimit       int64
	DownloadDailyLimit     int64
	UploadTotalLimit       int64
	DownloadTotalLimit     int64
	StorageThreshold       *int64
	UploadThreshold        *int64
	DownloadThreshold      *int64
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

	// Validate that limit fields are either -1 (unlimited), 0 (disabled), or positive (actual limit)
	// For required limits, we check that they're not unreasonably negative
	if q.StorageLimit < -1 {
		return ErrInvalidStorageLimit
	}

	if q.UploadDailyLimit < -1 {
		return ErrInvalidUploadDailyLimit
	}

	if q.DownloadDailyLimit < -1 {
		return ErrInvalidDownloadDailyLimit
	}

	if q.UploadTotalLimit < -1 {
		return ErrInvalidUploadTotalLimit
	}

	if q.DownloadTotalLimit < -1 {
		return ErrInvalidDownloadTotalLimit
	}
	

	return nil
}
