package managers

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// Test constants
const (
	testReservationBytes = 1000
	testIPAddress        = "192.168.1.1"
)

func TestNewReservationManager_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		manager := NewReservationManager(ctx)
		assert.NotNil(t, manager)
	}, pluginTesting.TestOptions())
}

func TestReservationManager_CreateReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)
		assert.NotNil(t, reservation)
		assert.Equal(t, userID, reservation.UserID)
		assert.Equal(t, pluginModels.UsageTypeUpload, reservation.Type)
		assert.Equal(t, uint64(testReservationBytes), reservation.Bytes)
		assert.Equal(t, testIPAddress, reservation.SourceIP)
		assert.Equal(t, pluginModels.ReservationStatusPending, reservation.Status)
		assert.NotNil(t, reservation.CreatedAt)
		assert.NotNil(t, reservation.UpdatedAt)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_CreateReservation_InvalidUserID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		manager := NewReservationManager(ctx)

		reservation, err := manager.CreateReservation(ctx, 0, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		assert.Error(t, err)
		assert.Nil(t, reservation)
	}, pluginTesting.TestOptions())
}

func TestReservationManager_CommitReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create a pending reservation
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		uploadID := dataManager.NextUploadID()
		err = manager.CommitReservation(ctx, reservation.ID, uploadID)
		require.NoError(t, err)

		// Verify reservation is committed
		committedReservation, err := manager.GetReservationByID(ctx, reservation.ID)
		require.NoError(t, err)
		assert.Equal(t, pluginModels.ReservationStatusCommitted, committedReservation.Status)
		assert.NotNil(t, committedReservation.UploadID)
		assert.Equal(t, uploadID, *committedReservation.UploadID)

		// Verify usage detail was created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeUpload, usageDetails[0].Type)
		assert.Equal(t, uint64(testReservationBytes), usageDetails[0].Bytes)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_CommitReservation_AlreadyCommitted(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create and commit a reservation
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		uploadID := dataManager.NextUploadID()
		err = manager.CommitReservation(ctx, reservation.ID, uploadID)
		require.NoError(t, err)

		// Try to commit again
		err = manager.CommitReservation(ctx, reservation.ID, uploadID)
		assert.NoError(t, err) // Should not error, just ignore

		// Verify only one usage detail exists
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_ReleaseReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create a pending reservation
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		err = manager.ReleaseReservation(ctx, reservation.ID)
		require.NoError(t, err)

		// Verify reservation is rolled back
		releasedReservation, err := manager.GetReservationByID(ctx, reservation.ID)
		require.NoError(t, err)
		assert.Equal(t, pluginModels.ReservationStatusRolledBack, releasedReservation.Status)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_ReleaseReservation_AlreadyReleased(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create and release a reservation
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		err = manager.ReleaseReservation(ctx, reservation.ID)
		require.NoError(t, err)

		// Try to release again
		err = manager.ReleaseReservation(ctx, reservation.ID)
		assert.NoError(t, err) // Should not error, just ignore

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_GetReservationByID_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create a reservation
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		// Get by ID
		fetchedReservation, err := manager.GetReservationByID(ctx, reservation.ID)
		require.NoError(t, err)
		assert.Equal(t, reservation.ID, fetchedReservation.ID)
		assert.Equal(t, reservation.UserID, fetchedReservation.UserID)
		assert.Equal(t, reservation.Type, fetchedReservation.Type)
		assert.Equal(t, reservation.Bytes, fetchedReservation.Bytes)
		assert.Equal(t, reservation.Status, fetchedReservation.Status)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_GetPendingReservationsForUser_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create multiple pending reservations
		reservation1, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, 100, testIPAddress)
		require.NoError(t, err)

		reservation2, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeDownload, 200, testIPAddress)
		require.NoError(t, err)

		// Get pending reservations
		pendingReservations, err := manager.GetPendingReservationsForUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, pendingReservations, 2)

		// Verify both reservations are present
		ids := []uint{pendingReservations[0].ID, pendingReservations[1].ID}
		assert.Contains(t, ids, reservation1.ID)
		assert.Contains(t, ids, reservation2.ID)

		// Commit one and verify only one remains pending
		uploadID := dataManager.NextUploadID()
		err = manager.CommitReservation(ctx, reservation1.ID, uploadID)
		require.NoError(t, err)

		pendingReservations, err = manager.GetPendingReservationsForUser(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, pendingReservations, 1)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_SumPendingBytesForUser_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create multiple upload reservations
		_, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, 100, testIPAddress)
		require.NoError(t, err)

		_, err = manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, 200, testIPAddress)
		require.NoError(t, err)

		_, err = manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, 300, testIPAddress)
		require.NoError(t, err)

		// Create some download reservations that shouldn't be counted
		_, err = manager.CreateReservation(ctx, userID, pluginModels.UsageTypeDownload, 150, testIPAddress)
		require.NoError(t, err)

		_, err = manager.CreateReservation(ctx, userID, pluginModels.UsageTypeDownload, 250, testIPAddress)
		require.NoError(t, err)

		// Sum pending upload bytes
		sum, err := manager.SumPendingBytesForUser(ctx, userID, pluginModels.UsageTypeUpload)
		require.NoError(t, err)
		assert.Equal(t, uint64(600), sum) // 100 + 200 + 300

		// Sum pending download bytes
		sum, err = manager.SumPendingBytesForUser(ctx, userID, pluginModels.UsageTypeDownload)
		require.NoError(t, err)
		assert.Equal(t, uint64(400), sum) // 150 + 250

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_CleanupStaleReservationsForUser_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create a reservation
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		// Manually set the created time to the past to simulate staleness
		pastTime := time.Now().UTC().Add(-2 * time.Hour)
		err = ctx.DB().Model(&pluginModels.QuotaReservation{}).
			Where("id = ?", reservation.ID).
			Update("created_at", pastTime).Error
		require.NoError(t, err)

		// Cleanup stale reservations
		cleaned, err := manager.CleanupStaleReservationsForUser(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(1), cleaned)

		// Verify reservation is soft deleted (expired)
		// Use Unscoped to access soft deleted records
		var rolledBackReservation pluginModels.QuotaReservation
		err = ctx.DB().Unscoped().First(&rolledBackReservation, reservation.ID).Error
		require.NoError(t, err)
		assert.True(t, rolledBackReservation.IsDeleted())
		assert.True(t, rolledBackReservation.IsPending()) // Status remains PENDING

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_CleanupStaleReservationsForUser_NoStaleReservations(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Create a fresh reservation
		_, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)

		// Cleanup stale reservations (should find none)
		cleaned, err := manager.CleanupStaleReservationsForUser(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, int64(0), cleaned)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_ConcurrentAccess(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		numGoroutines := 10
		var wg sync.WaitGroup
		reservations := make([]uint, numGoroutines)

		// Create reservations concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, uint64(100+index), testIPAddress)
				require.NoError(tb, err)
				reservations[index] = reservation.ID
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()

		// Verify all reservations were created
		count := int64(0)
		err := ctx.DB().Model(&pluginModels.QuotaReservation{}).Where("user_id = ?", userID).Count(&count).Error
		require.NoError(tb, err)
		assert.Equal(tb, int64(numGoroutines), count)

		// Clean up
		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_ReserveAndCommit(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Reservation should exist
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())
		assert.False(t, reservation.IsCommitted())
		assert.False(t, reservation.IsRolledBack())

		// Commit reservation
		uploadID := dataManager.NextUploadID()
		err = manager.CommitReservation(ctx, reservation.ID, uploadID)
		require.NoError(t, err)

		// Verify status changed
		committedReservation, err := manager.GetReservationByID(ctx, reservation.ID)
		require.NoError(t, err)
		assert.False(t, committedReservation.IsPending())
		assert.True(t, committedReservation.IsCommitted())
		assert.False(t, committedReservation.IsRolledBack())

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestReservationManager_ReserveAndRelease(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  nil,
			UploadLimitBytes:   nil,
			DownloadLimitBytes: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		manager := NewReservationManager(ctx)

		// Reservation should exist
		reservation, err := manager.CreateReservation(ctx, userID, pluginModels.UsageTypeUpload, testReservationBytes, testIPAddress)
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())
		assert.False(t, reservation.IsRolledBack())

		// Release reservation
		err = manager.ReleaseReservation(ctx, reservation.ID)
		require.NoError(t, err)

		// Verify status changed
		releasedReservation, err := manager.GetReservationByID(ctx, reservation.ID)
		require.NoError(t, err)
		assert.False(t, releasedReservation.IsPending())
		assert.False(t, releasedReservation.IsCommitted())
		assert.True(t, releasedReservation.IsRolledBack())

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
