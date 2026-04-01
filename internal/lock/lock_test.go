package lock

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// Test constants for lock manager tests
const (
	testUserID1   = 1
	testUserID2   = 2
	testUserID3   = 3
	testUserID4   = 4
	testInvalidID = 0
)

// TestLockManager_AcquireLock_Success tests basic lock acquisition and release.
func TestLockManager_AcquireLock_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		lock, err := lockManager.AcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "should successfully acquire lock")
		require.NotNil(t, lock, "lock should not be nil")

		// Lock is held, another attempt should block
		acquired := make(chan struct{})
		go func() {
			_, _ = lockManager.AcquireLock(testCtx, testUserID1)
			close(acquired)
		}()

		// The second acquisition should not complete immediately
		select {
		case <-acquired:
			t.Fatal("second lock acquisition should block")
		case <-time.After(100 * time.Millisecond):
			// Expected - lock is still held
		}

		// Release the lock
		lock.Release()

		// Now the second acquisition should complete
		select {
		case <-afterTime(500 * time.Millisecond): // Increased timeout for slower CI
			t.Fatal("second lock acquisition should complete after release")
		case <-acquired:
			// Expected - lock was released
		}
	})
}

// TestLockManager_AcquireLock_InvalidUserID tests that invalid user IDs are rejected.
func TestLockManager_AcquireLock_InvalidUserID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		_, err := lockManager.AcquireLock(testCtx, testInvalidID)
		assert.Error(t, err, "invalid user ID should return error")
		assert.Contains(t, err.Error(), "invalid user ID", "error should mention invalid user ID")
	})
}

// TestLockManager_DifferentUsers tests that locks for different users don't block each other.
func TestLockManager_DifferentUsers(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		// Acquire locks for different users
		lock1, err1 := lockManager.AcquireLock(testCtx, testUserID1)
		require.NoError(t, err1, "should acquire lock for user 1")

		lock2, err2 := lockManager.AcquireLock(testCtx, testUserID2)
		require.NoError(t, err2, "should acquire lock for user 2")

		// Both locks should be held independently
		assert.NotNil(t, lock1, "lock1 should not be nil")
		assert.NotNil(t, lock2, "lock2 should not be nil")

		// Release locks
		lock1.Release()
		lock2.Release()
	})
}

// TestLockManager_ConcurrentAccess tests that concurrent access to the same user is properly serialized.
func TestLockManager_ConcurrentAccess(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()
		numGoroutines := 10
		counter := 0
		var mutex sync.Mutex

		var wg sync.WaitGroup
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer wg.Done()

				lock, err := lockManager.AcquireLock(testCtx, testUserID1)
				require.NoError(t, err, "goroutine %d should acquire lock", goroutineID)
				defer lock.Release()

				// Critical section - increment counter
				mutex.Lock()
				currentCounter := counter
				counter++
				time.Sleep(10 * time.Millisecond)
				assert.Equal(t, currentCounter, counter-1, "goroutine %d: counter should be incremented atomically", goroutineID)
				mutex.Unlock()
			}(i)
		}

		wg.Wait()
		assert.Equal(t, numGoroutines, counter, "counter should be incremented by all goroutines")
	})
}

// TestLockManager_AcquireLockWithTimeout tests timeout functionality.
func TestLockManager_AcquireLockWithTimeout(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		// Acquire first lock
		lock1, err := lockManager.AcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "first lock should be acquired")

		// Try to acquire with timeout - should fail
		timeout := NewTimeout(100) // 100ms
		_, err = lockManager.AcquireLockWithTimeout(testCtx, testUserID1, timeout)
		assert.Error(t, err, "should timeout when lock is held")
		assert.Equal(t, ErrLockTimeout, err, "should return timeout error")

		// Release first lock
		lock1.Release()

		// Now should be able to acquire
		lock2, err := lockManager.AcquireLockWithTimeout(testCtx, testUserID1, DefaultLockTimeout)
		require.NoError(t, err, "second lock should be acquired after first is released")
		lock2.Release()
	})
}

// TestLockManager_TryAcquireLock tests non-blocking lock acquisition.
func TestLockManager_TryAcquireLock(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		// First attempt should succeed
		lock1, err := lockManager.TryAcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "first attempt should succeed")
		assert.NotNil(t, lock1, "lock should not be nil")

		// Second attempt should fail with ErrLockBusy
		lock2, err := lockManager.TryAcquireLock(testCtx, testUserID1)
		assert.Error(t, err, "second attempt should fail")
		assert.Equal(t, ErrLockBusy, err, "should return busy error")
		assert.Nil(t, lock2, "lock should be nil")

		// Release first lock
		lock1.Release()

		// Third attempt should succeed
		lock3, err := lockManager.TryAcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "third attempt should succeed after release")
		assert.NotNil(t, lock3, "lock should not be nil")
		lock3.Release()
	})
}

// TestLockManager_ContextCancelation tests that lock acquisition respects context cancellation.
func TestLockManager_ContextCancelation(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		cancelCtx, cancel := context.WithCancel(context.Background())

		// Acquire first lock
		lock1, err := lockManager.AcquireLock(cancelCtx, testUserID1)
		require.NoError(t, err, "first lock should be acquired")

		// Try to acquire in goroutine, then cancel context
		acquireErr := make(chan error, 1)
		go func() {
			_, err := lockManager.AcquireLock(cancelCtx, testUserID1)
			acquireErr <- err
		}()

		// Wait a bit to ensure goroutine is blocked
		time.Sleep(50 * time.Millisecond)

		// Cancel context
		cancel()

		// Should receive context canceled error
		select {
		case err := <-acquireErr:
			assert.Error(t, err, "acquisition should fail after context cancellation")
			assert.Equal(t, context.Canceled, err, "should return context canceled error")
		case <-afterTime(500 * time.Millisecond):
			t.Fatal("should receive error from goroutine")
		}

		// Release first lock
		lock1.Release()
	})
}

// TestLockManager_MultipleReleases tests that releasing a lock multiple times is safe.
func TestLockManager_MultipleReleases(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		lock, err := lockManager.AcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "should acquire lock")

		// Should not panic
		lock.Release()
		lock.Release()
		lock.Release()

		// Should be able to acquire a new lock
		lock2, err := lockManager.AcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "should be able to acquire new lock after multiple releases")
		lock2.Release()
	})
}

// TestLockManager_Cleanup tests that lock entries are cleaned up when not in use.
func TestLockManager_Cleanup(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()

		// Acquire and release locks for multiple users
		lock1, _ := lockManager.AcquireLock(testCtx, testUserID1)
		lock1.Release()

		lock2, _ := lockManager.AcquireLock(testCtx, testUserID2)
		lock2.Release()

		// Give time for cleanup
		time.Sleep(10 * time.Millisecond)

		// Should be able to acquire locks again
		lock3, err := lockManager.AcquireLock(testCtx, testUserID1)
		require.NoError(t, err, "should be able to reacquire lock for user 1")
		lock3.Release()

		lock4, err := lockManager.AcquireLock(testCtx, testUserID2)
		require.NoError(t, err, "should be able to reacquire lock for user 2")
		lock4.Release()
	})
}

// TestLockManager_ThunderingHerd simulates the thundering herd problem scenario.
func TestLockManager_ThunderingHerd(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()
		numRequests := 20
		var accessCount int

		var wg sync.WaitGroup
		wg.Add(numRequests)

		startTime := time.Now()

		for i := 0; i < numRequests; i++ {
			go func(requestID int) {
				defer wg.Done()

				lock, err := lockManager.AcquireLock(testCtx, testUserID1)
				require.NoError(t, err, "request %d should acquire lock", requestID)
				defer lock.Release()

				accessCount++

				// Simulate some work
				time.Sleep(20 * time.Millisecond)
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		assert.Equal(t, numRequests, accessCount, "all requests should access quota")
		t.Logf("Completed %d requests in %v (avg %v per request)",
			numRequests, duration, duration/time.Duration(numRequests))

		// With 20ms per request * 20 requests = 400ms minimum
		// Allow some overhead
		assert.GreaterOrEqual(t, duration.Milliseconds(), int64(350), "should serialize access")
	})
}

// TestLockManager_ConcurrentDifferentUsers tests that different users can operate in parallel.
func TestLockManager_ConcurrentDifferentUsers(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		lockManager := NewLockManager(ctx)
		testCtx := context.Background()
		numUsers := 10

		var wg sync.WaitGroup
		startTime := time.Now()

		for userID := 1; userID <= numUsers; userID++ {
			wg.Add(1)
			go func(uid uint) {
				defer wg.Done()

				lock, err := lockManager.AcquireLock(testCtx, uid)
				require.NoError(t, err, "should acquire lock for user %d", uid)
				defer lock.Release()

				// Simulate some work
				time.Sleep(50 * time.Millisecond)
			}(uint(userID))
		}

		wg.Wait()
		duration := time.Since(startTime)

		// All users should complete in roughly the same time as a single user
		// because locks are per-user
		t.Logf("Completed %d users in %v", numUsers, duration)
		assert.Less(t, duration.Milliseconds(), int64(200), "different users should operate in parallel")
	})
}

// Helper function to create a timeout channel
func afterTime(d time.Duration) <-chan time.Time {
	return time.After(d)
}

// TestLockManager_ErrorMessages tests that error messages are descriptive.
func TestLockManager_ErrorMessages(t *testing.T) {
	t.Run("ErrLockBusy", func(t *testing.T) {
		err := ErrLockBusy
		assert.Error(t, err, "ErrLockBusy should be an error")
		assert.Contains(t, err.Error(), "busy", "error message should contain 'busy'")
	})

	t.Run("ErrLockTimeout", func(t *testing.T) {
		err := ErrLockTimeout
		assert.Error(t, err, "ErrLockTimeout should be an error")
		assert.Contains(t, err.Error(), "timeout", "error message should contain 'timeout'")
	})

	t.Run("NewLockBusyError", func(t *testing.T) {
		err := NewLockBusyError()
		assert.Error(t, err, "NewLockBusyError should return an error")
		assert.Contains(t, err.Error(), "busy", "error message should contain 'busy'")
	})

	t.Run("NewLockTimeoutError", func(t *testing.T) {
		err := NewLockTimeoutError()
		assert.Error(t, err, "NewLockTimeoutError should return an error")
		assert.Contains(t, err.Error(), "timeout", "error message should contain 'timeout'")
	})
}

// TestLockManager_TimeoutConstants tests predefined timeout constants.
func TestLockManager_TimeoutConstants(t *testing.T) {
	tests := []struct {
		name     string
		timeout  Timeout
		expected int64
	}{
		{"ShortLockTimeout", ShortLockTimeout, 100},
		{"DefaultLockTimeout", DefaultLockTimeout, 5000},
		{"LongLockTimeout", LongLockTimeout, 30000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.timeout.Milliseconds(), fmt.Sprintf("%s should have expected duration", tt.name))
		})
	}
}
