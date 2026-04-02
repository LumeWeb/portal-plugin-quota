package reservation

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// setupTestManager creates a new ReservationManager for testing
func setupTestManager(t *testing.T) pluginCore.ReservationManager {
	ctx, _ := coreTesting.NewTestContext(t)
	return NewReservationManager(ctx)
}

// TestNewReservationManager tests the factory functions
func TestNewReservationManager(t *testing.T) {
	t.Run("creates manager with default cleanup age", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		manager := NewReservationManager(ctx)
		assert.NotNil(t, manager)

		// Verify it creates a ReservationManagerDefault
		rm, ok := manager.(*ReservationManagerDefault)
		assert.True(t, ok, "Expected ReservationManagerDefault type")
		assert.NotNil(t, rm)
		assert.NotNil(t, rm.reservations)
		assert.NotNil(t, rm.userReservations)
	})
}

func TestNewReservationManagerWithCleanup(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	t.Run("creates manager with custom cleanup age", func(t *testing.T) {
		cleanupAge := 10 * time.Minute
		manager := NewReservationManagerWithCleanup(ctx, cleanupAge)
		assert.NotNil(t, manager)

		rm, ok := manager.(*ReservationManagerDefault)
		assert.True(t, ok)
		assert.NotNil(t, rm)
		assert.Equal(t, cleanupAge, rm.cleanupAge)
	})
}

func TestReservationManagerDefault_Reserve(t *testing.T) {
	ctx := context.Background()

	t.Run("creates reservation successfully", func(t *testing.T) {
		manager := setupTestManager(t)
		reservation, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)
		assert.NotNil(t, reservation)
		assert.NotEmpty(t, reservation.UUID())
		assert.Equal(t, uint(1), reservation.UserID())
		assert.Equal(t, pluginCore.UsageTypeUpload, reservation.UsageType())
		assert.Equal(t, int64(1000), reservation.Bytes())
	})

	t.Run("fails with invalid user ID (zero)", func(t *testing.T) {
		manager := setupTestManager(t)
		reservation, err := manager.Reserve(ctx, 0, pluginCore.UsageTypeUpload, 1000)
		assert.Error(t, err)
		assert.Nil(t, reservation)
		assert.Contains(t, err.Error(), "invalid user ID")
	})

	t.Run("fails with invalid bytes (zero)", func(t *testing.T) {
		manager := setupTestManager(t)
		reservation, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 0)
		assert.Error(t, err)
		assert.Nil(t, reservation)
		assert.Contains(t, err.Error(), "invalid bytes")
	})

	t.Run("fails with negative bytes", func(t *testing.T) {
		manager := setupTestManager(t)
		reservation, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, -100)
		assert.Error(t, err)
		assert.Nil(t, reservation)
		assert.Contains(t, err.Error(), "invalid bytes")
	})

	t.Run("creates unique UUIDs for multiple reservations", func(t *testing.T) {
		manager := setupTestManager(t)
		res1, _ := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		res2, _ := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 2000)

		assert.NotEqual(t, res1.UUID(), res2.UUID(), "Each reservation should have a unique UUID")
	})

	t.Run("creates reservations for different users", func(t *testing.T) {
		manager := setupTestManager(t)
		res1, _ := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		res2, _ := manager.Reserve(ctx, 2, pluginCore.UsageTypeUpload, 2000)

		assert.Equal(t, uint(1), res1.UserID())
		assert.Equal(t, uint(2), res2.UserID())
	})

	t.Run("creates reservations for different usage types", func(t *testing.T) {
		manager := setupTestManager(t)
		res1, _ := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		res2, _ := manager.Reserve(ctx, 1, pluginCore.UsageTypeDownload, 2000)

		assert.Equal(t, pluginCore.UsageTypeUpload, res1.UsageType())
		assert.Equal(t, pluginCore.UsageTypeDownload, res2.UsageType())
	})
}

func TestReservationManagerDefault_GetReservation(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieves existing reservation", func(t *testing.T) {
		manager := setupTestManager(t)
		res1, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)

		res2 := manager.GetReservation(res1.UUID())
		assert.NotNil(t, res2)
		assert.Equal(t, res1.UUID(), res2.UUID())
		assert.Equal(t, res1.UserID(), res2.UserID())
		assert.Equal(t, res1.Bytes(), res2.Bytes())
	})

	t.Run("returns nil for non-existent UUID", func(t *testing.T) {
		manager := setupTestManager(t)
		res := manager.GetReservation("non-existent-uuid")
		assert.Nil(t, res)
	})

	t.Run("returns nil for released reservation", func(t *testing.T) {
		manager := setupTestManager(t)
		res1, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)

		res1.Release()

		res2 := manager.GetReservation(res1.UUID())
		assert.Nil(t, res2, "Released reservation should not be retrievable")
	})

	t.Run("returns nil for empty UUID", func(t *testing.T) {
		manager := setupTestManager(t)
		res := manager.GetReservation("")
		assert.Nil(t, res)
	})
}

func TestReservationManagerDefault_SumPendingBytesForUser(t *testing.T) {
	ctx := context.Background()

	t.Run("returns zero for user with no reservations", func(t *testing.T) {
		manager := setupTestManager(t)
		total := manager.SumPendingBytesForUser(ctx, 999, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(0), total)
	})

	t.Run("sums bytes for single reservation", func(t *testing.T) {
		manager := setupTestManager(t)
		_, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)

		total := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(1000), total)
	})

	t.Run("sums bytes for multiple reservations of same type", func(t *testing.T) {
		manager := setupTestManager(t)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 2000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 3000)

		total := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(6000), total)
	})

	t.Run("filters by usage type", func(t *testing.T) {
		manager := setupTestManager(t)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 2000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeDownload, 500)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeDownload, 1500)

		uploadTotal := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(3000), uploadTotal)

		downloadTotal := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeDownload)
		assert.Equal(t, int64(2000), downloadTotal)
	})

	t.Run("excludes released reservations", func(t *testing.T) {
		manager := setupTestManager(t)
		res1, _ := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 2000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 3000)

		// Release one reservation
		res1.Release()

		total := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(5000), total, "Released reservation should be excluded from sum")
	})

	t.Run("separates reservations by user", func(t *testing.T) {
		manager := setupTestManager(t)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 2000)
		_, _ = manager.Reserve(ctx, 2, pluginCore.UsageTypeUpload, 500)

		user1Total := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(3000), user1Total)

		user2Total := manager.SumPendingBytesForUser(ctx, 2, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(500), user2Total)
	})

	t.Run("handles mixed usage types across users", func(t *testing.T) {
		manager := setupTestManager(t)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		_, _ = manager.Reserve(ctx, 1, pluginCore.UsageTypeDownload, 2000)
		_, _ = manager.Reserve(ctx, 2, pluginCore.UsageTypeUpload, 500)
		_, _ = manager.Reserve(ctx, 2, pluginCore.UsageTypeDownload, 1500)

		user1UploadTotal := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(1000), user1UploadTotal)

		user1DownloadTotal := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeDownload)
		assert.Equal(t, int64(2000), user1DownloadTotal)

		user2UploadTotal := manager.SumPendingBytesForUser(ctx, 2, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(500), user2UploadTotal)

		user2DownloadTotal := manager.SumPendingBytesForUser(ctx, 2, pluginCore.UsageTypeDownload)
		assert.Equal(t, int64(1500), user2DownloadTotal)
	})
}

func TestDefaultReservation_Release(t *testing.T) {
	ctx := context.Background()

	t.Run("releases reservation successfully", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)

		uuid := res.UUID()

		// Verify reservation exists before release
		r1 := manager.GetReservation(uuid)
		assert.NotNil(t, r1)

		// Release the reservation
		res.Release()

		// Verify reservation no longer exists after release
		r2 := manager.GetReservation(uuid)
		assert.Nil(t, r2)
	})

	t.Run("release is idempotent", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)

		// Release multiple times - should not panic
		res.Release()
		res.Release()
		res.Release()

		// Should still not exist
		r := manager.GetReservation(res.UUID())
		assert.Nil(t, r)
	})

	t.Run("released reservation is excluded from sum", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)

		// Verify it's counted
		total := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(1000), total)

		// Release
		res.Release()

		// Verify it's no longer counted
		total = manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(0), total)
	})
}

func TestDefaultReservation_Getters(t *testing.T) {
	ctx := context.Background()

	t.Run("UUID returns the reservation UUID", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)
		assert.NotEmpty(t, res.UUID())
	})

	t.Run("UserID returns the reservation user ID", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 123, pluginCore.UsageTypeUpload, 1000)
		require.NoError(t, err)
		assert.Equal(t, uint(123), res.UserID())
	})

	t.Run("UsageType returns the reservation usage type", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeDownload, 1000)
		require.NoError(t, err)
		assert.Equal(t, pluginCore.UsageTypeDownload, res.UsageType())
	})

	t.Run("Bytes returns the reservation bytes", func(t *testing.T) {
		manager := setupTestManager(t)
		res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 5678)
		require.NoError(t, err)
		assert.Equal(t, int64(5678), res.Bytes())
	})
}

func TestReservationManagerDefault_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent reservations from multiple goroutines", func(t *testing.T) {
		manager := setupTestManager(t)
		const numGoroutines = 100
		const reservationsPerGoroutine = 10

		var wg sync.WaitGroup
		var errors atomic.Int32

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			// Skip userID 0 as it's invalid - start from 1
			userID := uint(i + 1)
			go func(userID uint) {
				defer wg.Done()
				for j := 0; j < reservationsPerGoroutine; j++ {
					_, err := manager.Reserve(ctx, userID, pluginCore.UsageTypeUpload, 100)
					if err != nil {
						errors.Add(1)
					}
				}
			}(userID)
		}

		wg.Wait()

		assert.Equal(t, int32(0), errors.Load())
	})

	t.Run("concurrent releases lookups", func(t *testing.T) {
		manager := setupTestManager(t)
		const numReservations = 100

		// Create reservations
		var reservations []pluginCore.Reservation
		for i := 0; i < numReservations; i++ {
			res, err := manager.Reserve(ctx, 1, pluginCore.UsageTypeUpload, 100)
			require.NoError(t, err)
			reservations = append(reservations, res)
		}

		var wg sync.WaitGroup

		// Concurrently release and look up reservations
		for i := 0; i < numReservations; i++ {
			wg.Add(1)
			go func(res pluginCore.Reservation) {
				defer wg.Done()
				// Lookup
				_ = manager.GetReservation(res.UUID())
				// Release
				res.Release()
				// Lookup again (should be nil after some time, but might be in cleanup queue)
				_ = manager.GetReservation(res.UUID())
			}(reservations[i])
		}

		wg.Wait()

		// Give time for cleanup to complete
		time.Sleep(10 * time.Millisecond)

		// All reservations should be released
		var unreleased int
		for _, res := range reservations {
			r := manager.GetReservation(res.UUID())
			if r != nil {
				unreleased++
			}
		}
		assert.Equal(t, 0, unreleased, "All reservations should be released and cleaned up")
	})

	t.Run("concurrent sum calculations with reservations and releases", func(t *testing.T) {
		manager := setupTestManager(t)
		const numReservations = 50

		// Two wait groups: one for reservations, one for sum checker
		var sumWg sync.WaitGroup
		var resWg sync.WaitGroup

		// Use a context for cancelling the sum checker
		sumCtx, sumCancel := context.WithCancel(context.Background())

		// Start goroutine that constantly checks sum
		var sums []int64
		var sumMu sync.Mutex
		var sumCount atomic.Int32

		sumWg.Add(1)
		go func() {
			defer sumWg.Done()
			for {
				select {
				case <-sumCtx.Done():
					return
				default:
					total := manager.SumPendingBytesForUser(sumCtx, 1, pluginCore.UsageTypeUpload)
					sumMu.Lock()
					sums = append(sums, total)
					sumMu.Unlock()
					sumCount.Add(1)
				}
			}
		}()

		// Concurrently make reservations and releases
		for i := 0; i < numReservations; i++ {
			resWg.Add(1)
			go func() {
				defer resWg.Done()
				res, err := manager.Reserve(sumCtx, 1, pluginCore.UsageTypeUpload, 100)
				if err == nil {
					time.Sleep(1 * time.Millisecond)
					res.Release()
				}
			}()
		}

		// Wait for all reservation goroutines to complete
		resWg.Wait()

		// Stop the sum-checking goroutine and wait for it to finish
		sumCancel()
		sumWg.Wait()

		// Verify we got multiple sum readings while operations were ongoing
		assert.Greater(t, sumCount.Load(), int32(10), "Should have captured multiple sum readings during operations")

		// Final sum should be zero (all released)
		finalSum := manager.SumPendingBytesForUser(ctx, 1, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(0), finalSum)
	})
}

func TestReservationManagerDefault_AllUsageTypes(t *testing.T) {
	ctx := context.Background()

	usageTypes := []pluginCore.UsageType{
		pluginCore.UsageTypeUpload,
		pluginCore.UsageTypeDownload,
		pluginCore.UsageTypeStorageAdd,
		pluginCore.UsageTypeStorageRemove,
	}

	t.Run("handles all usage types", func(t *testing.T) {
		manager := setupTestManager(t)

		for _, usageType := range usageTypes {
			res, err := manager.Reserve(ctx, 1, usageType, 1000)
			require.NoError(t, err, "Failed to create reservation for usage type: %v", usageType)

			// Verify each usage type sums correctly
			total := manager.SumPendingBytesForUser(ctx, 1, usageType)
			assert.Equal(t, int64(1000), total, "Wrong total for usage type: %v", usageType)

			res.Release()
		}
	})
}
