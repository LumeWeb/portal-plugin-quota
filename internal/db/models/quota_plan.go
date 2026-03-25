package models

import (
	"gorm.io/gorm"
)

// QuotaPlan - Reusable quota configuration templates (for subscription-style models)
type QuotaPlan struct {
	gorm.Model
	Name               string `gorm:"uniqueIndex"`
	Description        string
	StorageLimit       int64
	UploadDailyLimit   int64
	DownloadDailyLimit int64
	UploadTotalLimit   int64
	DownloadTotalLimit int64
	StorageThreshold   *int64
	UploadThreshold    *int64
	DownloadThreshold  *int64
	IsDefault          bool
	IsActive           *bool
}

// BeforeDelete validates the QuotaPlan model before deletion
func (q *QuotaPlan) BeforeDelete(tx *gorm.DB) error {
	// Prevent deletion if this is the default plan
	if q.IsDefault {
		return ErrCannotDeleteDefaultPlan
	}

	// Check if any UserQuotaConfig references this plan
	var count int64
	if err := tx.Model(&UserQuotaConfig{}).Where("quota_plan_id = ?", q.ID).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return ErrCannotDeleteReferencedPlan
	}

	return nil
}

// BeforeSave validates the QuotaPlan model before both create and update operations
func (q *QuotaPlan) BeforeSave(tx *gorm.DB) error {
	// Default active unless explicitly set (only runs for create since updates preserve existing value)
	if q.IsActive == nil {
		q.IsActive = new(true)
	}

	// Name must not be empty (applies to both create and update)
	if q.Name == "" {
		return ErrInvalidPlanName
	}

	// Validate limits are not unreasonably negative
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

	// Validate thresholds
	if q.StorageThreshold != nil {
		if *q.StorageThreshold < 0 {
			return ErrInvalidStorageThreshold
		}
		if q.StorageLimit > 0 && *q.StorageThreshold > q.StorageLimit {
			return ErrThresholdExceedsLimit
		}
	}

	if q.UploadThreshold != nil {
		if *q.UploadThreshold < 0 {
			return ErrInvalidUploadThreshold
		}
		if q.UploadDailyLimit > 0 && *q.UploadThreshold > q.UploadDailyLimit {
			return ErrThresholdExceedsLimit
		}
	}

	if q.DownloadThreshold != nil {
		if *q.DownloadThreshold < 0 {
			return ErrInvalidDownloadThreshold
		}
		if q.DownloadDailyLimit > 0 && *q.DownloadThreshold > q.DownloadDailyLimit {
			return ErrThresholdExceedsLimit
		}
	}

	return nil
}
