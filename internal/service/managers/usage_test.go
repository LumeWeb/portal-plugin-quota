package managers

import (
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreMocks "go.lumeweb.com/portal/core/testing/mocks"
	"go.lumeweb.com/portal/db/models"
)

// Test constants for byte values
const (
	testUsageBytesSmall      = 100
	testUsageBytesMedium     = 200
	testUsageBytesLarge      = 300
	testUsageBytesExtraLarge = 500
	testUsageBytesHuge       = 600
	testUsageBytesMassive    = 1000
)

// Test constants for user counts
const (
	testUserCountSmall = 3
	testUserCountLarge = 10
)

// Test constants for goroutine counts
const (
	testGoroutineCount = 5
)

// Test constants for validation values
const (
	testValidUserID   = 1
	testInvalidUserID = 0
	testValidBytes    = 100
)

// Test options for different configurations
func testOptionsWithSharedUsageDisabled() coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    false,
		SharedUsagePrecision: 0,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

func testOptionsWithSharedUsageEnabled() coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    true,
		SharedUsagePrecision: 0,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

func testOptionsWithSharedUsagePrecision(precision int) coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    true,
		SharedUsagePrecision: precision,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

// Test options with precision 0 for backward compatibility with existing tests
func testOptionsWithPrecision0() coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    true,
		SharedUsagePrecision: 0,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

// TestUsageManager_RecordUpload_ValidInput_Success tests the RecordUpload method
func TestUsageManager_RecordUpload_ValidInput_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesExtraLarge)
		ip := "192.168.1.1"

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordUpload(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeUpload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Uploads are not shared

		// Verify the daily quota was updated
		today := time.Now().UTC().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, bytes, dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())
}

func TestUsageManager_GetUserQuotaConfig_ExistingConfig_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		config, err := usageManager.GetUserQuotaConfig(ctx, userID)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, userID, config.UserID)
		assert.Equal(t, pluginModels.EnforcementPolicyHardLimits, config.EnforcementPolicy)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_GetUserQuotaConfig_NonExistentConfig_CreatesDefault(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		usageManager := NewUsageManager(ctx)

		config, err := usageManager.GetUserQuotaConfig(ctx, userID)
		require.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, userID, config.UserID)
		assert.Equal(t, pluginModels.EnforcementPolicyHardLimits, config.EnforcementPolicy) // Default policy

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_GetCurrentUsage_NoUsageRecords_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		usage, err := usageManager.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.NotNil(t, usage)
		assert.Equal(t, userID, usage.UserID)
		assert.Equal(t, uint64(0), usage.BytesUploaded)
		assert.Equal(t, uint64(0), usage.BytesDownloaded)
		assert.Equal(t, uint64(0), usage.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_GetCurrentUsage_WithUsageRecords_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Create usage records
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testUsageBytesSmall, "192.168.1.1")
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeDownload, testUsageBytesMedium, "192.168.1.1")
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeStorageAdd, testUsageBytesLarge, "192.168.1.1")

		usageManager := NewUsageManager(ctx)

		usage, err := usageManager.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.NotNil(t, usage)
		assert.Equal(t, userID, usage.UserID)
		assert.Equal(t, uint64(testUsageBytesSmall), usage.BytesUploaded)
		assert.Equal(t, uint64(testUsageBytesMedium), usage.BytesDownloaded)
		assert.Equal(t, uint64(testUsageBytesLarge), usage.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_GetUsageHistory_RecentUsageHistory_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Create usage records with different timestamps
		now := time.Now()
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testUsageBytesSmall, "192.168.1.1")

		// Create a record in the past
		oldDetail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      pluginModels.UsageTypeUpload,
			Bytes:     testUsageBytesMedium,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour), // 2 days ago
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		usageManager := NewUsageManager(ctx)

		history, err := usageManager.GetUsageHistory(ctx, userID, 1, pluginModels.UsageTypeUpload)
		require.NoError(t, err)
		assert.Len(t, history, 1) // Only the recent record
		assert.Equal(t, uint64(testUsageBytesSmall), history[0].Bytes)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_GetUsageHistory_AllUsageHistory_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Create usage records with different timestamps
		now := time.Now()
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testUsageBytesSmall, "192.168.1.1")

		// Create a record in the past
		oldDetail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      pluginModels.UsageTypeUpload,
			Bytes:     testUsageBytesMedium,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour), // 2 days ago
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		usageManager := NewUsageManager(ctx)

		history, err := usageManager.GetUsageHistory(ctx, userID, 3, pluginModels.UsageTypeUpload)
		require.NoError(t, err)
		assert.Len(t, history, 2)                                       // Both records
		assert.Equal(t, uint64(testUsageBytesMedium), history[0].Bytes) // Older record first
		assert.Equal(t, uint64(testUsageBytesSmall), history[1].Bytes)  // Newer record second

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_GetDetailedUsage_WithinTimeRange_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Create usage records
		now := time.Now()
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testUsageBytesSmall, "192.168.1.1")
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeDownload, testUsageBytesMedium, "192.168.1.1")

		// Create a record outside the time range
		oldDetail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     testUsageBytesLarge,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour),
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		usageManager := NewUsageManager(ctx)

		start := now.Add(-24 * time.Hour)
		end := now.Add(24 * time.Hour)

		details, err := usageManager.GetDetailedUsage(ctx, userID, start, end)
		require.NoError(t, err)
		assert.Len(t, details, 2) // Only records within time range

		// Verify records are in ascending order by timestamp (as per the implementation)
		assert.True(t, details[0].Timestamp.Before(details[1].Timestamp))

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_RecordUserUsageDetail_RecordDetail_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		detail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  uploadID,
			Type:      pluginModels.UsageTypeUpload,
			Bytes:     testUsageBytesSmall,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordUserUsageDetail(ctx, detail, nil)
		require.NoError(t, err)

		// Verify the record was created
		var savedDetail pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).First(&savedDetail).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedDetail.UserID)
		assert.Equal(t, uint64(testUsageBytesSmall), savedDetail.Bytes)
		assert.Equal(t, pluginModels.UsageTypeUpload, savedDetail.Type)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_UpdateDailyUsage_CreateNewRecord_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		err := usageManager.UpdateDailyUsage(ctx, userID, pluginModels.UsageTypeUpload, int64(testUsageBytesSmall))
		require.NoError(t, err)

		// Verify the record was created
		var dailyQuota pluginModels.UserQuota
		today := time.Now().UTC().Truncate(24 * time.Hour)
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, userID, dailyQuota.UserID)
		assert.Equal(t, uint64(testUsageBytesSmall), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_UpdateDailyUsage_UpdateExistingRecord_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// First create a record
		err := usageManager.UpdateDailyUsage(ctx, userID, pluginModels.UsageTypeUpload, int64(testUsageBytesSmall))
		require.NoError(t, err)

		// Then update it
		err = usageManager.UpdateDailyUsage(ctx, userID, pluginModels.UsageTypeUpload, int64(testUsageBytesSmall/2))
		require.NoError(t, err)

		// Verify the record was updated
		var dailyQuota pluginModels.UserQuota
		today := time.Now().UTC().Truncate(24 * time.Hour)
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(testUsageBytesSmall+testUsageBytesSmall/2), dailyQuota.BytesUploaded) // testUsageBytesSmall + testUsageBytesSmall/2

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestUsageManager_UpdateDailyUsage_DifferentUsageTypes_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Add different types of usage
		err := usageManager.UpdateDailyUsage(ctx, userID, pluginModels.UsageTypeUpload, int64(testUsageBytesSmall))
		require.NoError(t, err)
		err = usageManager.UpdateDailyUsage(ctx, userID, pluginModels.UsageTypeDownload, int64(testUsageBytesMedium))
		require.NoError(t, err)
		err = usageManager.UpdateDailyUsage(ctx, userID, pluginModels.UsageTypeStorageAdd, int64(testUsageBytesLarge))
		require.NoError(t, err)

		// Verify all types were recorded correctly
		var dailyQuota pluginModels.UserQuota
		today := time.Now().UTC().Truncate(24 * time.Hour)
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(testUsageBytesSmall), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(testUsageBytesMedium), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(testUsageBytesLarge), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

// TestUsageManager_RecordDownload_ValidInput_Success tests the RecordDownload method
func TestUsageManager_RecordDownload_ValidInput_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesExtraLarge)
		ip := "192.168.1.1"

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordDownload(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Not shared when disabled

		// Verify the daily quota was updated
		today := time.Now().UTC().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, bytes, dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageDisabled())
}

// TestUsageManager_RecordDownload_AnonymousWithSharedUsage_Success tests anonymous download recording with shared usage
func TestUsageManager_RecordDownload_AnonymousWithSharedUsage_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesHuge)
		ip := "192.168.1.1"

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)

		userID1 := dataManager.NextUserID()
		userID2 := dataManager.NextUserID()
		userID3 := dataManager.NextUserID()

		pins := []*models.Pin{
			{UserID: userID1},
			{UserID: userID2},
			{UserID: userID3},
		}
		mockPinService.EXPECT().GetPinsByUploadID(mock.Anything, uploadID).Return(pins, nil)

		// Create test users
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, limits)
		dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, limits)
		dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Anonymous download (userID = 0) - should be shared among all pinned users
		err := usageManager.RecordDownload(ctx, 0, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded with shared calculation for each pinned user
		for _, userID := range []uint{userID1, userID2, userID3} {
			var usageDetails []pluginModels.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
			assert.Equal(t, uint64(testUsageBytesHuge/testUserCountSmall), usageDetails[0].Bytes) // testUsageBytesHuge/testUserCountSmall = testUsageBytesMedium
			assert.Equal(t, ip, usageDetails[0].IP)
			assert.Equal(t, uint(testUserCountSmall), usageDetails[0].SharedWith)

			// Verify the daily quota was updated
			today := time.Now().UTC().Truncate(24 * time.Hour)
			var dailyQuota pluginModels.UserQuota
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
			assert.Equal(t, uint64(testUsageBytesHuge/testUserCountSmall), dailyQuota.BytesDownloaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesStored)
		}

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordDownload_AnonymousNoPins_Skips tests anonymous download with no pinned users
func TestUsageManager_RecordDownload_AnonymousNoPins_Skips(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesHuge)
		ip := "192.168.1.1"

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)
		pins := []*models.Pin{}
		mockPinService.EXPECT().GetPinsByUploadID(mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		// Anonymous download with no pinned users - should skip gracefully
		err := usageManager.RecordDownload(ctx, 0, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify no usage details were recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Empty(t, usageDetails)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordDownload_AnonymousDuplicatePins_Deduplicates tests anonymous download with duplicate pins
func TestUsageManager_RecordDownload_AnonymousDuplicatePins_Deduplicates(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesHuge)
		ip := "192.168.1.1"

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)

		userID1 := dataManager.NextUserID()
		userID2 := dataManager.NextUserID()
		userID3 := dataManager.NextUserID()

		pins := []*models.Pin{
			{UserID: userID1},
			{UserID: userID1}, // Duplicate
			{UserID: userID2},
			{UserID: userID2}, // Duplicate
			{UserID: userID3},
		}
		mockPinService.EXPECT().GetPinsByUploadID(mock.Anything, uploadID).Return(pins, nil)

		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, limits)
		dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, limits)
		dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Anonymous download - should deduplicate users correctly
		err := usageManager.RecordDownload(ctx, 0, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify each unique user has exactly one usage record with correct shared bytes
		for _, userID := range []uint{userID1, userID2, userID3} {
			var usageDetails []pluginModels.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, uint64(testUsageBytesHuge/testUserCountSmall), usageDetails[0].Bytes)
			assert.Equal(t, uint(testUserCountSmall), usageDetails[0].SharedWith)
		}

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordDownload_AnonymousPinServiceError_ReturnsError tests anonymous download with pin service error
func TestUsageManager_RecordDownload_AnonymousPinServiceError_ReturnsError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesHuge)
		ip := "192.168.1.1"

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)
		mockPinService.EXPECT().GetPinsByUploadID(mock.Anything, uploadID).Return([]*models.Pin{}, fmt.Errorf("pin service error"))

		usageManager := NewUsageManager(ctx)

		// Anonymous download with pin service error - should return error
		err := usageManager.RecordDownload(ctx, 0, uploadID, bytes, ip)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get pinned users")

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordDownload_AnonymousPinServiceUnavailable_ReturnsError tests anonymous download with nil pin service
func TestUsageManager_RecordDownload_AnonymousPinServiceUnavailable_ReturnsError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testUsageBytesHuge)
		ip := "192.168.1.1"

		usageManager := NewUsageManager(ctx)
		usageManager.pinService = nil

		// Anonymous download with nil pin service - should return error
		err := usageManager.RecordDownload(ctx, 0, uploadID, bytes, ip)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pin service not available")

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordStorageChange_ValidInput_Success tests the RecordStorageChange method
func TestUsageManager_RecordStorageChange_ValidInput_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := int64(testUsageBytesExtraLarge)
		ip := "192.168.1.1"

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeStorageAdd, usageDetails[0].Type)
		assert.Equal(t, uint64(bytes), usageDetails[0].Bytes)
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Not shared when disabled

		// Verify the daily quota was updated
		today := time.Now().UTC().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(bytes), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageDisabled())
}

// TestUsageManager_RecordStorageChange_Remove_Success tests storage removal recording
func TestUsageManager_RecordStorageChange_Remove_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := int64(-testUsageBytesLarge)
		ip := "192.168.1.1"

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeStorageRemove, usageDetails[0].Type)
		assert.Equal(t, uint64(-bytes), usageDetails[0].Bytes) // Positive bytes in record
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Removals are not shared

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageDisabled())
}

// TestUsageManager_calculateSharedBytes_MultipleScenarios_Success tests the calculateSharedBytes method
func TestUsageManager_calculateSharedBytes_MultipleScenarios_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(testUsageBytesMassive) // 1000 bytes
		userCount := uint(4)                        // Shared among 4 users

		sharedBytes := pluginCore.CalculateSharedBytes(totalBytes, userCount, 0)
		// Verifies exact division: 1000 bytes / 4 users = 250 bytes per user
		expected := uint64(250)
		assert.Equal(t, expected, sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(1)               // 1 byte total
		userCount := uint(testUserCountLarge) // Shared among 10 users

		sharedBytes := pluginCore.CalculateSharedBytes(totalBytes, userCount, 0)
		// Verifies floor division behavior: 1 byte / 10 users = 0 bytes per user (rounded down)
		assert.Equal(t, uint64(0), sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(testUsageBytesMassive) // 1000 bytes
		userCount := uint(testUserCountSmall)       // Shared among 3 users

		sharedBytes := pluginCore.CalculateSharedBytes(totalBytes, userCount, 2)
		// Verifies precision handling with 2 decimal places:
		// 1000 bytes / 3 users = 333.333... bytes per user
		// With 2 decimal places precision: 333.33, rounded up to 333.34
		// Actual bytes charged per user: 334 bytes (333.34 rounded up)
		// Rounding mode: ceil to precision, then ceil to whole bytes
		assert.Equal(t, uint64(334), sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsagePrecision(2))

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(testUsageBytesMassive) // 1000 bytes
		userCount := uint(testUserCountSmall)       // Shared among 3 users

		sharedBytes := pluginCore.CalculateSharedBytes(totalBytes, userCount, 10)
		// Verifies precision handling with 10 decimal places:
		// 1000 bytes / 3 users = 333.333... bytes per user
		// With 10 decimal places precision: 333.3333333333, rounded up to 333.3333333334
		// Actual bytes charged per user: 334 bytes (333.3333333334 rounded up)
		// Rounding mode: ceil to precision, then ceil to whole bytes
		assert.Equal(t, uint64(334), sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsagePrecision(10))

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(testUsageBytesMassive) // 1000 bytes
		userCount := uint(testUserCountSmall)       // Shared among 3 users

		sharedBytes := pluginCore.CalculateSharedBytes(totalBytes, userCount, 1)
		// Verifies precision handling with 1 decimal place:
		// 1000 bytes / 3 users = 333.333... bytes per user
		// With 1 decimal place precision: 333.3, rounded up to 333.4
		// Actual bytes charged per user: 334 bytes (333.4 rounded up)
		// Rounding mode: ceil to precision, then ceil to whole bytes
		assert.Equal(t, uint64(334), sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsagePrecision(1))
}

// TestUsageManager_ConcurrentAccess_MultipleOperations_Success tests concurrent usage recording
func TestUsageManager_ConcurrentAccess_MultipleOperations_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Run concurrent upload recordings
		var errors []error
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := testGoroutineCount
		bytesPerGoroutine := uint64(testUsageBytesSmall)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordUpload(ctx, userID, dataManager.NextUploadID(), bytesPerGoroutine, "192.168.1.1")
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

		// Verify total bytes recorded
		usage, err := usageManager.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, bytesPerGoroutine*uint64(numGoroutines), usage.BytesUploaded)

		// Optional: verify number of details
		var detailsCount int64
		err = ctx.DB().Model(&pluginModels.UserUsageDetail{}).Where("user_id = ? AND type = ?", userID, pluginModels.UsageTypeUpload).Count(&detailsCount).Error
		require.NoError(t, err)
		assert.Equal(t, int64(numGoroutines), detailsCount)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Run concurrent download recordings
		var errors []error
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := testGoroutineCount
		bytesPerGoroutine := uint64(testUsageBytesMedium)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordDownload(ctx, userID, dataManager.NextUploadID(), bytesPerGoroutine, "192.168.1.2")
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

		// Verify total bytes recorded
		usage, err := usageManager.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, bytesPerGoroutine*uint64(numGoroutines), usage.BytesDownloaded)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageDisabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Run concurrent storage addition recordings
		var errors []error
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := testGoroutineCount
		bytesPerGoroutine := int64(testUsageBytesLarge)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordStorageChange(ctx, userID, dataManager.NextUploadID(), bytesPerGoroutine, "192.168.1.3")
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

		// Verify total bytes recorded
		usage, err := usageManager.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(bytesPerGoroutine)*uint64(numGoroutines), usage.BytesStored)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageDisabled())
}

// TestUsageManager_Validation_MultipleScenarios_Success tests validation methods
func TestUsageManager_Validation_MultipleScenarios_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateUserID(testValidUserID)
		assert.NoError(t, err)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateUserID(testInvalidUserID)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateBytes(testValidBytes)
		assert.NoError(t, err)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateBytes(0)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidBytes)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())
}

// TestUsageManager_RecordStorageChange_MinInt64 tests handling of math.MinInt64 to prevent overflow
func TestUsageManager_RecordStorageChange_MinInt64(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		ip := "192.168.1.1"

		// Create test user
		limits := &testdata.TestUserLimits{
			StorageLimit:       nil,
			UploadDailyLimit:   nil,
			DownloadDailyLimit: nil,
			UploadTotalLimit:   nil,
			DownloadTotalLimit: nil,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		usageManager := NewUsageManager(ctx)

		// Test with math.MinInt64
		bytes := int64(math.MinInt64)

		// This should not cause an overflow panic when converting to uint64
		err := usageManager.RecordStorageChange(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify that a record was created with the correct values
		var details []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&details).Error
		require.NoError(t, err)
		assert.Len(t, details, 1)

		detail := details[0]
		assert.Equal(t, userID, detail.UserID)
		assert.Equal(t, uploadID, detail.UploadID)
		assert.Equal(t, pluginModels.UsageTypeStorageRemove, detail.Type)
		// math.MinInt64 should be converted to math.MaxInt64
		assert.Equal(t, uint64(math.MaxInt64), detail.Bytes)
		assert.Equal(t, uint(1), detail.SharedWith)
		assert.Equal(t, ip, detail.IP)
		assert.WithinDuration(t, time.Now(), detail.Timestamp, time.Second*10)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
