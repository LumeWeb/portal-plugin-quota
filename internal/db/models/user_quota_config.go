package models

import (
	"fmt"
	"gorm.io/gorm"
)

// UserQuotaConfig - Per-user quota configuration
type UserQuotaConfig struct {
	gorm.Model
	UserID            uint
	EnforcementPolicy EnforcementPolicy
	QuotaPlanID       *uint64

	// Window configuration (overrides plan if set, otherwise uses plan's)
	WindowType      WindowType
	WindowDuration  *int64
	WindowStartHour *int
	WindowTimezone  *string

	// Byte limits
	StorageLimitBytes   uint64
	UploadLimitBytes    uint64
	DownloadLimitBytes  uint64

	// Thresholds (for THRESHOLD policy)
	StorageThreshold  *int64
	UploadThreshold   *int64
	DownloadThreshold *int64

	// ExcludedFromHealthReports excludes this specific user from CID pin health
	// aggregates (e.g. admin/system accounts). Per-user override; plan-level flag
	// on QuotaPlan.ExcludedFromHealthReports also applies.
	ExcludedFromHealthReports bool

	// Relationships
	AllowanceGrants []*AllowanceGrant `gorm:"foreignKey:UserID;references:UserID"`
}

// TableName sets the table name for UserQuotaConfig
func (UserQuotaConfig) TableName() string {
	return "user_quota_configs"
}

// BeforeCreate validates the UserQuotaConfig model before creation
func (u *UserQuotaConfig) BeforeCreate(_ *gorm.DB) error {
	return u.validate()
}

// BeforeUpdate validates the UserQuotaConfig model before update
func (u *UserQuotaConfig) BeforeUpdate(_ *gorm.DB) error {
	return u.validate()
}

func (u *UserQuotaConfig) validate() error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if !u.EnforcementPolicy.IsValid() {
		return ErrInvalidEnforcementPolicy
	}

	// Validate window configuration
	if u.WindowType != "" && !u.WindowType.IsValid() {
		return fmt.Errorf("invalid window type: %s", u.WindowType)
	}

	// For ROLLING windows, duration must be positive
	if u.WindowType == WindowTypeRolling && u.WindowDuration != nil {
		if *u.WindowDuration <= 0 {
			return fmt.Errorf("ROLLING window requires positive duration")
		}
	}

	// Validate start hour
	if u.WindowStartHour != nil && (*u.WindowStartHour < 0 || *u.WindowStartHour > 23) {
		return fmt.Errorf("start_hour must be 0-23")
	}

	return nil
}
