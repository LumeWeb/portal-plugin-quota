package policies

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"sync"
)

// TestThresholdPolicyEnforcer_CheckUploadQuota tests the CheckUploadQuota method
func TestThresholdPolicyEnforcer_CheckUploadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(30000)

		t.Run("Within daily limit", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(int64(1000)),
				UploadThreshold:  lo.ToPtr(int64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with upload daily limit
			config.UploadDailyLimit = lo.ToPtr(int64(1000))
			config.UploadThreshold = lo.ToPtr(int64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckUploadQuota(config.UserID, 500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold))
		})

		t.Run("Exceeding daily limit", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(int64(1000)),
				UploadThreshold:  lo.ToPtr(int64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with upload daily limit
			config.UploadDailyLimit = lo.ToPtr(int64(1000))
			config.UploadThreshold = lo.ToPtr(int64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 600)

			result, err := enforcer.CheckUploadQuota(config.UserID, 500)
			require.NoError(t, err)
			assertQuotaCheckResultWithDetails(t, result, false, models.QuotaCheckReasonLimitExceeded,
				pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), 600, 1000)
		})

		t.Run("At threshold warning level", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(int64(1000)),
				UploadThreshold:  lo.ToPtr(int64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with upload daily limit and threshold
			config.UploadDailyLimit = lo.ToPtr(int64(1000))
			config.UploadThreshold = lo.ToPtr(int64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 700)

			result, err := enforcer.CheckUploadQuota(config.UserID, 200)
			require.NoError(t, err)
			assertQuotaCheckResultWithThreshold(t, result, true, models.QuotaCheckReasonWarningThreshold,
				pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), 700, 800, 1000)
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
			result, err := enforcer.CheckUploadQuota(config.UserID, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_CheckDownloadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(31000)

		t.Run("Within daily limit", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: lo.ToPtr(int64(2000)),
				DownloadThreshold:  lo.ToPtr(int64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with download daily limit
			config.DownloadDailyLimit = lo.ToPtr(int64(2000))
			config.DownloadThreshold = lo.ToPtr(int64(1500))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckDownloadQuota(config.UserID, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold))
		})

		t.Run("Exceeding daily limit", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: lo.ToPtr(int64(2000)),
				DownloadThreshold:  lo.ToPtr(int64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with download daily limit
			config.DownloadDailyLimit = lo.ToPtr(int64(2000))
			config.DownloadThreshold = lo.ToPtr(int64(1500))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 1500)

			result, err := enforcer.CheckDownloadQuota(config.UserID, 1000)
			require.NoError(t, err)
			assertQuotaCheckResultWithDetails(t, result, false, models.QuotaCheckReasonLimitExceeded,
				pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), 1500, 2000)
		})

		t.Run("At threshold warning level", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				DownloadDailyLimit: lo.ToPtr(int64(2000)),
				DownloadThreshold:  lo.ToPtr(int64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with download daily limit and threshold
			config.DownloadDailyLimit = lo.ToPtr(int64(2000))
			config.DownloadThreshold = lo.ToPtr(int64(1500))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 1400)

			result, err := enforcer.CheckDownloadQuota(config.UserID, 200)
			require.NoError(t, err)
			assertQuotaCheckResultWithThreshold(t, result, true, models.QuotaCheckReasonWarningThreshold,
				pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), 1400, 1500, 2000)
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
			result, err := enforcer.CheckDownloadQuota(config.UserID, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_CheckStorageQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(32000)

		t.Run("Within storage limit", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(3000)),
				StorageThreshold: lo.ToPtr(int64(2000)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with storage limit
			config.StorageLimit = lo.ToPtr(int64(3000))
			config.StorageThreshold = lo.ToPtr(int64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckStorageQuota(config.UserID, 1500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold))
		})

		t.Run("Exceeding storage limit", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(3000)),
				StorageThreshold: lo.ToPtr(int64(2000)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with storage limit
			config.StorageLimit = lo.ToPtr(int64(3000))
			config.StorageThreshold = lo.ToPtr(int64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 2500)

			result, err := enforcer.CheckStorageQuota(config.UserID, 1000)
			require.NoError(t, err)
			assertQuotaCheckResultWithDetails(t, result, false, models.QuotaCheckReasonLimitExceeded,
				pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), 2500, 3000)
		})

		t.Run("At threshold warning level", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(3000)),
				StorageThreshold: lo.ToPtr(int64(2000)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with storage limit and threshold
			config.StorageLimit = lo.ToPtr(int64(3000))
			config.StorageThreshold = lo.ToPtr(int64(2000))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold
			createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 1900)

			result, err := enforcer.CheckStorageQuota(config.UserID, 200)
			require.NoError(t, err)
			assertQuotaCheckResultWithThreshold(t, result, true, models.QuotaCheckReasonWarningThreshold,
				pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), 1900, 2000, 3000)
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
			result, err := enforcer.CheckStorageQuota(config.UserID, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
		})

		t.Run("Storage limit set to 0 (disabled)", func(t *testing.T) {
			userID := baseUserID + 5
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit: lo.ToPtr(int64(0)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			result, err := enforcer.CheckStorageQuota(userID, 100)
			require.NoError(t, err)
			assert.False(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
		})

		t.Run("Storage limit set to unlimited", func(t *testing.T) {
			userID := baseUserID + 6
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit: (*int64)(nil),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			result, err := enforcer.CheckStorageQuota(userID, 1000000)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		})

		t.Run("Threshold set to 0 with positive limit", func(t *testing.T) {
			userID := baseUserID + 7
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(3000)),
				StorageThreshold: lo.ToPtr(int64(0)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with storage limit and threshold
			config.StorageLimit = lo.ToPtr(int64(3000))
			config.StorageThreshold = lo.ToPtr(int64(0))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Should not warn when threshold is 0, even if approaching limit
			result, err := enforcer.CheckStorageQuota(config.UserID, 1500)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		})

		t.Run("Threshold set to -1 with positive limit", func(t *testing.T) {
			userID := baseUserID + 8
			negativeOne := int64(-1) // Will be converted to -1 in config
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(3000)),
				StorageThreshold: &negativeOne,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with storage limit and threshold
			config.StorageLimit = lo.ToPtr(int64(3000))
			config.StorageThreshold = &negativeOne
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Should not warn when threshold is -1, even if approaching limit
			result, err := enforcer.CheckStorageQuota(config.UserID, 1500)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
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
			config.UploadDailyLimit = lo.ToPtr(int64(1000))
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
			uploadDailyLimit := int64(1000)
			uploadThreshold := int64(800)
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
			config.DownloadDailyLimit = lo.ToPtr(int64(2000))
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
			downloadDailyLimit := int64(1000)
			downloadThreshold := int64(800)
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
			config.StorageLimit = lo.ToPtr(int64(3000))
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
			storageLimit := int64(1000)
			storageThreshold := int64(800)
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
			storageLimit := int64(1000)
			storageThreshold := int64(800)
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

		t.Run("Storage addition with limit set to 0 (disabled)", func(t *testing.T) {
			userID := baseUserID + 5
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit: lo.ToPtr(int64(0)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Should be blocked because storage limit is 0 (disabled)
			err := enforcer.RecordStorageChange(userID, 305, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "storage change blocked")
		})

		t.Run("Storage addition with unlimited limit", func(t *testing.T) {
			userID := baseUserID + 6
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit: (*int64)(nil),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Should be allowed because storage limit is unlimited
			err := enforcer.RecordStorageChange(userID, 306, 1000000, "127.0.0.1")
			assert.NoError(t, err)
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
		history, err := enforcer.GetUsageHistory(userID, period, pluginCore.UsageType(usageType))
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

func TestThresholdPolicyEnforcer_ThresholdWarningBehavior(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(40000)

		t.Run("Unlimited limits should not warn", func(t *testing.T) {
			userID := baseUserID + 1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: nil,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Should not warn when limit is unlimited
			result, err := enforcer.CheckUploadQuota(userID, 1000000)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		})

		t.Run("Disabled limits should not warn (should deny)", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(int64(0)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Should deny when limit is 0 (disabled)
			result, err := enforcer.CheckUploadQuota(userID, 100)
			require.NoError(t, err)
			assert.False(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
		})

		t.Run("Normal threshold warnings with positive limits", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(int64(1000)),
				UploadThreshold:  lo.ToPtr(int64(800)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get existing user config and update it
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update config with upload daily limit and threshold
			config.UploadDailyLimit = lo.ToPtr(int64(1000))
			config.UploadThreshold = lo.ToPtr(int64(800))
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold level (400 bytes, so 400+500=900 > 800 threshold)
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 400)

			// Should warn when approaching threshold
			result, err := enforcer.CheckUploadQuota(config.UserID, 500)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		})

		t.Run("Threshold warnings with mixed limit types", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit:   nil, // Unlimited
				UploadTotalLimit:   lo.ToPtr(int64(1000)),
				UploadThreshold:    lo.ToPtr(int64(800)),
				DownloadDailyLimit: nil, // Unlimited
				DownloadTotalLimit: lo.ToPtr(int64(2000)),
				DownloadThreshold:  lo.ToPtr(int64(1500)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config and set daily limits to unlimited
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)
			config.UploadDailyLimit = nil
			config.DownloadDailyLimit = nil
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Create usage that's at threshold level for download (400 bytes, so 400+1200=1600 > 1500 threshold)
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 400)

			// Should not warn for upload daily since it's unlimited
			result, err := enforcer.CheckUploadQuota(config.UserID, 500)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)

			// Should warn for download total since it has a normal limit
			result, err = enforcer.CheckDownloadQuota(config.UserID, 1200)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		})
	}, testOptions())
}

func TestThresholdPolicyEnforcer_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent quota checks", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			baseUserID := uint(50000)
			userID := baseUserID + 1

			uploadDailyLimit := int64(1000)
			uploadThreshold := int64(800)
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

	t.Run("Concurrent quota checks with 0 limit (should all deny)", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			baseUserID := uint(51000)
			userID := baseUserID + 1

			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: lo.ToPtr(int64(0)), // Disabled
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

			// All should succeed (no error in validation)
			for _, err := range errors {
				assert.NoError(t, err)
			}

			// All should be denied because limit is 0
			for _, result := range results {
				assert.False(t, result.Allowed)
				assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
			}
		}, testOptions())
	})

	t.Run("Concurrent quota checks with -1 limit (should all allow)", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			baseUserID := uint(52000)
			userID := baseUserID + 1

			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				UploadDailyLimit: nil,
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
					result, err := enforcer.CheckUploadQuota(userID, 1000000)
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

			// All should be allowed because limit is -1 (unlimited)
			for _, result := range results {
				assert.True(t, result.Allowed)
				assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
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
				StorageLimit:       lo.ToPtr(int64(1000)),
				UploadDailyLimit:   lo.ToPtr(int64(500)),
				DownloadDailyLimit: lo.ToPtr(int64(750)),
				UploadTotalLimit:   lo.ToPtr(int64(2000)),
				DownloadTotalLimit: lo.ToPtr(int64(3000)),
				StorageThreshold:   lo.ToPtr(int64(800)),
				UploadThreshold:    lo.ToPtr(int64(400)),
				DownloadThreshold:  lo.ToPtr(int64(600)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			config, err := enforcer.getUserQuotaConfig(userID)
			assert.NoError(t, err)
			assert.Equal(t, int64(1000), *config.StorageLimit)
			assert.Equal(t, int64(500), *config.UploadDailyLimit)
			assert.Equal(t, int64(750), *config.DownloadDailyLimit)
			assert.Equal(t, int64(2000), *config.UploadTotalLimit)
			assert.Equal(t, int64(3000), *config.DownloadTotalLimit)
			assert.Equal(t, int64(800), *config.StorageThreshold)
			assert.Equal(t, int64(400), *config.UploadThreshold)
			assert.Equal(t, int64(600), *config.DownloadThreshold)
		})

		t.Run("Quota plan limits", func(t *testing.T) {
			userID := baseUserID + 2
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Create a quota plan with threshold values
			storageThreshold := int64(800)
			uploadThreshold := int64(400)
			downloadThreshold := int64(600)
			plan := createTestQuotaPlan(t, ctx, "Test Plan", false, &testPlanLimits{
				StorageLimit:       1000,
				UploadDailyLimit:   500,
				DownloadDailyLimit: 750,
				UploadTotalLimit:   2000,
				DownloadTotalLimit: 3000,
				StorageThreshold:   &storageThreshold,
				UploadThreshold:    &uploadThreshold,
				DownloadThreshold:  &downloadThreshold,
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
			assert.Equal(t, uint64(plan.StorageLimit), *limits.StorageLimit)
			assert.Equal(t, uint64(plan.UploadDailyLimit), *limits.UploadDailyLimit)
			assert.Equal(t, uint64(plan.DownloadDailyLimit), *limits.DownloadDailyLimit)
			assert.Equal(t, uint64(plan.UploadTotalLimit), *limits.UploadTotalLimit)
			assert.Equal(t, uint64(plan.DownloadTotalLimit), *limits.DownloadTotalLimit)
			assert.Equal(t, uint64(*plan.StorageThreshold), *limits.StorageThreshold)
			assert.Equal(t, uint64(*plan.UploadThreshold), *limits.UploadThreshold)
			assert.Equal(t, uint64(*plan.DownloadThreshold), *limits.DownloadThreshold)
		})

		t.Run("Mixed configuration", func(t *testing.T) {
			userID := baseUserID + 3
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:    lo.ToPtr(int64(1000)),
				UploadThreshold: lo.ToPtr(int64(400)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Create a quota plan with threshold values
			storageThreshold := int64(1500)
			downloadThreshold := int64(1200)
			plan := createTestQuotaPlan(t, ctx, "Test Plan 2", false, &testPlanLimits{
				StorageLimit:       2000,
				UploadDailyLimit:   1000,
				DownloadDailyLimit: 1500,
				UploadTotalLimit:   5000,
				DownloadTotalLimit: 7500,
				StorageThreshold:   &storageThreshold,
				DownloadThreshold:  &downloadThreshold,
			})

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Update the user config with mixed configuration
			planID := uint64(plan.ID)
			config.QuotaPlanID = &planID
			config.StorageLimit = lo.ToPtr(int64(1000))   // Override plan's storage limit
			config.UploadThreshold = lo.ToPtr(int64(400)) // Override plan's upload threshold
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, uint64(1000), *limits.StorageLimit)                          // Custom value
			assert.Equal(t, uint64(plan.UploadDailyLimit), *limits.UploadDailyLimit)     // Plan value
			assert.Equal(t, uint64(plan.DownloadDailyLimit), *limits.DownloadDailyLimit) // Plan value
			assert.Equal(t, uint64(plan.UploadTotalLimit), *limits.UploadTotalLimit)     // Plan value
			assert.Equal(t, uint64(plan.DownloadTotalLimit), *limits.DownloadTotalLimit) // Plan value
			assert.Equal(t, uint64(*plan.StorageThreshold), *limits.StorageThreshold)    // Plan value
			assert.Equal(t, uint64(400), *limits.UploadThreshold)                        // Custom value
			assert.Equal(t, uint64(*plan.DownloadThreshold), *limits.DownloadThreshold)  // Plan value
		})

		t.Run("Default plan", func(t *testing.T) {
			userID := baseUserID + 4
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Create a default quota plan with threshold values
			defaultStorageThreshold := int64(400)
			defaultUploadThreshold := int64(200)
			defaultDownloadThreshold := int64(250)
			defaultPlan := createTestQuotaPlan(t, ctx, "Default Plan", true, &testPlanLimits{
				StorageLimit:       500,
				UploadDailyLimit:   250,
				DownloadDailyLimit: 300,
				UploadTotalLimit:   1000,
				DownloadTotalLimit: 1500,
				StorageThreshold:   &defaultStorageThreshold,
				UploadThreshold:    &defaultUploadThreshold,
				DownloadThreshold:  &defaultDownloadThreshold,
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
			assert.Equal(t, uint64(defaultPlan.StorageLimit), *limits.StorageLimit)
			assert.Equal(t, uint64(defaultPlan.UploadDailyLimit), *limits.UploadDailyLimit)
			assert.Equal(t, uint64(defaultPlan.DownloadDailyLimit), *limits.DownloadDailyLimit)
			assert.Equal(t, uint64(defaultPlan.UploadTotalLimit), *limits.UploadTotalLimit)
			assert.Equal(t, uint64(defaultPlan.DownloadTotalLimit), *limits.DownloadTotalLimit)
			assert.Equal(t, uint64(*defaultPlan.StorageThreshold), *limits.StorageThreshold)
			assert.Equal(t, uint64(*defaultPlan.UploadThreshold), *limits.UploadThreshold)
			assert.Equal(t, uint64(*defaultPlan.DownloadThreshold), *limits.DownloadThreshold)
		})

		t.Run("Limits set to 0 (disabled)", func(t *testing.T) {
			userID := baseUserID + 5
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:       lo.ToPtr(int64(0)),
				UploadDailyLimit:   lo.ToPtr(int64(0)),
				DownloadDailyLimit: lo.ToPtr(int64(0)),
				UploadTotalLimit:   lo.ToPtr(int64(0)),
				DownloadTotalLimit: lo.ToPtr(int64(0)),
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, uint64(0), *limits.StorageLimit)
			assert.Equal(t, uint64(0), *limits.UploadDailyLimit)
			assert.Equal(t, uint64(0), *limits.DownloadDailyLimit)
			assert.Equal(t, uint64(0), *limits.UploadTotalLimit)
			assert.Equal(t, uint64(0), *limits.DownloadTotalLimit)
		})

		t.Run("Limits set to -1 (unlimited)", func(t *testing.T) {
			userID := baseUserID + 6
			negativeOne := int64(-1) // Will be converted to -1
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:       &negativeOne,
				UploadDailyLimit:   &negativeOne,
				DownloadDailyLimit: &negativeOne,
				UploadTotalLimit:   &negativeOne,
				DownloadTotalLimit: &negativeOne,
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			// Get the user config and set limits to -1
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)
			negativeOneInt64 := int64(-1)
			config.StorageLimit = &negativeOneInt64
			config.UploadDailyLimit = &negativeOneInt64
			config.DownloadDailyLimit = &negativeOneInt64
			config.UploadTotalLimit = &negativeOneInt64
			config.DownloadTotalLimit = &negativeOneInt64
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			limits, err := enforcer.resolveEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Nil(t, limits.StorageLimit)       // -1 converted to nil (unlimited)
			assert.Nil(t, limits.UploadDailyLimit)   // -1 converted to nil (unlimited)
			assert.Nil(t, limits.DownloadDailyLimit) // -1 converted to nil (unlimited)
			assert.Nil(t, limits.UploadTotalLimit)   // -1 converted to nil (unlimited)
			assert.Nil(t, limits.DownloadTotalLimit) // -1 converted to nil (unlimited)
		})

		t.Run("Thresholds set to 0 or unlimited with positive limits", func(t *testing.T) {
			userID := baseUserID + 7
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(1000)),
				StorageThreshold: nil, // Unlimited
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			config, err := enforcer.getUserQuotaConfig(userID)
			assert.NoError(t, err)
			assert.Equal(t, int64(1000), *config.StorageLimit)
			assert.Nil(t, config.StorageThreshold) // Unlimited
		})

		t.Run("Threshold validation with positive limits", func(t *testing.T) {
			userID := baseUserID + 8
			createTestUser(t, ctx, userID, models.EnforcementPolicyThreshold, &testUserLimits{
				StorageLimit:     lo.ToPtr(int64(1000)),
				StorageThreshold: lo.ToPtr(int64(1500)), // Invalid: threshold > limit
			})
			enforcer := NewThresholdPolicyEnforcer(ctx)

			config, err := enforcer.getUserQuotaConfig(userID)
			assert.NoError(t, err)
			assert.Equal(t, int64(1000), *config.StorageLimit)
			assert.Equal(t, int64(1500), *config.StorageThreshold)
		})
	}, testOptions())
}
