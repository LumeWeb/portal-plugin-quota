package reservation

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/google/uuid"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// Reserve creates a new quota reservation.
func (rm *ReservationManagerDefault) Reserve(ctx context.Context, userID uint, usageType pluginCore.UsageType, bytes int64) (pluginCore.Reservation, error) {
	ctx, span := core.TraceMethod(ctx, "ReservationManagerDefault.Reserve")
	defer span.End()

	if userID == 0 {
		rm.logger.Error("Cannot create reservation for invalid user ID", zap.Uint("user_id", userID))
		return nil, errors.New("invalid user ID: user ID must be greater than 0")
	}

	if bytes <= 0 {
		rm.logger.Error("Cannot create reservation for invalid bytes", zap.Int64("bytes", bytes))
		return nil, errors.New("invalid bytes: bytes must be greater than 0")
	}

	// Generate UUID for the reservation
	reservationUUID := uuid.New().String()

	rm.logger.Debug("Creating reservation",
		zap.String("reservation_uuid", reservationUUID),
		zap.Uint("user_id", userID),
		zap.String("usage_type", string(usageType)),
		zap.Int64("bytes", bytes),
	)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Create the reservation
	res := &userReservation{
		uuid:      reservationUUID,
		userID:    userID,
		usageType: usageType,
		bytes:     bytes,
		released:  0,
		onRelease: func() {
			rm.cleanupReservation(reservationUUID, userID)
		},
	}

	// Store the reservation
	rm.reservations[reservationUUID] = res

	// Track reservations per user
	if rm.userReservations[userID] == nil {
		rm.userReservations[userID] = make(map[string]struct{})
	}
	rm.userReservations[userID][reservationUUID] = struct{}{}

	rm.logger.Debug("Reservation created successfully",
		zap.String("reservation_uuid", reservationUUID),
		zap.Uint("user_id", userID),
		zap.Int64("bytes", bytes),
	)

	return &defaultReservation{
		res:     res,
		manager: rm,
	}, nil
}

// cleanupReservation removes a reservation from tracking maps.
func (rm *ReservationManagerDefault) cleanupReservation(reservationUUID string, userID uint) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Remove from main map
	delete(rm.reservations, reservationUUID)

	// Remove from user tracking
	if userRes, ok := rm.userReservations[userID]; ok {
		delete(userRes, reservationUUID)
		// Clean up empty user map
		if len(userRes) == 0 {
			delete(rm.userReservations, userID)
		}
	}

	rm.logger.Debug("Reservation cleaned up",
		zap.String("reservation_uuid", reservationUUID),
		zap.Uint("user_id", userID),
	)
}

// GetReservation retrieves a reservation by its UUID.
// Returns nil if the reservation is not found or has been released.
func (rm *ReservationManagerDefault) GetReservation(uuid string) pluginCore.Reservation {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	res, ok := rm.reservations[uuid]
	if !ok || atomic.LoadInt32(&res.released) == 1 {
		return nil
	}
	return &defaultReservation{
		res:     res,
		manager: rm,
	}
}

// SumPendingBytesForUser returns the total bytes currently reserved for a user
// and usage type. This is used during quota checks to prevent over-allocation.
func (rm *ReservationManagerDefault) SumPendingBytesForUser(ctx context.Context, userID uint, usageType pluginCore.UsageType) int64 {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var total int64
	if userRes, ok := rm.userReservations[userID]; ok {
		for uuid := range userRes {
			if res, ok := rm.reservations[uuid]; ok && res.usageType == usageType {
				total += res.bytes
			}
		}
	}
	return total
}

// defaultReservation implements the pluginCore.Reservation interface.
type defaultReservation struct {
	res     *userReservation
	manager *ReservationManagerDefault
}

// Release releases the reservation.
// Multiple calls to Release are safe - subsequent calls are no-ops.
func (r *defaultReservation) Release() {
	if !atomic.CompareAndSwapInt32(&r.res.released, 0, 1) {
		return
	}

	r.manager.logger.Debug("Releasing reservation",
		zap.String("reservation_uuid", r.res.uuid),
		zap.Uint("user_id", r.res.userID),
		zap.Int64("bytes", r.res.bytes),
	)

	if r.res.onRelease != nil {
		r.res.onRelease()
	}
}

// UUID returns the reservation UUID.
func (r *defaultReservation) UUID() string {
	return r.res.uuid
}

// UserID returns the user ID for this reservation.
func (r *defaultReservation) UserID() uint {
	return r.res.userID
}

// UsageType returns the usage type for this reservation.
func (r *defaultReservation) UsageType() pluginCore.UsageType {
	return r.res.usageType
}

// Bytes returns the number of bytes reserved.
func (r *defaultReservation) Bytes() int64 {
	return r.res.bytes
}
