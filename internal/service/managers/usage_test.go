package managers

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
	testBytesSmall      = 100
	testBytesMedium     = 200
	testBytesLarge      = 300
	testBytesExtraLarge = 500
	testBytesHuge       = 600
	testBytesMassive    = 1000
)

// Test constants for user counts
const (
	testUserCountSmall  = 3
	testUserCountMedium = 4
	testUserCountLarge  = 10
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

// testUserLimits represents test user quota limits
type testUserLimits struct {
	storageLimit       *int64
	uploadDailyLimit   *int64
	downloadDailyLimit *int64
	uploadTotalLimit   *int64
	downloadTotalLimit *int64
}

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
		bytes := uint64(testBytesExtraLarge)
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

		err := usageManager.RecordUpload(userID, uploadID, bytes, ip)
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

		config, err := usageManager.GetUserQuotaConfig(userID)
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

		config, err := usageManager.GetUserQuotaConfig(userID)
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

		usage, err := usageManager.GetCurrentUsage(userID)
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
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testBytesSmall, "192.168.1.1")
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeDownload, testBytesMedium, "192.168.1.1")
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeStorageAdd, testBytesLarge, "192.168.1.1")

		usageManager := NewUsageManager(ctx)

		usage, err := usageManager.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.NotNil(t, usage)
		assert.Equal(t, userID, usage.UserID)
		assert.Equal(t, uint64(testBytesSmall), usage.BytesUploaded)
		assert.Equal(t, uint64(testBytesMedium), usage.BytesDownloaded)
		assert.Equal(t, uint64(testBytesLarge), usage.BytesStored)

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
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testBytesSmall, "192.168.1.1")

		// Create a record in the past
		oldDetail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      pluginModels.UsageTypeUpload,
			Bytes:     testBytesMedium,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour), // 2 days ago
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		usageManager := NewUsageManager(ctx)

		history, err := usageManager.GetUsageHistory(userID, 1, pluginModels.UsageTypeUpload)
		require.NoError(t, err)
		assert.Len(t, history, 1) // Only the recent record
		assert.Equal(t, uint64(testBytesSmall), history[0].Bytes)

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
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testBytesSmall, "192.168.1.1")

		// Create a record in the past
		oldDetail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      pluginModels.UsageTypeUpload,
			Bytes:     testBytesMedium,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour), // 2 days ago
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		usageManager := NewUsageManager(ctx)

		history, err := usageManager.GetUsageHistory(userID, 3, pluginModels.UsageTypeUpload)
		require.NoError(t, err)
		assert.Len(t, history, 2)                                  // Both records
		assert.Equal(t, uint64(testBytesMedium), history[0].Bytes) // Older record first
		assert.Equal(t, uint64(testBytesSmall), history[1].Bytes)  // Newer record second

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
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeUpload, testBytesSmall, "192.168.1.1")
		dataManager.CreateUsageDetail(userID, dataManager.NextUploadID(), pluginModels.UsageTypeDownload, testBytesMedium, "192.168.1.1")

		// Create a record outside the time range
		oldDetail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     testBytesLarge,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour),
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		usageManager := NewUsageManager(ctx)

		start := now.Add(-24 * time.Hour)
		end := now.Add(24 * time.Hour)

		details, err := usageManager.GetDetailedUsage(userID, start, end)
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
			Bytes:     testBytesSmall,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordUserUsageDetail(detail)
		require.NoError(t, err)

		// Verify the record was created
		var savedDetail pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).First(&savedDetail).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedDetail.UserID)
		assert.Equal(t, uint64(testBytesSmall), savedDetail.Bytes)
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

		err := usageManager.UpdateDailyUsage(userID, pluginModels.UsageTypeUpload, int64(testBytesSmall))
		require.NoError(t, err)

		// Verify the record was created
		var dailyQuota pluginModels.UserQuota
		today := time.Now().UTC().Truncate(24 * time.Hour)
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, userID, dailyQuota.UserID)
		assert.Equal(t, uint64(testBytesSmall), dailyQuota.BytesUploaded)
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
		err := usageManager.UpdateDailyUsage(userID, pluginModels.UsageTypeUpload, int64(testBytesSmall))
		require.NoError(t, err)

		// Then update it
		err = usageManager.UpdateDailyUsage(userID, pluginModels.UsageTypeUpload, int64(testBytesSmall/2))
		require.NoError(t, err)

		// Verify the record was updated
		var dailyQuota pluginModels.UserQuota
		today := time.Now().UTC().Truncate(24 * time.Hour)
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(testBytesSmall+testBytesSmall/2), dailyQuota.BytesUploaded) // testBytesSmall + testBytesSmall/2

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
		err := usageManager.UpdateDailyUsage(userID, pluginModels.UsageTypeUpload, int64(testBytesSmall))
		require.NoError(t, err)
		err = usageManager.UpdateDailyUsage(userID, pluginModels.UsageTypeDownload, int64(testBytesMedium))
		require.NoError(t, err)
		err = usageManager.UpdateDailyUsage(userID, pluginModels.UsageTypeStorageAdd, int64(testBytesLarge))
		require.NoError(t, err)

		// Verify all types were recorded correctly
		var dailyQuota pluginModels.UserQuota
		today := time.Now().UTC().Truncate(24 * time.Hour)
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(testBytesSmall), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(testBytesMedium), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(testBytesLarge), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

// TestUsageManager_RecordDownload_ValidInput_Success tests the RecordDownload method
func TestUsageManager_RecordDownload_ValidInput_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testBytesExtraLarge)
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

		err := usageManager.RecordDownload(userID, uploadID, bytes, ip)
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

// TestUsageManager_RecordDownload_WithSharedUsage_Success tests download recording with shared usage
func TestUsageManager_RecordDownload_WithSharedUsage_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := uint64(testBytesHuge)
		ip := "192.168.1.1"

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)

		pins := []*models.Pin{
			{UserID: userID},
			{UserID: dataManager.NextUserID()},
			{UserID: dataManager.NextUserID()},
		}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

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

		err := usageManager.RecordDownload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded with shared calculation
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, uint64(testBytesHuge/testUserCountSmall), usageDetails[0].Bytes) // testBytesHuge/testUserCountSmall = testBytesMedium
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(testUserCountSmall), usageDetails[0].SharedWith)

		// Verify the daily quota was updated
		today := time.Now().UTC().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(testBytesHuge/testUserCountSmall), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordStorageChange_ValidInput_Success tests the RecordStorageChange method
func TestUsageManager_RecordStorageChange_ValidInput_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := int64(testBytesExtraLarge)
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

		err := usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
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
		bytes := int64(-testBytesLarge)
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

		err := usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
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

// TestUsageManager_calculateSharedUsage_MultipleScenarios_Success tests the calculateSharedUsage method
func TestUsageManager_calculateSharedUsage_MultipleScenarios_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		totalBytes := uint64(testBytesHuge)

		var mockPinService = core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)
		pins := []*models.Pin{
			{UserID: userID},
			{UserID: dataManager.NextUserID()},
			{UserID: dataManager.NextUserID()},
		}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		require.NoError(t, err)
		assert.Equal(t, uint(testUserCountSmall), sharedWith)
		assert.Equal(t, uint64(testBytesMedium), sharedBytes) // testBytesHuge/testUserCountSmall = testBytesMedium

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		totalBytes := uint64(testBytesExtraLarge)

		// Simulate pin service unavailability by getting the service and setting it to nil
		// This test now checks that the usage manager properly handles nil pin service
		usageManager := NewUsageManager(ctx)
		usageManager.pinService = nil

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pin service not available")
		assert.Equal(t, uint(1), sharedWith)
		assert.Equal(t, totalBytes, sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		totalBytes := uint64(testBytesExtraLarge)

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return([]*models.Pin{}, fmt.Errorf("pin service error"))

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get pins for upload")
		assert.Equal(t, uint(1), sharedWith)
		assert.Equal(t, totalBytes, sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		uploadID := dataManager.NextUploadID()
		totalBytes := uint64(testBytesExtraLarge)

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)
		pins := []*models.Pin{}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		require.NoError(t, err)
		assert.Equal(t, uint(1), sharedWith) // Default to 1
		assert.Equal(t, totalBytes, sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		totalBytes := uint64(testBytesHuge)

		mockPinService := core.GetService[*coreMocks.MockPinService](ctx, core.PIN_SERVICE)
		user2ID := dataManager.NextUserID()
		user3ID := dataManager.NextUserID()
		pins := []*models.Pin{
			{UserID: userID},
			{UserID: userID}, // Duplicate
			{UserID: user2ID},
			{UserID: user2ID}, // Duplicate
			{UserID: user3ID},
		}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		require.NoError(t, err)
		assert.Equal(t, uint(testUserCountSmall), sharedWith)                  // Unique users only
		assert.Equal(t, uint64(testBytesHuge/testUserCountSmall), sharedBytes) // testBytesHuge/testUserCountSmall = testBytesMedium

		dataManager.Cleanup()
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_calculateSharedBytes_MultipleScenarios_Success tests the calculateSharedBytes method
func TestUsageManager_calculateSharedBytes_MultipleScenarios_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(testBytesMassive) // 1000 bytes
		userCount := uint(4)                   // Shared among 4 users

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 0)
		// Verifies exact division: 1000 bytes / 4 users = 250 bytes per user
		expected := uint64(250)
		assert.Equal(t, expected, sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(1)               // 1 byte total
		userCount := uint(testUserCountLarge) // Shared among 10 users

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 0)
		// Verifies floor division behavior: 1 byte / 10 users = 0 bytes per user (rounded down)
		assert.Equal(t, uint64(0), sharedBytes)

		dataManager.Cleanup()
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		totalBytes := uint64(testBytesMassive) // 1000 bytes
		userCount := uint(testUserCountSmall)  // Shared among 3 users

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 2)
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
		totalBytes := uint64(testBytesMassive) // 1000 bytes
		userCount := uint(testUserCountSmall)  // Shared among 3 users

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 10)
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
		totalBytes := uint64(testBytesMassive) // 1000 bytes
		userCount := uint(testUserCountSmall)  // Shared among 3 users

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 1)
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
		bytesPerGoroutine := uint64(testBytesSmall)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordUpload(userID, dataManager.NextUploadID(), bytesPerGoroutine, "192.168.1.1")
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
		usage, err := usageManager.GetCurrentUsage(userID)
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
		bytesPerGoroutine := uint64(testBytesMedium)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordDownload(userID, dataManager.NextUploadID(), bytesPerGoroutine, "192.168.1.2")
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
		usage, err := usageManager.GetCurrentUsage(userID)
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
		bytesPerGoroutine := int64(testBytesLarge)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordStorageChange(userID, dataManager.NextUploadID(), bytesPerGoroutine, "192.168.1.3")
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
		usage, err := usageManager.GetCurrentUsage(userID)
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
