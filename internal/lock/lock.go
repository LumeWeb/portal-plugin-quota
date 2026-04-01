package lock

import (
	"context"
	"errors"
	"sync"
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
	waiters int32
}

// NewLockManager creates a new LockManager instance.
func NewLockManager(ctx core.Context) LockManager {
	return &LockManagerDefault{
		ctx:    ctx,
		logger: ctx.Logger(),
		locks:  make(map[uint]*userLock),
	}
}

// validateUserID validates the user ID and returns an error if invalid.
func (lm *LockManagerDefault) validateUserID(userID uint) error {
	if userID == 0 {
		lm.logger.Error("Cannot acquire lock for invalid user ID", zap.Uint("user_id", userID))
		return errors.New("invalid user ID: user ID must be greater than 0")
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
	ul.waiters++

	// Choose the context for acquisition
	acquireCtx := ctx
	if timeoutCtx != nil {
		acquireCtx = *timeoutCtx
	}

	lock, err := lm.acquireLock(acquireCtx, ul)
	if err != nil {
		// Decrement waiters and cleanup on failure
		lm.mu.Lock()
		ul.waiters--
		if ul.waiters <= 0 {
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

// AcquireLock acquires a lock for the specified user ID.
// If the lock is already held, this blocks until it becomes available
// or the context is canceled.
func (lm *LockManagerDefault) AcquireLock(ctx context.Context, userID uint) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.AcquireLock")
	defer span.End()

	if err := lm.validateUserID(userID); err != nil {
		return nil, err
	}

	lm.logger.Debug("Acquiring lock for user", zap.Uint("user_id", userID))

	return lm.acquireWithSetup(ctx, userID, nil)
}

// AcquireLockWithTimeout attempts to acquire a lock for the specified user ID
// within the given timeout. If the timeout is exceeded, it returns ErrLockTimeout.
func (lm *LockManagerDefault) AcquireLockWithTimeout(ctx context.Context, userID uint, timeout Timeout) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.AcquireLockWithTimeout")
	defer span.End()

	if err := lm.validateUserID(userID); err != nil {
		return nil, err
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
func (lm *LockManagerDefault) TryAcquireLock(ctx context.Context, userID uint) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.TryAcquireLock")
	defer span.End()

	if err := lm.validateUserID(userID); err != nil {
		return nil, err
	}

	lm.logger.Debug("Trying to acquire lock for user", zap.Uint("user_id", userID))

	// Get or create the user lock
	ul := lm.getUserLock(userID)
	ul.waiters++

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
	ul.waiters--
	// Clean up if no waiters remain
	if ul.waiters <= 0 {
		delete(lm.locks, userID)
	}

	lm.logger.Debug("Lock is busy for user", zap.Uint("user_id", userID))
	return nil, ErrLockBusy
}

// getUserLock gets or creates a lock for the specified user ID.
// This is safe for concurrent access.
func (lm *LockManagerDefault) getUserLock(userID uint) *userLock {
	lm.mu.RLock()
	ul, exists := lm.locks[userID]
	lm.mu.RUnlock()

	if exists {
		return ul
	}

	// Create a new lock if it doesn't exist
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Double-check after acquiring write lock
	if ul, exists := lm.locks[userID]; exists {
		return ul
	}

	ul = &userLock{}
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

	ul.waiters--

	// Only remove if there are no waiters
	if ul.waiters <= 0 {
		delete(lm.locks, userID)
	}
}

// defaultLock implements the Lock interface with a cleanup callback.
type defaultLock struct {
	mu        *sync.Mutex
	released  bool
	onRelease func()
}

// Release releases the lock.
// Multiple calls to Release are safe - subsequent calls are no-ops.
func (l *defaultLock) Release() {
	if l.released {
		return
	}

	l.mu.Unlock()
	l.released = true

	if l.onRelease != nil {
		l.onRelease()
	}
}
