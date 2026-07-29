package models

import (
	"fmt"
	"gorm.io/gorm"
)

// QuotaPlan - Reusable quota configuration templates with window-based limits
type QuotaPlan struct {
	gorm.Model
	Name       string
	Description string
	IsDefault   bool
	IsActive    *bool

	// Window configuration (shared across all limits)
	WindowType      WindowType
	WindowDuration  *int64
	WindowStartHour *int
	WindowTimezone  *string

	// Byte limits (all use the same window configuration above)
	StorageLimitBytes   uint64
	UploadLimitBytes    uint64
	DownloadLimitBytes  uint64

	// Thresholds (for THRESHOLD policy)
	StorageThreshold  *int64
	UploadThreshold   *int64
	DownloadThreshold *int64

	// ExcludedFromHealthReports excludes users on this plan from CID pin health
	// aggregates. Use for admin/system accounts that would otherwise skew health
	// reporting (e.g. unlimited admin accounts reporting indefinite days remaining).
	ExcludedFromHealthReports bool `gorm:"default:false"`
}

// TableName sets the table name for QuotaPlan
func (QuotaPlan) TableName() string {
	return "quota_plans"
}

// validateWindow validates the window configuration
func (q *QuotaPlan) validateWindow() error {
	// Validate window type
	if q.WindowType != "" && !q.WindowType.IsValid() {
		return fmt.Errorf("invalid window type: %s", q.WindowType)
	}

	// For ROLLING windows, duration must be positive
	if q.WindowType == WindowTypeRolling && q.WindowDuration != nil {
		if *q.WindowDuration <= 0 {
			return fmt.Errorf("ROLLING window requires positive duration")
		}
	}

	// Validate start hour
	if q.WindowStartHour != nil && (*q.WindowStartHour < 0 || *q.WindowStartHour > 23) {
		return fmt.Errorf("start_hour must be 0-23")
	}

	return nil
}

// validateThresholds validates threshold values against limits
func (q *QuotaPlan) validateThresholds() error {
	if q.StorageThreshold != nil {
		if *q.StorageThreshold < 0 {
			return ErrInvalidStorageThreshold
		}
		if q.StorageLimitBytes > 0 && uint64(*q.StorageThreshold) > q.StorageLimitBytes {
			return ErrThresholdExceedsLimit
		}
	}

	if q.UploadThreshold != nil {
		if *q.UploadThreshold < 0 {
			return ErrInvalidUploadThreshold
		}
		if q.UploadLimitBytes > 0 && uint64(*q.UploadThreshold) > q.UploadLimitBytes {
			return ErrThresholdExceedsLimit
		}
	}

	if q.DownloadThreshold != nil {
		if *q.DownloadThreshold < 0 {
			return ErrInvalidDownloadThreshold
		}
		if q.DownloadLimitBytes > 0 && uint64(*q.DownloadThreshold) > q.DownloadLimitBytes {
			return ErrThresholdExceedsLimit
		}
	}

	return nil
}

// BeforeSave validates the QuotaPlan model before both create and update operations
func (q *QuotaPlan) BeforeSave(tx *gorm.DB) error {
	if q.IsActive == nil {
		q.IsActive = new(true)
	}

	if q.Name == "" {
		return ErrInvalidPlanName
	}

	if err := q.validateWindow(); err != nil {
		return err
	}

	if err := q.validateThresholds(); err != nil {
		return err
	}

	return nil
}

// BeforeDelete validates the QuotaPlan model before deletion
func (q *QuotaPlan) BeforeDelete(tx *gorm.DB) error {
	if q.IsDefault {
		return ErrCannotDeleteDefaultPlan
	}

	var count int64
	if err := tx.Model(&UserQuotaConfig{}).Where("quota_plan_id = ?", q.ID).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		return ErrCannotDeleteReferencedPlan
	}

	return nil
}
