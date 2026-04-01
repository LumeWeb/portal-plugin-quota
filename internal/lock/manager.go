package lock

import (
	"context"
	"time"
)

// LockManager provides locking capabilities for quota operations to prevent
// race conditions and the thundering herd problem when multiple requests check
// or modify quota for the same user concurrently.
//
// Locks are keyed by user ID, ensuring that operations on the same user's quota
// are serialized, while operations on different users can proceed in parallel.
//
// This interface is designed for both in-memory and distributed locking implementations,
// enabling future clustering support through implementations that use Redis, etcd,
// or other distributed lock systems.
//
// LockManager is an internal component used by QuotaService to synchronize
// access to quota data.
type LockManager interface {
	// AcquireLock acquires a lock for the specified user ID.
	// If the lock is already held by another caller, this blocks until the lock
	// becomes available or the context is canceled.
	//
	// The returned Lock must be released by calling Release().
	// It is recommended to use defer to ensure the lock is always released:
	//
	//	lock, err := lockManager.AcquireLock(ctx, userID)
	//	if err != nil {
	//	    return err
	//	}
	//	defer lock.Release()
	//
	//	// Critical section - quota operations
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout
	//   - userID: User ID to lock quota operations for
	//
	// Returns:
	//   - Lock: Handle that must be released when done
	//   - error: Error if lock acquisition fails (e.g., context canceled)
	AcquireLock(ctx context.Context, userID uint) (Lock, error)

	// AcquireLockWithTimeout attempts to acquire a lock for the specified user ID.
	// If the lock cannot be acquired within the timeout, it returns an error.
	//
	// This is useful for preventing indefinite blocking when lock contention is high.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - userID: User ID to lock quota operations for
	//   - timeout: Maximum time to wait for the lock
	//
	// Returns:
	//   - Lock: Handle that must be released when done
	//   - error: Error if timeout exceeded or context canceled
	AcquireLockWithTimeout(ctx context.Context, userID uint, timeout Timeout) (Lock, error)

	// TryAcquireLock attempts to acquire a lock for the specified user ID without blocking.
	// If the lock is immediately available, it returns the Lock handle.
	// Otherwise, it returns ErrLockBusy.
	//
	// This is useful for non-blocking operations where you want to check
	// if a lock is available without waiting.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - userID: User ID to lock quota operations for
	//
	// Returns:
	//   - Lock: Handle that must be released when done (if immediately available)
	//   - error: ErrLockBusy if lock is held, or other errors
	TryAcquireLock(ctx context.Context, userID uint) (Lock, error)
}

// Lock represents a held lock that must be released.
// The Lock handle ensures that the lock is properly released
// and prevents accidental lock leakage.
type Lock interface {
	// Release releases the lock, allowing other waiters to acquire it.
	// Calling Release multiple times is safe - subsequent calls are no-ops.
	// It is recommended to use defer to ensure the lock is always released.
	Release()
}

// Timeout represents a duration for lock acquisition attempts.
// This type provides type safety for timeout values.
type Timeout struct {
	d int64
}

// NewTimeout creates a new Timeout from an int64 duration in milliseconds.
func NewTimeout(duration int64) Timeout {
	return Timeout{d: duration}
}

// Milliseconds returns the timeout duration in milliseconds.
func (t Timeout) Milliseconds() int64 {
	return t.d
}

// Duration returns the timeout as a time.Duration.
func (t Timeout) Duration() time.Duration {
	return time.Duration(t.d) * time.Millisecond
}

// Common timeout values for lock acquisition.
var (
	// DefaultLockTimeout is the default timeout for lock acquisition.
	DefaultLockTimeout = NewTimeout(5000) // 5 seconds

	// ShortLockTimeout is a shorter timeout for fast-failing operations.
	ShortLockTimeout = NewTimeout(100) // 100ms

	// LongLockTimeout is a longer timeout for operations that may take more time.
	LongLockTimeout = NewTimeout(30000) // 30 seconds
)

// Lock-related errors.
var (
	// ErrLockBusy is returned when TryAcquireLock fails because the lock is held.
	ErrLockBusy = NewLockBusyError()

	// ErrLockTimeout is returned when AcquireLockWithTimeout exceeds the specified timeout.
	ErrLockTimeout = NewLockTimeoutError()
)

// lockBusyError is returned when a lock is already held by another caller.
type lockBusyError struct{}

func (e *lockBusyError) Error() string {
	return "lock is busy and cannot be acquired"
}

// NewLockBusyError creates a new lock busy error.
func NewLockBusyError() error {
	return &lockBusyError{}
}

// lockTimeoutError is returned when a lock acquisition attempt times out.
type lockTimeoutError struct{}

func (e *lockTimeoutError) Error() string {
	return "lock acquisition timeout"
}

// NewLockTimeoutError creates a new lock timeout error.
func NewLockTimeoutError() error {
	return &lockTimeoutError{}
}
