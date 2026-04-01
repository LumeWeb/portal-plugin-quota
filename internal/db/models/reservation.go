package models

import (
	"errors"

	"gorm.io/gorm"
)

// Predefined validation errors for reservations
var (
	ErrInvalidReservationStatus = errors.New("reservation status is invalid")
	ErrReservationAlreadyUsed   = errors.New("reservation has already been committed or rolled back")
)

// ReservationStatus represents the status of a quota reservation
type ReservationStatus string

const (
	ReservationStatusPending   ReservationStatus = "PENDING"    // Reservation created, quota deducted, not committed yet
	ReservationStatusCommitted ReservationStatus = "COMMITTED"  // Reservation converted to usage record
	ReservationStatusRolledBack ReservationStatus = "ROLLED_BACK" // Reservation explicitly canceled by caller
)

// String returns the string representation of the reservation status
func (r ReservationStatus) String() string {
	return string(r)
}

// TableName sets the table name for ReservationStatus
func (ReservationStatus) TableName() string {
	return "reservation_statuses"
}

// IsValid checks if the reservation status is valid
func (r ReservationStatus) IsValid() bool {
	switch r {
	case ReservationStatusPending, ReservationStatusCommitted, ReservationStatusRolledBack:
		return true
	default:
		return false
	}
}

// QuotaReservation represents a quota reservation that deducts quota
// at check time and is committed to a usage record after the operation completes.
//
// Status lifecycle:
//   - PENDING: Reservation created, awaiting operation completion
//   - COMMITTED: Operation succeeded, usage recorded
//   - ROLLED_BACK: Operation failed/canceled, quota released
//
// Soft delete (DeletedAt): Used for expired/stale reservations that
// weren't explicitly committed or rolled back. Use Unscoped() to access
// these records if needed for debugging or recovery.
type QuotaReservation struct {
	gorm.Model // Includes ID, CreatedAt, UpdatedAt, DeletedAt

	UserID   uint
	Type     UsageType // UPLOAD, DOWNLOAD, STORAGE_ADD
	Bytes    uint64
	Status   ReservationStatus
	UploadID *uint // Associated upload (set when committed)
	SourceIP string // IP address of the requester
}

// TableName specifies the table name for QuotaReservation
func (QuotaReservation) TableName() string {
	return "quota_reservations"
}

// BeforeCreate validates the reservation before creation
func (qr *QuotaReservation) BeforeCreate(tx *gorm.DB) error {
	if qr.UserID == 0 {
		return errors.New("user ID is required")
	}
	if qr.Bytes == 0 {
		return errors.New("bytes cannot be zero")
	}
	if qr.Status == "" {
		qr.Status = ReservationStatusPending
	}
	return nil
}

// IsPending returns true if the reservation is still pending
func (qr *QuotaReservation) IsPending() bool {
	return qr.Status == ReservationStatusPending
}

// IsCommitted returns true if the reservation has been committed
func (qr *QuotaReservation) IsCommitted() bool {
	return qr.Status == ReservationStatusCommitted
}

// IsRolledBack returns true if the reservation has been rolled back
func (qr *QuotaReservation) IsRolledBack() bool {
	return qr.Status == ReservationStatusRolledBack
}

// IsDeleted returns true if the reservation has been soft deleted (expired/cleanup)
func (qr *QuotaReservation) IsDeleted() bool {
	return qr.DeletedAt.Valid
}
