package reservation

import (
	"sync"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// ReservationManagerDefault implements the pluginCore.ReservationManager interface
// using in-memory tracking. This implementation tracks reservations per user in
// memory and automatically cleans them up when released.
type ReservationManagerDefault struct {
	ctx    core.Context
	logger *core.Logger

	// reservations maps reservation UUIDs to their data
	reservations map[string]*userReservation

	// mu protects the reservations map
	mu sync.RWMutex

	// Per-user tracking for fast lookup
	userReservations map[uint]map[string]struct{} // userID -> reservationUUIDs

	// cleanupAge is the age after which inactive reservations are cleaned up
	cleanupAge time.Duration
}

// userReservation represents a reservation for a specific user.
type userReservation struct {
	uuid       string
	userID     uint
	usageType  pluginCore.UsageType
	bytes      int64
	released   int32
	onRelease  func()
	createdAt  time.Time
}

// NewReservationManager creates a new ReservationManager instance.
func NewReservationManager(ctx core.Context) pluginCore.ReservationManager {
	return NewReservationManagerWithCleanup(ctx, 5*time.Minute)
}

// NewReservationManagerWithCleanup creates a new ReservationManager instance
// with a custom cleanup interval for stale reservations.
func NewReservationManagerWithCleanup(ctx core.Context, cleanupAge time.Duration) pluginCore.ReservationManager {
	if cleanupAge <= 0 {
		cleanupAge = 5 * time.Minute
	}
	rm := &ReservationManagerDefault{
		ctx:              ctx,
		logger:           ctx.Logger(),
		reservations:     make(map[string]*userReservation),
		userReservations: make(map[uint]map[string]struct{}),
		cleanupAge:       cleanupAge,
	}
	go rm.cleanupLoop()
	return rm
}

// cleanupLoop periodically cleans up stale reservations that have exceeded
// the cleanup age threshold. This prevents memory leaks from abandoned
// reservations.
func (rm *ReservationManagerDefault) cleanupLoop() {
	ticker := time.NewTicker(rm.cleanupAge)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.cleanupStaleReservations()
		}
	}
}

// cleanupStaleReservations removes reservations that have exceeded the
// cleanup age threshold and have been released.
func (rm *ReservationManagerDefault) cleanupStaleReservations() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoffTime := time.Now().Add(-rm.cleanupAge)
	releasedCount := 0

	for uuid, res := range rm.reservations {
		// Only clean up released, aged reservations
		if res.released == 1 && res.createdAt.Before(cutoffTime) {
			delete(rm.reservations, uuid)
			if userRes, ok := rm.userReservations[res.userID]; ok {
				delete(userRes, uuid)
				if len(userRes) == 0 {
					delete(rm.userReservations, res.userID)
				}
			}
			releasedCount++
		}
	}

	if releasedCount > 0 {
		rm.logger.Debug("Cleaned up stale reservations",
			zap.Int("count", releasedCount),
			zap.Duration("cleanup_age", rm.cleanupAge),
		)
	}
}




