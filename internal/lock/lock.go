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

// AcquireLock acquires a lock for the specified user ID.
// If the lock is already held, this blocks until it becomes available
// or the context is canceled.
func (lm *LockManagerDefault) AcquireLock(ctx context.Context, userID uint) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.AcquireLock")
	defer span.End()

	if userID == 0 {
		lm.logger.Error("Cannot acquire lock for invalid user ID", zap.Uint("user_id", userID))
		return nil, errors.New("invalid user ID: user ID must be greater than 0")
	}

	lm.logger.Debug("Acquiring lock for user", zap.Uint("user_id", userID))

	// Get or create the user lock
	ul := lm.getUserLock(userID)
	ul.waiters++

	// Wait for the lock or context cancellation
	done := make(chan struct{})

	go func() {
		ul.mu.Lock()
		close(done)
	}()

	select {
	case <-done:
		ul.waiters--
		lm.logger.Debug("Lock acquired for user", zap.Uint("user_id", userID))
		return &defaultLock{
			mu:    &ul.mu,
			onRelease: func() {
				lm.cleanupLock(userID)
			},
		}, nil
	case <-ctx.Done():
		ul.waiters--
		return nil, ctx.Err()
	}
}

// AcquireLockWithTimeout attempts to acquire a lock for the specified user ID
// within the given timeout. If the timeout is exceeded, it returns ErrLockTimeout.
func (lm *LockManagerDefault) AcquireLockWithTimeout(ctx context.Context, userID uint, timeout Timeout) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.AcquireLockWithTimeout")
	defer span.End()

	if userID == 0 {
		lm.logger.Error("Cannot acquire lock for invalid user ID", zap.Uint("user_id", userID))
		return nil, errors.New("invalid user ID: user ID must be greater than 0")
	}

	lm.logger.Debug("Acquiring lock with timeout for user",
		zap.Uint("user_id", userID),
		zap.Int64("timeout_ms", timeout.Milliseconds()))

	// Create a timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout.Milliseconds())*time.Millisecond)
	defer cancel()

	// Get or create the user lock
	ul := lm.getUserLock(userID)
	ul.waiters++

	// Wait for the lock, timeout, or context cancellation
	done := make(chan struct{})
	go func() {
		ul.mu.Lock()
		close(done)
	}()

	select {
	case <-done:
		ul.waiters--
		lm.logger.Debug("Lock acquired for user", zap.Uint("user_id", userID))
		return &defaultLock{
			mu:    &ul.mu,
			onRelease: func() {
				lm.cleanupLock(userID)
			},
		}, nil
	case <-timeoutCtx.Done():
		ul.waiters--
		return nil, ErrLockTimeout
	case <-ctx.Done():
		ul.waiters--
		return nil, ctx.Err()
	}
}

// TryAcquireLock attempts to acquire a lock for the specified user ID without blocking.
// If the lock is immediately available, it returns the Lock handle.
// Otherwise, it returns ErrLockBusy.
func (lm *LockManagerDefault) TryAcquireLock(ctx context.Context, userID uint) (Lock, error) {
	ctx, span := core.TraceMethod(ctx, "LockManagerDefault.TryAcquireLock")
	defer span.End()

	if userID == 0 {
		lm.logger.Error("Cannot acquire lock for invalid user ID", zap.Uint("user_id", userID))
		return nil, errors.New("invalid user ID: user ID must be greater than 0")
	}

	lm.logger.Debug("Trying to acquire lock for user", zap.Uint("user_id", userID))

	// Get or create the user lock
	ul := lm.getUserLock(userID)

	// Try to lock without blocking
	if ul.mu.TryLock() {
		ul.waiters++
		lm.logger.Debug("Lock acquired for user", zap.Uint("user_id", userID))
		return &defaultLock{
			mu:    &ul.mu,
			onRelease: func() {
				lm.cleanupLock(userID)
			},
		}, nil
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
