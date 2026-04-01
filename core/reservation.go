package core

import (
	"context"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// ReservationManager handles quota reservations
type ReservationManager interface {
	// CreateReservation creates a new quota reservation
	CreateReservation(ctx context.Context, userID uint, usageType UsageType, bytes uint64, ip string) (*models.QuotaReservation, error)

	// CommitReservation commits a reservation to a usage record
	CommitReservation(ctx context.Context, reservationID uint, uploadID uint) error

	// ReleaseReservation releases a reservation
	ReleaseReservation(ctx context.Context, reservationID uint) error

	// GetReservationByID retrieves a reservation
	GetReservationByID(ctx context.Context, reservationID uint) (*models.QuotaReservation, error)

	// GetPendingReservationsForUser gets all pending reservations for a user
	GetPendingReservationsForUser(ctx context.Context, userID uint) ([]*models.QuotaReservation, error)

	// SumPendingBytesForUser sums pending reservation bytes for a user and type
	SumPendingBytesForUser(ctx context.Context, userID uint, usageType UsageType) (uint64, error)

	// CleanupStaleReservations releases pending reservations older than configured timeout
	// This cleans up for ALL users and should be called periodically
	CleanupStaleReservations(ctx context.Context) (int64, error)

	// CleanupStaleReservationsForUser releases pending reservations for a specific user
	// Optimized for hot path cleanup during quota checks
	CleanupStaleReservationsForUser(ctx context.Context, userID uint) (int64, error)
}
