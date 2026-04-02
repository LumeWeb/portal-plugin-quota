package core

import "context"

// ReservationManager provides quota reservation capabilities. Reservations
// hold quota capacity during operations and must be released when complete.
//
// This is an in-memory interface. Implementations track reservations in memory
// and automatically clean them up when released.
type ReservationManager interface {
	// Reserve creates a new quota reservation.
	//
	// The returned Reservation must be released by calling Release().
	// It is recommended to use defer to ensure the reservation is always released:
	//
	//	reservation, err := reservationManager.Reserve(ctx, userID, usageType, bytes)
	//	if err != nil {
	//	    return err
	//	}
	//	defer reservation.Release()
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - userID: User ID to reserve quota for
	//   - usageType: Type of usage (UPLOAD, DOWNLOAD, STORAGE_ADD)
	//   - bytes: Number of bytes to reserve
	//
	// Returns:
	//   - Reservation: Handle that must be released when done
	//   - error: Error if reservation fails (e.g., quota exceeded)
	Reserve(ctx context.Context, userID uint, usageType UsageType, bytes int64) (Reservation, error)

	// GetReservation retrieves a reservation by its UUID.
	// Returns nil if the reservation is not found or has been released.
	GetReservation(uuid string) Reservation

	// SumPendingBytesForUser returns the total bytes currently reserved for a user
	// and usage type. This is used during quota checks to prevent over-allocation.
	SumPendingBytesForUser(ctx context.Context, userID uint, usageType UsageType) int64

	// CountPendingReservationsForUser returns the number of active reservations
	// for a user and usage type. This is used for debugging and monitoring.
	CountPendingReservationsForUser(ctx context.Context, userID uint, usageType UsageType) int
}

// Reservation represents a held quota reservation that must be released.
// The Reservation handle ensures that reserved quota is properly released,
// preventing quota leakage.
type Reservation interface {
	// Release releases the reservation, returning the quota to the pool.
	// Calling Release multiple times is safe - subsequent calls are no-ops.
	// It is recommended to use defer to ensure the reservation is always released.
	Release()

	// UUID returns the unique identifier for this reservation.
	UUID() string

	// UserID returns the user ID for this reservation.
	UserID() uint

	// UsageType returns the usage type for this reservation.
	UsageType() UsageType

	// Bytes returns the number of bytes reserved.
	Bytes() int64
}
