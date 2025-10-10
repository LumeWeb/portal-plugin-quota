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
func (u *UserUsageDetail) BeforeUpdate(tx *gorm.DB) error {
	return u.validatePartial(tx)
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

	if u.SharedWith > 1000 {
		return ErrInvalidSharedWith
	}

	if u.SharedWith < 0 {
		return ErrInvalidSharedWith
	}

	return nil
}

// validatePartial performs validation only on changed fields
func (u *UserUsageDetail) validatePartial(tx *gorm.DB) error {
	if tx.Statement.Changed("user_id") && u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if tx.Statement.Changed("upload_id") && u.UploadID <= 0 {
		return ErrInvalidUploadID
	}

	if tx.Statement.Changed("type") && !u.Type.IsValid() {
		return ErrInvalidUsageType
	}

	if tx.Statement.Changed("bytes") && u.Bytes <= 0 {
		return ErrInvalidBytes
	}

	if tx.Statement.Changed("ip") {
		if net.ParseIP(u.IP) == nil {
			return ErrInvalidIP
		}
	}

	if tx.Statement.Changed("timestamp") && u.Timestamp.IsZero() {
		return ErrInvalidTimestamp
	}

	if tx.Statement.Changed("shared_with") && (u.SharedWith > 1000 || u.SharedWith < 0) {
		return ErrInvalidSharedWith
	}

	return nil
}
