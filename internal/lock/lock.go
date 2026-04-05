package lock

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
)

// LockManagerDefault implements the LockManager interface using in-memory mutexes.
// This implementation is designed for single-node deployments. For distributed
// deployments, this can be replaced with a Redis, etcd, or other distributed
// lock implementation while maintaining the same LockManager interface.
type LockManagerDefault struct {
	ctx    core.Context
	logger *core.Logger

	// locks maps user IDs to their respective mutex locks
	locks map[uint]*userLock

	// mu protects the locks map itself
	mu sync.RWMutex
}

// userLock represents a lock for a specific user.
// It wraps sync.Mutex and tracks the number of waiters.
type userLock struct {
	mu      sync.Mutex
	waiters int64
}

// NewLockManager creates a new LockManager instance.
func NewLockManager(ctx core.Context) LockManager {
	return &LockManagerDefault{
		ctx:    ctx,
		logger: ctx.Logger(),
		locks:  make(map[uint]*userLock),
	}
}

// validateUserID validates the user ID. Returns no-op for userID 0 (anonymous operations).
func (lm *LockManagerDefault) validateUserID(userID uint) error {
	if userID == 0 {
		return nil // Anonymous operations use no-op lock
	}
	return nil
}

// acquireLock attempts to acquire a lock with polling to avoid goroutine leaks.
// This is a helper method used by both AcquireLock and AcquireLockWithTimeout.
func (lm *LockManagerDefault) acquireLock(ctx context.Context, ul *userLock) (*defaultLock, error) {
	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			if ul.mu.TryLock() {
				return &defaultLock{
					mu: &ul.mu,
				}, nil
			}
		}
	}
}

// acquireWithSetup handles common lock acquisition logic: validation, user lock setup,
// and error handling. This reduces duplication between AcquireLock and AcquireLockWithTimeout.
// The timeoutCtx parameter is used for error conversion in AcquireLockWithTimeout.
func (lm *LockManagerDefault) acquireWithSetup(ctx context.Context, userID uint, timeoutCtx *context.Context) (*defaultLock, error) {
	// Get or create the user lock
	ul := lm.getUserLock(userID)
	atomic.AddInt64(&ul.waiters, 1)

	// Choose the context for acquisition
	acquireCtx := ctx
	if timeoutCtx != nil {
		acquireCtx = *timeoutCtx
	}

	lock, err := lm.acquireLock(acquireCtx, ul)
	if err != nil {
		// Decrement waiters and cleanup on failure
		lm.mu.Lock()
		if atomic.AddInt64(&ul.waiters, -1) <= 0 {
			delete(lm.locks, userID)
		}
		lm.mu.Unlock()

		// Convert timeout error if applicable
		if timeoutCtx != nil && (*timeoutCtx).Err() == context.DeadlineExceeded {
			return nil, ErrLockTimeout
		}
		return nil, err
	}

	// Add cleanup callback to the lock
	lock.onRelease = func() {
		lm.cleanupLock(userID)
	}

	return lock, nil
}

// noOpLock is a no-operation lock implementation for anonymous operations.
// It satisfies the Lock interface but does nothing, avoiding serialization
// of anonymous operations while maintaining a consistent API.
type noOpLock struct{}

// Release is a no-op for the no-operation lock.
func (n *noOpLock) Release() {
	// No-op: anonymous operations don't need locking
}

// AcquireLock acquires a lock for the specified user ID.
// If the lock is already held, this blocks until it becomes available
// or the context is canceled.
// For userID 0 (anonymous operations), returns a no-op lock to avoid
// serializing all anonymous operations as a global mutex.
func (lm *LockManagerDefault) AcquireLock(ctx context.Context, userID uint) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.AcquireLock")
	defer span.End()

	// Return no-op lock for anonymous operations (userID == 0)
	if userID == 0 {
		return &noOpLock{}, nil
	}

	lm.logger.Debug("Acquiring lock for user", zap.Uint("user_id", userID))

	return lm.acquireWithSetup(ctx, userID, nil)
}

// AcquireLockWithTimeout attempts to acquire a lock for the specified user ID
// within the given timeout. If the timeout is exceeded, it returns ErrLockTimeout.
// For userID 0 (anonymous operations), returns a no-op lock immediately.
func (lm *LockManagerDefault) AcquireLockWithTimeout(ctx context.Context, userID uint, timeout Timeout) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.AcquireLockWithTimeout")
	defer span.End()

	// Return no-op lock for anonymous operations (userID == 0)
	if userID == 0 {
		return &noOpLock{}, nil
	}

	lm.logger.Debug("Acquiring lock with timeout for user",
		zap.Uint("user_id", userID),
		zap.Int64("timeout_ms", timeout.Milliseconds()))

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout.Duration())
	defer cancel()

	return lm.acquireWithSetup(ctx, userID, &timeoutCtx)
}

// TryAcquireLock attempts to acquire a lock for the specified user ID without blocking.
// If the lock is immediately available, it returns the Lock handle.
// Otherwise, it returns ErrLockBusy.
// For userID 0 (anonymous operations), returns a no-op lock immediately.
func (lm *LockManagerDefault) TryAcquireLock(ctx context.Context, userID uint) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.TryAcquireLock")
	defer span.End()

	// Return no-op lock for anonymous operations (userID == 0)
	if userID == 0 {
		return &noOpLock{}, nil
	}

	lm.logger.Debug("Trying to acquire lock for user", zap.Uint("user_id", userID))

	// Get or create the user lock
	ul := lm.getUserLock(userID)
	atomic.AddInt64(&ul.waiters, 1)

	// Try to lock without blocking
	if ul.mu.TryLock() {
		lm.logger.Debug("Lock acquired for user", zap.Uint("user_id", userID))
		return &defaultLock{
			mu:        &ul.mu,
			onRelease: func() { lm.cleanupLock(userID) },
		}, nil
	}

	// Decrement waiters since we didn't acquire the lock
	lm.mu.Lock()
	defer lm.mu.Unlock()
	// Clean up if no waiters remain
	if atomic.AddInt64(&ul.waiters, -1) <= 0 {
		delete(lm.locks, userID)
	}

	lm.logger.Debug("Lock is busy for user", zap.Uint("user_id", userID))
	return nil, ErrLockBusy
}

// getUserLock gets or creates a lock for the specified user ID.
// This is safe for concurrent access.
func (lm *LockManagerDefault) getUserLock(userID uint) *userLock {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Check if lock already exists
	if ul, exists := lm.locks[userID]; exists {
		return ul
	}

	// Create a new lock if it doesn't exist
	ul := &userLock{}
	lm.locks[userID] = ul
	return ul
}

// cleanupLock removes the lock entry if there are no more waiters.
// This prevents the locks map from growing indefinitely.
func (lm *LockManagerDefault) cleanupLock(userID uint) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	ul, exists := lm.locks[userID]
	if !exists {
		return
	}

	// Only remove if there are no waiters
	if atomic.AddInt64(&ul.waiters, -1) <= 0 {
		delete(lm.locks, userID)
	}
}

// defaultLock implements the Lock interface with a cleanup callback.
type defaultLock struct {
	mu        *sync.Mutex
	released  int32
	onRelease func()
}

// Release releases the lock.
// Multiple calls to Release are safe - subsequent calls are no-ops.
func (l *defaultLock) Release() {
	if !atomic.CompareAndSwapInt32(&l.released, 0, 1) {
		return
	}

	l.mu.Unlock()

	if l.onRelease != nil {
		l.onRelease()
	}
}
