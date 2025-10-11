package policies

import (
	"testing"
	"time"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"sync"
)

func TestUnlimitedPolicyEnforcer_CheckUploadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(1000)

		t.Run("Valid user with unlimited policy", func(t *testing.T) {
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
			})

			result, err := enforcer.CheckUploadQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyUnlimited))
		})

		t.Run("Invalid bytes should return error", func(t *testing.T) {
			userID := baseUserID + 2
			uploadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
			})

			_, err := enforcer.CheckUploadQuota(config, 0)
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})

		t.Run("Large bytes amount should still be allowed", func(t *testing.T) {
			userID := baseUserID + 3
			uploadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
			})

			result, err := enforcer.CheckUploadQuota(config, uint64(units.TiB)) // 1 Tibibyte
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyUnlimited))
		})
	}, testOptions())
}

func TestUnlimitedPolicyEnforcer_CheckDownloadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(2000)

		t.Run("Valid user with unlimited policy", func(t *testing.T) {
			userID := baseUserID + 1
			downloadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
			})

			result, err := enforcer.CheckDownloadQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyUnlimited))
		})

		t.Run("Invalid bytes should return error", func(t *testing.T) {
			userID := baseUserID + 2
			downloadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
			})

			_, err := enforcer.CheckDownloadQuota(config, 0)
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})

		t.Run("Large bytes amount should still be allowed", func(t *testing.T) {
			userID := baseUserID + 3
			downloadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
			})

			result, err := enforcer.CheckDownloadQuota(config, uint64(units.TiB)) // 1 Tibibyte
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyUnlimited))
		})
	}, testOptions())
}

func TestUnlimitedPolicyEnforcer_CheckStorageQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(3000)

		t.Run("Valid user with unlimited policy", func(t *testing.T) {
			userID := baseUserID + 1
			storageLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				StorageLimit: &storageLimit,
			})

			result, err := enforcer.CheckStorageQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyUnlimited))
		})

		t.Run("Invalid bytes should return error", func(t *testing.T) {
			userID := baseUserID + 2
			storageLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				StorageLimit: &storageLimit,
			})

			_, err := enforcer.CheckStorageQuota(config, 0)
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})

		t.Run("Large bytes amount should still be allowed", func(t *testing.T) {
			userID := baseUserID + 3
			storageLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{
				StorageLimit: &storageLimit,
			})

			result, err := enforcer.CheckStorageQuota(config, uint64(units.TiB)) // 1 Tibibyte
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyUnlimited))
		})
	}, testOptions())
}

func TestUnlimitedPolicyEnforcer_RecordUpload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(4000)

		t.Run("Valid upload recording", func(t *testing.T) {
			userID := baseUserID + 1
			uploadID := uint(100)
			bytes := uint64(500)
			ip := "192.168.1.1"
			createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})

			err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
			require.NoError(t, err)

			// Verify the usage detail was recorded
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeUpload, usageDetails[0].Type)
			assert.Equal(t, bytes, usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)
			assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Uploads are not shared

			// Verify the daily quota was updated
			today := time.Now().Truncate(24 * time.Hour)
			var dailyQuota models.UserQuota
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, bytes, dailyQuota.BytesUploaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesStored)
		})

		t.Run("Invalid user ID should return error", func(t *testing.T) {
			err := enforcer.RecordUpload(0, 1, 100, "192.168.1.1")
			assert.ErrorIs(t, err, models.ErrInvalidUserID)
		})

		t.Run("Zero bytes should return error", func(t *testing.T) {
			err := enforcer.RecordUpload(baseUserID + 2, 1, 0, "192.168.1.1")
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})
	}, testOptions())
}

func TestUnlimitedPolicyEnforcer_RecordDownload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(5000)

		t.Run("Valid download recording", func(t *testing.T) {
			userID := baseUserID + 1
			uploadID := uint(100)
			bytes := uint64(500)
			ip := "192.168.1.1"
			createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})

			err := enforcer.RecordDownload(userID, uploadID, bytes, ip)
			require.NoError(t, err)

			// Verify the usage detail was recorded
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeDownload, usageDetails[0].Type)
			assert.Equal(t, bytes, usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)

			// Verify the daily quota was updated
			today := time.Now().Truncate(24 * time.Hour)
			var dailyQuota models.UserQuota
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
			assert.Equal(t, bytes, dailyQuota.BytesDownloaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesStored)
		})

		t.Run("Invalid user ID should return error", func(t *testing.T) {
			err := enforcer.RecordDownload(0, 1, 100, "192.168.1.1")
			assert.ErrorIs(t, err, models.ErrInvalidUserID)
		})

		t.Run("Zero bytes should return error", func(t *testing.T) {
			err := enforcer.RecordDownload(baseUserID + 2, 1, 0, "192.168.1.1")
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})
	}, testOptions())
}

func TestUnlimitedPolicyEnforcer_RecordStorageChange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(6000)

		t.Run("Valid positive storage change", func(t *testing.T) {
			userID := baseUserID + 1
			uploadID := uint(100)
			bytes := int64(500)
			ip := "192.168.1.1"
			createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})

			err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
			require.NoError(t, err)

			// Verify the usage detail was recorded
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeStorageAdd, usageDetails[0].Type)
			assert.Equal(t, uint64(bytes), usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)

			// Verify the daily quota was updated
			today := time.Now().Truncate(24 * time.Hour)
			var dailyQuota models.UserQuota
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
			assert.Equal(t, uint64(bytes), dailyQuota.BytesStored)
		})

		t.Run("Valid negative storage change", func(t *testing.T) {
			userID := baseUserID + 2
			uploadID := uint(100)
			bytes := int64(-300)
			ip := "192.168.1.1"
			createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})

			err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
			require.NoError(t, err)

			// Verify the usage detail was recorded
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeStorageRemove, usageDetails[0].Type)
			assert.Equal(t, uint64(-bytes), usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)
		})

		t.Run("Zero bytes should return error", func(t *testing.T) {
			err := enforcer.RecordStorageChange(baseUserID + 3, 1, 0, "192.168.1.1")
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})

		t.Run("Invalid user ID should return error", func(t *testing.T) {
			err := enforcer.RecordStorageChange(0, 1, 100, "192.168.1.1")
			assert.ErrorIs(t, err, models.ErrInvalidUserID)
		})
	}, testOptions())
}

func TestUnlimitedPolicyEnforcer_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent upload recordings", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewUnlimitedPolicyEnforcer(ctx)
			baseUserID := uint(8000)

			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})

			// Run concurrent upload recordings
			var errors []error
			var mu sync.Mutex
			var wg sync.WaitGroup

			numGoroutines := 5
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(goroutineID int) {
					defer wg.Done()
					err := enforcer.RecordUpload(userID, uint(goroutineID+1), 100, "192.168.1.1")
					mu.Lock()
					errors = append(errors, err)
					mu.Unlock()
				}(i)
			}

			wg.Wait()

			// All should succeed
			for _, err := range errors {
				assert.NoError(t, err)
			}
		}, testOptions())
	})
}

func TestUnlimitedPolicyEnforcer_DelegationMethods(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewUnlimitedPolicyEnforcer(ctx)
		baseUserID := uint(7000)

		userID := baseUserID + 1
		createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})

		// Record usage through the enforcer methods to ensure both detail and daily records are created
		now := time.Now()
		err := enforcer.RecordUpload(userID, 1, 100, "192.168.1.1")
		require.NoError(t, err)

		err = enforcer.RecordDownload(userID, 2, 200, "192.168.1.2")
		require.NoError(t, err)

		err = enforcer.RecordStorageChange(userID, 3, 300, "192.168.1.3")
		require.NoError(t, err)

		t.Run("GetDetailedUsage", func(t *testing.T) {
			start := now.Add(-3 * time.Hour)
			end := now.Add(1 * time.Hour)

			details, err := enforcer.GetDetailedUsage(userID, start, end)
			require.NoError(t, err)
			assert.Len(t, details, 3)

			// Verify records are in descending order by timestamp
			if len(details) >= 2 {
				assert.True(t, details[0].Timestamp.After(details[1].Timestamp))
			}
			if len(details) >= 3 {
				assert.True(t, details[1].Timestamp.After(details[2].Timestamp))
			}
		})

		t.Run("GetCurrentUsage", func(t *testing.T) {
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, userID, usage.UserID)
			// Now these should have proper values since we used the enforcer's record methods
			assert.Equal(t, uint64(100), usage.BytesUploaded)
			assert.Equal(t, uint64(200), usage.BytesDownloaded)
			assert.Equal(t, uint64(300), usage.BytesStored)
		})

		t.Run("GetUsageHistory", func(t *testing.T) {
			history, err := enforcer.GetUsageHistory(userID, 1, models.UsageTypeUpload)
			require.NoError(t, err)
			assert.Len(t, history, 1)
			assert.Equal(t, models.UsageTypeUpload, history[0].Type)
			assert.Equal(t, uint64(100), history[0].Bytes)
		})
	}, testOptions())
}
