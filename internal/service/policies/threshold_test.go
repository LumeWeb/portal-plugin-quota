package policies

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"sync"
)

// TestThresholdPolicyEnforcer_CheckUploadQuota tests the CheckUploadQuota method
func TestThresholdPolicyEnforcer_CheckUploadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(30000)

		t.Run("Within daily limit", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(uint64(1000)),
				UploadThreshold:  lo.ToPtr(uint64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with upload daily limit
			config.UploadDailyLimit = lo.ToPtr(uint64(1000))
			config.UploadThreshold = lo.ToPtr(uint64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckUploadQuota(userID, 500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyThreshold))
		})

		t.Run("Exceeding daily limit", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(uint64(1000)),
				UploadThreshold:  lo.ToPtr(uint64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with upload daily limit
			config.UploadDailyLimit = lo.ToPtr(uint64(1000))
			config.UploadThreshold = lo.ToPtr(uint64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 600)

			result, err := enforcer.CheckUploadQuota(userID, 500)
			require.NoError(t, err)
			assertQuotaCheckResultWithDetails(t, result, false, models.QuotaCheckReasonLimitExceeded,
				core.EnforcementPolicy(models.EnforcementPolicyThreshold), 600, 1000)
		})

		t.Run("At threshold warning level", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(uint64(1000)),
				UploadThreshold:  lo.ToPtr(uint64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with upload daily limit and threshold
			config.UploadDailyLimit = lo.ToPtr(uint64(1000))
			config.UploadThreshold = lo.ToPtr(uint64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 700)

			result, err := enforcer.CheckUploadQuota(userID, 200)
			require.NoError(t, err)
			assertQuotaCheckResultWithThreshold(t, result, true, models.QuotaCheckReasonWarningThreshold,
				core.EnforcementPolicy(models.EnforcementPolicyThreshold), 700, 800, 1000)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes (0)
			result, err := enforcer.CheckUploadQuota(userID, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, core.QuotaCheckReason(""), result.Reason)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_CheckDownloadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(31000)

		t.Run("Within daily limit", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: lo.ToPtr(uint64(2000)),
				DownloadThreshold:  lo.ToPtr(uint64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with download daily limit
			config.DownloadDailyLimit = lo.ToPtr(uint64(2000))
			config.DownloadThreshold = lo.ToPtr(uint64(1500))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckDownloadQuota(userID, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyThreshold))
		})

		t.Run("Exceeding daily limit", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: lo.ToPtr(uint64(2000)),
				DownloadThreshold:  lo.ToPtr(uint64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with download daily limit
			config.DownloadDailyLimit = lo.ToPtr(uint64(2000))
			config.DownloadThreshold = lo.ToPtr(uint64(1500))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 1500)

			result, err := enforcer.CheckDownloadQuota(userID, 1000)
			require.NoError(t, err)
			assertQuotaCheckResultWithDetails(t, result, false, models.QuotaCheckReasonLimitExceeded,
				core.EnforcementPolicy(models.EnforcementPolicyThreshold), 1500, 2000)
		})

		t.Run("At threshold warning level", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: lo.ToPtr(uint64(2000)),
				DownloadThreshold:  lo.ToPtr(uint64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with download daily limit and threshold
			config.DownloadDailyLimit = lo.ToPtr(uint64(2000))
			config.DownloadThreshold = lo.ToPtr(uint64(1500))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 1400)

			result, err := enforcer.CheckDownloadQuota(userID, 200)
			require.NoError(t, err)
			assertQuotaCheckResultWithThreshold(t, result, true, models.QuotaCheckReasonWarningThreshold,
				core.EnforcementPolicy(models.EnforcementPolicyThreshold), 1400, 1500, 2000)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes (0)
			result, err := enforcer.CheckDownloadQuota(userID, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, core.QuotaCheckReason(""), result.Reason)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_CheckStorageQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(32000)

		t.Run("Within storage limit", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(uint64(3000)),
				StorageThreshold: lo.ToPtr(uint64(2000)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with storage limit
			config.StorageLimit = lo.ToPtr(uint64(3000))
			config.StorageThreshold = lo.ToPtr(uint64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckStorageQuota(userID, 1500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, core.EnforcementPolicy(models.EnforcementPolicyThreshold))
		})

		t.Run("Exceeding storage limit", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(uint64(3000)),
				StorageThreshold: lo.ToPtr(uint64(2000)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with storage limit
			config.StorageLimit = lo.ToPtr(uint64(3000))
			config.StorageThreshold = lo.ToPtr(uint64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 2500)

			result, err := enforcer.CheckStorageQuota(userID, 1000)
			require.NoError(t, err)
			assertQuotaCheckResultWithDetails(t, result, false, models.QuotaCheckReasonLimitExceeded,
				core.EnforcementPolicy(models.EnforcementPolicyThreshold), 2500, 3000)
		})

		t.Run("At threshold warning level", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(uint64(3000)),
				StorageThreshold: lo.ToPtr(uint64(2000)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with storage limit and threshold
			config.StorageLimit = lo.ToPtr(uint64(3000))
			config.StorageThreshold = lo.ToPtr(uint64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold
			createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 1900)

			result, err := enforcer.CheckStorageQuota(userID, 200)
			require.NoError(t, err)
			assertQuotaCheckResultWithThreshold(t, result, true, models.QuotaCheckReasonWarningThreshold,
				core.EnforcementPolicy(models.EnforcementPolicyThreshold), 1900, 2000, 3000)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes (0)
			result, err := enforcer.CheckStorageQuota(userID, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, core.QuotaCheckReason(""), result.Reason)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_RecordUpload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(33000)

		t.Run("Successful upload recording", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with upload daily limit
			config.UploadDailyLimit = lo.ToPtr(uint64(1000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Record upload
			err = enforcer.RecordUpload(userID, 100, 500, "127.0.0.1")
			assert.NoError(t, err)

			// Verify the usage was recorded
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(500), usage.BytesUploaded)
		})

		t.Run("Upload that exceeds quota", func(t *testing.T) {
			userID := baseUserID + 2
			uploadDailyLimit := uint64(1000)
			uploadThreshold := uint64(800)
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadThreshold:  &uploadThreshold,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 600)

			// Test upload that exceeds quota
			err = enforcer.RecordUpload(userID, 101, 500, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "upload blocked")
		})

		t.Run("Invalid user ID", func(t *testing.T) {
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Test invalid user ID
			err := enforcer.RecordUpload(0, 102, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Test invalid bytes
			err := enforcer.RecordUpload(userID, 103, 0, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_RecordDownload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(34000)

		t.Run("Successful download recording", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with download daily limit
			config.DownloadDailyLimit = lo.ToPtr(uint64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Record download
			err = enforcer.RecordDownload(userID, 200, 1000, "127.0.0.1")
			assert.NoError(t, err)

			// Verify the usage was recorded
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(1000), usage.BytesDownloaded)
		})

		t.Run("Download that exceeds quota", func(t *testing.T) {
			userID := baseUserID + 2
			downloadDailyLimit := uint64(1000)
			downloadThreshold := uint64(800)
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadThreshold:  &downloadThreshold,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 600)

			// Test download that exceeds quota
			err = enforcer.RecordDownload(userID, 201, 500, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "download blocked")
		})

		t.Run("Invalid user ID", func(t *testing.T) {
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Test invalid user ID
			err := enforcer.RecordDownload(0, 202, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Test invalid bytes
			err := enforcer.RecordDownload(userID, 203, 0, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_RecordStorageChange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(35000)

		t.Run("Successful storage addition recording", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with storage limit
			config.StorageLimit = lo.ToPtr(uint64(3000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Record storage change
			err = enforcer.RecordStorageChange(userID, 300, 1500, "127.0.0.1")
			assert.NoError(t, err)

			// Verify the usage was recorded
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(1500), usage.BytesStored)
		})

		t.Run("Storage addition that exceeds quota", func(t *testing.T) {
			userID := baseUserID + 2
			storageLimit := uint64(1000)
			storageThreshold := uint64(800)
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     &storageLimit,
				StorageThreshold: &storageThreshold,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test storage addition that exceeds quota
			err = enforcer.RecordStorageChange(userID, 301, 1500, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "storage change blocked")
		})

		t.Run("Storage removal (no quota enforcement)", func(t *testing.T) {
			userID := baseUserID + 3
			storageLimit := uint64(1000)
			storageThreshold := uint64(800)
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     &storageLimit,
				StorageThreshold: &storageThreshold,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test storage removal
			err = enforcer.RecordStorageChange(userID, 302, -500, "127.0.0.1")
			assert.NoError(t, err)
		})

		t.Run("Invalid user ID", func(t *testing.T) {
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Test invalid user ID
			err := enforcer.RecordStorageChange(0, 303, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Zero bytes", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Test zero bytes
			err := enforcer.RecordStorageChange(userID, 304, 0, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_GetDetailedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(36001)
		createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})

		enforcer := NewThresholdPolicyEnforcer(ctx)

		// Record some usage
		err := enforcer.RecordUpload(userID, 400, 500, "127.0.0.1")
		require.NoError(t, err)

		err = enforcer.RecordDownload(userID, 401, 1000, "127.0.0.1")
		require.NoError(t, err)

		err = enforcer.RecordStorageChange(userID, 402, 1500, "127.0.0.1")
		require.NoError(t, err)

		// Test getting detailed usage
		start := time.Now().Add(-time.Hour)
		end := time.Now().Add(time.Hour)

		details, err := enforcer.GetDetailedUsage(userID, start, end)
		assert.NoError(t, err)
		assert.Len(t, details, 3)

		// Verify the types of usage recorded
		types := make(map[models.UsageType]bool)
		for _, detail := range details {
			types[detail.Type] = true
		}

		assert.True(t, types[models.UsageTypeUpload])
		assert.True(t, types[models.UsageTypeDownload])
		assert.True(t, types[models.UsageTypeStorageAdd])
	}, testOptions())
}

func TestThresholdPolicyEnforcer_GetCurrentUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(37001)
		createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})

		enforcer := NewThresholdPolicyEnforcer(ctx)

		// Record some usage
		err := enforcer.RecordUpload(userID, 500, 300, "127.0.0.1")
		require.NoError(t, err)

		err = enforcer.RecordDownload(userID, 501, 600, "127.0.0.1")
		require.NoError(t, err)

		err = enforcer.RecordStorageChange(userID, 502, 900, "127.0.0.1")
		require.NoError(t, err)

		// Test getting current usage
		usage, err := enforcer.GetCurrentUsage(userID)
		assert.NoError(t, err)
		assert.Equal(t, userID, usage.UserID)
		assert.Equal(t, uint64(300), usage.BytesUploaded)
		assert.Equal(t, uint64(600), usage.BytesDownloaded)
		assert.Equal(t, uint64(900), usage.BytesStored)
	}, testOptions())
}

func TestThresholdPolicyEnforcer_GetUsageHistory(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(38001)
		createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})

		enforcer := NewThresholdPolicyEnforcer(ctx)

		usageType := models.UsageTypeUpload
		period := 30 // days

		// Record some usage
		err := enforcer.RecordUpload(userID, 600, 200, "127.0.0.1")
		require.NoError(t, err)

		err = enforcer.RecordUpload(userID, 601, 400, "127.0.0.1")
		require.NoError(t, err)

		// Test getting usage history
		history, err := enforcer.GetUsageHistory(userID, period, core.UsageType(usageType))
		assert.NoError(t, err)
		assert.Len(t, history, 2)

		// Verify the bytes values
		bytes := make([]uint64, 0)
		for _, point := range history {
			bytes = append(bytes, point.Bytes)
		}

		assert.Contains(t, bytes, uint64(200))
		assert.Contains(t, bytes, uint64(400))
	}, testOptions())
}

func TestThresholdPolicyEnforcer_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent quota checks", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			baseUserID := uint(50000)
			userID := baseUserID + 1

			uploadDailyLimit := uint64(1000)
			uploadThreshold := uint64(800)
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadThreshold:  &uploadThreshold,
			})

			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Run concurrent quota checks
			var results []pluginCore.QuotaCheckResult
			var errors []error
			var mu sync.Mutex
			var wg sync.WaitGroup

			numGoroutines := 5
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					result, err := enforcer.CheckUploadQuota(userID, 100)
					mu.Lock()
					results = append(results, result)
					errors = append(errors, err)
					mu.Unlock()
				}()
			}

			wg.Wait()

			// All should succeed
			for _, err := range errors {
				assert.NoError(t, err)
			}

			// All should be allowed
			for _, result := range results {
				assert.True(t, result.Allowed)
			}
		}, testOptions())
	})
}

func TestThresholdPolicyEnforcer_resolveEffectiveLimits(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(39000)

		t.Run("Custom limits only", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:       lo.ToPtr(uint64(1000)),
				UploadDailyLimit:   lo.ToPtr(uint64(500)),
				DownloadDailyLimit: lo.ToPtr(uint64(750)),
				UploadTotalLimit:   lo.ToPtr(uint64(2000)),
				DownloadTotalLimit: lo.ToPtr(uint64(3000)),
				StorageThreshold:   lo.ToPtr(uint64(800)),
				UploadThreshold:    lo.ToPtr(uint64(400)),
				DownloadThreshold:  lo.ToPtr(uint64(600)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with custom limits
			config.StorageLimit = lo.ToPtr(uint64(1000))
			config.UploadDailyLimit = lo.ToPtr(uint64(500))
			config.DownloadDailyLimit = lo.ToPtr(uint64(750))
			config.UploadTotalLimit = lo.ToPtr(uint64(2000))
			config.DownloadTotalLimit = lo.ToPtr(uint64(3000))
			config.StorageThreshold = lo.ToPtr(uint64(800))
			config.UploadThreshold = lo.ToPtr(uint64(400))
			config.DownloadThreshold = lo.ToPtr(uint64(600))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, userID, limits.UserID)
			assert.Equal(t, core.EnforcementPolicy(models.EnforcementPolicyThreshold), limits.EnforcementPolicy)
			assert.Equal(t, uint64(1000), *limits.StorageLimit)
			assert.Equal(t, uint64(500), *limits.UploadDailyLimit)
			assert.Equal(t, uint64(750), *limits.DownloadDailyLimit)
			assert.Equal(t, uint64(2000), *limits.UploadTotalLimit)
			assert.Equal(t, uint64(3000), *limits.DownloadTotalLimit)
			assert.Equal(t, uint64(800), *limits.StorageThreshold)
			assert.Equal(t, uint64(400), *limits.UploadThreshold)
			assert.Equal(t, uint64(600), *limits.DownloadThreshold)
		})

		t.Run("Quota plan limits", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Create a quota plan
			plan := createTestQuotaPlan(t, ctx, "Test Plan", false, &testPlanLimits{
				StorageLimit:       1000,
				UploadDailyLimit:   500,
				DownloadDailyLimit: 750,
				UploadTotalLimit:   2000,
				DownloadTotalLimit: 3000,
			})

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config to use the plan
			planID := uint64(plan.ID)
			config.QuotaPlanID = &planID
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, plan.StorageLimit, *limits.StorageLimit)
			assert.Equal(t, plan.UploadDailyLimit, *limits.UploadDailyLimit)
			assert.Equal(t, plan.DownloadDailyLimit, *limits.DownloadDailyLimit)
			assert.Equal(t, plan.UploadTotalLimit, *limits.UploadTotalLimit)
			assert.Equal(t, plan.DownloadTotalLimit, *limits.DownloadTotalLimit)
			assert.Equal(t, *plan.StorageThreshold, *limits.StorageThreshold)
			assert.Equal(t, *plan.UploadThreshold, *limits.UploadThreshold)
			assert.Equal(t, *plan.DownloadThreshold, *limits.DownloadThreshold)
		})

		t.Run("Mixed configuration", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:    lo.ToPtr(uint64(1000)),
				UploadThreshold: lo.ToPtr(uint64(400)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Create a quota plan
			plan := createTestQuotaPlan(t, ctx, "Test Plan 2", false, &testPlanLimits{
				StorageLimit:       2000,
				UploadDailyLimit:   1000,
				DownloadDailyLimit: 1500,
				UploadTotalLimit:   5000,
				DownloadTotalLimit: 7500,
			})

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with mixed configuration
			planID := uint64(plan.ID)
			config.QuotaPlanID = &planID
			config.StorageLimit = lo.ToPtr(uint64(1000))   // Override plan's storage limit
			config.UploadThreshold = lo.ToPtr(uint64(400)) // Override plan's upload threshold
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, uint64(1000), *limits.StorageLimit)                  // Custom value
			assert.Equal(t, plan.UploadDailyLimit, *limits.UploadDailyLimit)     // Plan value
			assert.Equal(t, plan.DownloadDailyLimit, *limits.DownloadDailyLimit) // Plan value
			assert.Equal(t, plan.UploadTotalLimit, *limits.UploadTotalLimit)     // Plan value
			assert.Equal(t, plan.DownloadTotalLimit, *limits.DownloadTotalLimit) // Plan value
			assert.Equal(t, *plan.StorageThreshold, *limits.StorageThreshold)    // Plan value
			assert.Equal(t, uint64(400), *limits.UploadThreshold)                // Custom value
			assert.Equal(t, *plan.DownloadThreshold, *limits.DownloadThreshold)  // Plan value
		})

		t.Run("Default plan", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Create a default quota plan
			defaultPlan := createTestQuotaPlan(t, ctx, "Default Plan", true, &testPlanLimits{
				StorageLimit:       500,
				UploadDailyLimit:   250,
				DownloadDailyLimit: 300,
				UploadTotalLimit:   1000,
				DownloadTotalLimit: 1500,
			})

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Ensure no custom plan is set
			config.QuotaPlanID = nil
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, defaultPlan.StorageLimit, *limits.StorageLimit)
			assert.Equal(t, defaultPlan.UploadDailyLimit, *limits.UploadDailyLimit)
			assert.Equal(t, defaultPlan.DownloadDailyLimit, *limits.DownloadDailyLimit)
			assert.Equal(t, defaultPlan.UploadTotalLimit, *limits.UploadTotalLimit)
			assert.Equal(t, defaultPlan.DownloadTotalLimit, *limits.DownloadTotalLimit)
			assert.Equal(t, *defaultPlan.StorageThreshold, *limits.StorageThreshold)
			assert.Equal(t, *defaultPlan.UploadThreshold, *limits.UploadThreshold)
			assert.Equal(t, *defaultPlan.DownloadThreshold, *limits.DownloadThreshold)
		})
	}, testOptions())
}
