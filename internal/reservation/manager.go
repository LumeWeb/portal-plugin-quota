package reservation

import (
	"sync"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal/core"
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
}

// NewReservationManager creates a new ReservationManager instance.
func NewReservationManager(ctx core.Context) pluginCore.ReservationManager {
	return NewReservationManagerWithCleanup(ctx, 5*time.Minute)
}

// NewReservationManagerWithCleanup creates a new ReservationManager instance
// with a custom cleanup interval for stale reservations.
func NewReservationManagerWithCleanup(ctx core.Context, cleanupAge time.Duration) pluginCore.ReservationManager {
	return &ReservationManagerDefault{
		ctx:              ctx,
		logger:           ctx.Logger(),
		reservations:     make(map[string]*userReservation),
		userReservations: make(map[uint]map[string]struct{}),
		cleanupAge:       cleanupAge,
	}
}




