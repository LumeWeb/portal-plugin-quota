package policies

import (
	"sync"
	"testing"
	"time"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestHardLimitsPolicyEnforcer_ValidateLimitValue tests the validateLimitValue function
func TestHardLimitsPolicyEnforcer_ValidateLimitValue(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewHardLimitsPolicyEnforcer(ctx)

		t.Run("Valid positive limit values", func(t *testing.T) {
			validValues := []int64{
				1,
				100,
				1000,
				int64(units.GB),
				int64(units.TB),
				int64(units.PB),
				int64(units.PiB), // Maximum reasonable value
			}

			for _, value := range validValues {
				err := enforcer.validateLimitValue(value)
				assert.NoError(t, err, "Value %d should be valid", value)
			}
		})

		t.Run("Valid zero limit value (disabled)", func(t *testing.T) {
			// 0 means disabled - should be valid
			err := enforcer.validateLimitValue(0)
			assert.NoError(t, err, "Zero should be a valid limit value (disabled)")
		})

		t.Run("Valid negative one limit value (unlimited)", func(t *testing.T) {
			// -1 means unlimited - should be valid
			err := enforcer.validateLimitValue(-1)
			assert.NoError(t, err, "Negative one should be a valid limit value (unlimited)")
		})

		t.Run("Unreasonably large limit values", func(t *testing.T) {
			// Values larger than 1 PiB should be rejected
			unreasonableValues := []int64{
				int64(units.PiB) + 1,
				int64(units.PiB) * 10,
				int64(units.PiB) * 1000,
			}

			for _, value := range unreasonableValues {
				err := enforcer.validateLimitValue(value)
				assert.Error(t, err, "Value %d should be unreasonably large and invalid", value)
			}
		})
	}, testOptions())
}

func TestHardLimitsPolicyEnforcer_CheckUploadQuota(t *testing.T) {
	t.Run("Within daily limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(1000)

			// Create a test user
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test within daily limit
			result, err := enforcer.CheckUploadQuota(config, 500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Exceeding daily limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(2000)

			// Create a test user
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 600)

			// Test exceeding daily limit
			result, err := enforcer.CheckUploadQuota(config, 500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, false, models.QuotaCheckReasonLimitExceeded, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Exceeding total limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(2000)

			// Create a test user with high daily limit but lower total limit
			userID := baseUserID + 2
			uploadDailyLimit := int64(10000)
			uploadTotalLimit := int64(1000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Record uploads to accumulate usage under daily limit but approaching total
			err = enforcer.RecordUpload(userID, 200, 750, "127.0.0.1")
			require.NoError(t, err)

			// Test exceeding total limit
			result, err := enforcer.CheckUploadQuota(config, 500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, false, models.QuotaCheckReasonLimitExceeded, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Invalid bytes", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(3000)

			// Create a test user
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes (0)
			result, err := enforcer.CheckUploadQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota(t *testing.T) {
	t.Run("Within daily limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(4000)

			// Create a test user
			userID := baseUserID + 1
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test within daily limit
			result, err := enforcer.CheckDownloadQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Exceeding daily limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(5000)

			// Create a test user
			userID := baseUserID + 1
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 1500)

			// Test exceeding daily limit
			result, err := enforcer.CheckDownloadQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, false, models.QuotaCheckReasonLimitExceeded, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Exceeding total limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(5000)

			// Create a test user with high daily limit but lower total limit
			userID := baseUserID + 2
			downloadDailyLimit := int64(10000)
			downloadTotalLimit := int64(2000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Record downloads to accumulate usage under daily limit but approaching total
			err = enforcer.RecordDownload(userID, 200, 1500, "127.0.0.1")
			require.NoError(t, err)

			// Test exceeding total limit
			result, err := enforcer.CheckDownloadQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, false, models.QuotaCheckReasonLimitExceeded, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Invalid bytes", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(6000)

			// Create a test user
			userID := baseUserID + 1
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes (0)
			result, err := enforcer.CheckDownloadQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_CheckStorageQuota(t *testing.T) {
	t.Run("Within storage limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(7000)

			// Create a test user
			userID := baseUserID + 1
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit: &storageLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test within storage limit
			result, err := enforcer.CheckStorageQuota(config, 1500)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, true, models.QuotaCheckReasonOK, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Exceeding storage limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(8000)

			// Create a test user
			userID := baseUserID + 1
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit: &storageLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Create usage that's close to limit
			createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 2500)

			// Test exceeding storage limit
			result, err := enforcer.CheckStorageQuota(config, 1000)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, false, models.QuotaCheckReasonLimitExceeded, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Exceeding total limit", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(8000)

			// Create a test user with storage limit and upload total limit
			userID := baseUserID + 2
			storageLimit := int64(3000)
			uploadTotalLimit := int64(500)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit:     &storageLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Record uploads to accumulate usage under daily limit but approaching total
			err = enforcer.RecordUpload(userID, 300, 400, "127.0.0.1")
			require.NoError(t, err)

			// Test exceeding total limit
			result, err := enforcer.CheckUploadQuota(config, 200)
			require.NoError(t, err)
			assertQuotaCheckResult(t, result, false, models.QuotaCheckReasonLimitExceeded, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits))
		}, testOptions())
	})

	t.Run("Invalid bytes", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(9000)

			// Create a test user
			userID := baseUserID + 1
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit: &storageLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes (0)
			result, err := enforcer.CheckStorageQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
			assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_RecordUpload(t *testing.T) {
	t.Run("Successful upload recording", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(10000)

			// Create a test user
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test successful upload recording
			err = enforcer.RecordUpload(userID, 100, 500, "127.0.0.1")
			assert.NoError(t, err)

			// Verify the usage was recorded
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(500), usage.BytesUploaded)
		}, testOptions())
	})

	t.Run("Upload that exceeds quota", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(11000)

			// Create a test user
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
				UploadTotalLimit: &uploadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test upload that exceeds quota
			err = enforcer.RecordUpload(userID, 101, 1500, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "upload blocked")
		}, testOptions())
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Test invalid user ID
			err := enforcer.RecordUpload(0, 102, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		}, testOptions())
	})

	t.Run("Invalid bytes", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Create a test user
			userID := uint(12001)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)
			err = ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test invalid bytes
			err = enforcer.RecordUpload(userID, 103, 0, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_RecordDownload(t *testing.T) {
	t.Run("Successful download recording", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(13000)

			// Create a test user
			userID := baseUserID + 1
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test successful download recording
			err = enforcer.RecordDownload(userID, 200, 1000, "127.0.0.1")
			assert.NoError(t, err)

			// Verify the usage was recorded
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(1000), usage.BytesDownloaded)
		}, testOptions())
	})

	t.Run("Download that exceeds quota", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(14000)

			// Create a test user
			userID := baseUserID + 1
			downloadDailyLimit := int64(1000)
			downloadTotalLimit := int64(5000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
			require.NoError(t, err)

			// Test download that exceeds quota
			err = enforcer.RecordDownload(userID, 201, 1500, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "download blocked")
		}, testOptions())
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Test invalid user ID
			err := enforcer.RecordDownload(0, 202, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		}, testOptions())
	})

	t.Run("Invalid bytes", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Create a test user
			userID := uint(15001)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			// Test invalid bytes
			err := enforcer.RecordDownload(userID, 203, 0, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_RecordStorageChange(t *testing.T) {
	t.Run("Successful storage addition recording", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(16000)

			// Create a test user
			userID := baseUserID + 1
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit: &storageLimit,
			})

			// Test successful storage addition recording
			err := enforcer.RecordStorageChange(userID, 300, 1500, "127.0.0.1")
			assert.NoError(t, err)

			// Verify the usage was recorded
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(1500), usage.BytesStored)
		}, testOptions())
	})

	t.Run("Storage addition that exceeds quota", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(17000)

			// Create a test user
			userID := baseUserID + 1
			storageLimit := int64(1000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit: &storageLimit,
			})

			// Test storage addition that exceeds quota
			err := enforcer.RecordStorageChange(userID, 301, 1500, "127.0.0.1")
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "storage change blocked")
		}, testOptions())
	})

	t.Run("Storage removal (no quota enforcement)", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(18000)

			// Create a test user
			userID := baseUserID + 1
			storageLimit := int64(1000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit: &storageLimit,
			})

			// Test storage removal
			err := enforcer.RecordStorageChange(userID, 302, -500, "127.0.0.1")
			assert.NoError(t, err)
		}, testOptions())
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Test invalid user ID
			err := enforcer.RecordStorageChange(0, 303, 100, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		}, testOptions())
	})

	t.Run("Invalid bytes", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Create a test user
			userID := uint(19001)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			// Test invalid bytes (0)
			err := enforcer.RecordStorageChange(userID, 304, 0, "127.0.0.1")
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_GetDetailedUsage(t *testing.T) {
	t.Run("Get detailed usage", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(20000)

			userID := baseUserID + 1

			// Create a test user with limits
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit:   &uploadDailyLimit,
				UploadTotalLimit:   &uploadTotalLimit,
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
				StorageLimit:       &storageLimit,
			})

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
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			start := time.Now().Add(-time.Hour)
			end := time.Now().Add(time.Hour)

			// Test with invalid user ID
			_, err := enforcer.GetDetailedUsage(0, start, end)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_GetCurrentUsage(t *testing.T) {
	t.Run("Get current usage", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(21000)

			userID := baseUserID + 1

			// Create a test user with limits
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit:   &uploadDailyLimit,
				UploadTotalLimit:   &uploadTotalLimit,
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
				StorageLimit:       &storageLimit,
			})

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
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Test with invalid user ID
			_, err := enforcer.GetCurrentUsage(0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_GetUsageHistory(t *testing.T) {
	t.Run("Get usage history", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(22000)

			userID := baseUserID + 1
			usageType := models.UsageTypeUpload
			period := 30 // days

			// Create a test user with limits
			uploadDailyLimit := int64(1000)
			uploadTotalLimit := int64(5000)
			downloadDailyLimit := int64(2000)
			downloadTotalLimit := int64(10000)
			storageLimit := int64(3000)

			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit:   &uploadDailyLimit,
				UploadTotalLimit:   &uploadTotalLimit,
				DownloadDailyLimit: &downloadDailyLimit,
				DownloadTotalLimit: &downloadTotalLimit,
				StorageLimit:       &storageLimit,
			})

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
	})

	t.Run("Invalid user ID", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			usageType := models.UsageTypeUpload
			period := 30 // days

			// Test with invalid user ID
			_, err := enforcer.GetUsageHistory(0, period, usageType)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		}, testOptions())
	})
}

func TestHardLimitsPolicyEnforcer_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent quota checks", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(40000)

			// Create a test user
			userID := baseUserID + 1
			uploadDailyLimit := int64(1000)
			config := createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				UploadDailyLimit: &uploadDailyLimit,
			})

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
					result, err := enforcer.CheckUploadQuota(config, 100)
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

func TestHardLimitsPolicyEnforcer_getEffectiveLimits(t *testing.T) {
	t.Run("Custom limits only", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(23000)

			userID := baseUserID + 1

			// Test with custom limits
			storageLimit := int64(1000)
			uploadDailyLimit := int64(500)
			downloadDailyLimit := int64(750)
			uploadTotalLimit := int64(2000)
			downloadTotalLimit := int64(3000)

			config := createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				StorageLimit:       &storageLimit,
				UploadDailyLimit:   &uploadDailyLimit,
				DownloadDailyLimit: &downloadDailyLimit,
				UploadTotalLimit:   &uploadTotalLimit,
				DownloadTotalLimit: &downloadTotalLimit,
			})

			limits, err := enforcer.getEffectiveLimits(config)
			assert.NoError(t, err)
			assert.Equal(t, userID, limits.UserID)
			assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits), limits.EnforcementPolicy)
			assert.Equal(t, uint64(storageLimit), *limits.StorageLimit)
			assert.Equal(t, uint64(uploadDailyLimit), *limits.UploadDailyLimit)
			assert.Equal(t, uint64(downloadDailyLimit), *limits.DownloadDailyLimit)
			assert.Equal(t, uint64(uploadTotalLimit), *limits.UploadTotalLimit)
			assert.Equal(t, uint64(downloadTotalLimit), *limits.DownloadTotalLimit)
		}, testOptions())
	})

	t.Run("Quota plan limits", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(24000)

			userID := baseUserID + 1

			// Test with quota plan
			plan := createTestQuotaPlan(t, ctx, "Test Plan", true, &testPlanLimits{
				StorageLimit:       int64(2000),
				UploadDailyLimit:   int64(1000),
				DownloadDailyLimit: int64(1500),
				UploadTotalLimit:   int64(5000),
				DownloadTotalLimit: int64(7500),
			})

			planID := uint64(plan.ID)
			configWithPlan := createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				QuotaPlanID: &planID,
			})

			limitsWithPlan, err := enforcer.getEffectiveLimits(configWithPlan)
			assert.NoError(t, err)
			assert.Equal(t, uint64(plan.StorageLimit), *limitsWithPlan.StorageLimit)
			assert.Equal(t, uint64(plan.UploadDailyLimit), *limitsWithPlan.UploadDailyLimit)
			assert.Equal(t, uint64(plan.DownloadDailyLimit), *limitsWithPlan.DownloadDailyLimit)
			assert.Equal(t, uint64(plan.UploadTotalLimit), *limitsWithPlan.UploadTotalLimit)
			assert.Equal(t, uint64(plan.DownloadTotalLimit), *limitsWithPlan.DownloadTotalLimit)
		}, testOptions())
	})

	t.Run("Mixed configuration", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			baseUserID := uint(25000)

			userID := baseUserID + 1

			// Test with mixed configuration (plan with custom overrides)
			plan := createTestQuotaPlan(t, ctx, "Test Plan 2", false, &testPlanLimits{
				StorageLimit:       int64(2000),
				UploadDailyLimit:   int64(1000),
				DownloadDailyLimit: int64(1500),
				UploadTotalLimit:   int64(5000),
				DownloadTotalLimit: int64(7500),
			})

			storageLimit := int64(1000)
			uploadDailyLimit := int64(500)
			planID := uint64(plan.ID)

			configWithMixed := createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
				QuotaPlanID:      &planID,
				StorageLimit:     &storageLimit,     // Override plan's storage limit
				UploadDailyLimit: &uploadDailyLimit, // Override plan's upload daily limit
			})

			limitsWithMixed, err := enforcer.getEffectiveLimits(configWithMixed)
			assert.NoError(t, err)
			assert.Equal(t, uint64(storageLimit), *limitsWithMixed.StorageLimit)                  // Custom value
			assert.Equal(t, uint64(uploadDailyLimit), *limitsWithMixed.UploadDailyLimit)          // Custom value
			assert.Equal(t, uint64(plan.DownloadDailyLimit), *limitsWithMixed.DownloadDailyLimit) // Plan value
			assert.Equal(t, uint64(plan.UploadTotalLimit), *limitsWithMixed.UploadTotalLimit)     // Plan value
			assert.Equal(t, uint64(plan.DownloadTotalLimit), *limitsWithMixed.DownloadTotalLimit) // Plan value
		}, testOptions())
	})
}
