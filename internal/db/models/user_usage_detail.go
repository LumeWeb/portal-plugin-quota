package models

import (
	"net"
	"time"

	"gorm.io/gorm"
)

// UserUsageDetail - Detailed usage records for billing
type UserUsageDetail struct {
	gorm.Model
	UserID     uint      `gorm:"index"`
	UploadID   uint      `gorm:"index"`
	Type       UsageType `gorm:"index"` // UPLOAD, DOWNLOAD, STORAGE_ADD, STORAGE_REMOVE
	Bytes      uint64
	IP         string    `gorm:"index"`
	SharedWith uint      // Number of users sharing this object
	Timestamp  time.Time `gorm:"index"`

	// Link to consumption records for detailed tracking
	AllowanceConsumptions []*AllowanceConsumption `gorm:"foreignKey:UsageDetailID"`
}

// BeforeCreate validates the UserUsageDetail model before creation
func (u *UserUsageDetail) BeforeCreate(_ *gorm.DB) error {
	return u.validate()
}

// BeforeUpdate validates the UserUsageDetail model before update
func (u *UserUsageDetail) BeforeUpdate(_ *gorm.DB) error {
	return u.validate()
}

// validate performs validation checks on the UserUsageDetail fields
func (u *UserUsageDetail) validate() error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if u.UploadID <= 0 {
		return ErrInvalidUploadID
	}

	if !u.Type.IsValid() {
		return ErrInvalidUsageType
	}

	if u.Bytes <= 0 {
		return ErrInvalidBytes
	}

	if net.ParseIP(u.IP) == nil {
		return ErrInvalidIP
	}

	if u.Timestamp.IsZero() {
		return ErrInvalidTimestamp
	}

	return nil
}
