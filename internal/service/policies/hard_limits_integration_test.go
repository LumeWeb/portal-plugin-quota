package policies

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestHardLimitsPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadTotalLimit:  lo.ToPtr(int64(5000)),
	}

	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
	// Mock current usage
	mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:        1,
		BytesUploaded: 200,
	}, nil)

	// Mock aggregated usage for total limit check
	mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
	mockUsageAggregator.On("GetAggregatedUsageByType", uint(1), models.UsageTypeUpload).Return(uint64(200), nil)
	mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)

	result, err := enforcer.CheckUploadQuota(config, uint64(500))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestHardLimitsPolicyEnforcer_CheckUploadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadTotalLimit:  lo.ToPtr(int64(5000)),
	}

	// Mock current usage that's close to daily limit
	mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:        2,
		BytesUploaded: 800,
	}, nil)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)

	result, err := enforcer.CheckUploadQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckUploadQuota_ExceedingTotalLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		UploadDailyLimit:  lo.ToPtr(int64(10000)),
		UploadTotalLimit:  lo.ToPtr(int64(1000)),
	}

	// Mock current usage that's close to total limit
	mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:        3,
		BytesUploaded: 900,
	}, nil)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
	mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
	mockUsageAggregator.On("GetAggregatedUsageByType", uint(3), models.UsageTypeUpload).Return(uint64(900), nil)
	mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)

	result, err := enforcer.CheckUploadQuota(config, uint64(200))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:             1,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadTotalLimit: lo.ToPtr(int64(10000)),
	}

	// Mock current usage
	mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:          1,
		BytesDownloaded: 500,
	}, nil)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)

	// Mock usage aggregator for total limit check
	mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
	mockUsageAggregator.On("GetAggregatedUsageByType", uint(1), models.UsageTypeDownload).Return(uint64(500), nil)
	mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)

	result, err := enforcer.CheckDownloadQuota(config, uint64(1000))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:             2,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadTotalLimit: lo.ToPtr(int64(10000)),
	}

	// Mock current usage that's close to daily limit
	mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:          2,
		BytesDownloaded: 1800,
	}, nil)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)

	result, err := enforcer.CheckDownloadQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_ExceedingTotalLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:             3,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadDailyLimit: lo.ToPtr(int64(10000)),
		DownloadTotalLimit: lo.ToPtr(int64(2000)),
	}

	// Mock current usage that's close to total limit
	mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:          3,
		BytesDownloaded: 1900,
	}, nil)

	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
	mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
	mockUsageAggregator.On("GetAggregatedUsageByType", uint(3), models.UsageTypeDownload).Return(uint64(1900), nil)
	mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)

	result, err := enforcer.CheckDownloadQuota(config, uint64(200))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckStorageQuota_WithinStorageLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		StorageLimit:      lo.ToPtr(int64(3000)),
	}

	// Mock current usage
	mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:      1,
		BytesStored: 1000,
	}, nil)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)

	result, err := enforcer.CheckStorageQuota(config, uint64(1500))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestHardLimitsPolicyEnforcer_CheckStorageQuota_ExceedingStorageLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		StorageLimit:      lo.ToPtr(int64(3000)),
	}

	// Mock current usage that's close to storage limit
	mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:      2,
		BytesStored: 2800,
	}, nil)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)

	result, err := enforcer.CheckStorageQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_RecordUpload_SuccessfulUploadRecording_Integration_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)

		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		// Set up mock expectations
		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		uploadDailyLimit := int64(1000)
		uploadTotalLimit := int64(5000)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &uploadDailyLimit,
			uploadTotalLimit: &uploadTotalLimit,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
		mockQuotaService.On("GetTodayUsage", uint(userID)).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 0,
		}, nil)
		mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
		mockUsageAggregator.On("GetAggregatedUsageByType", userID, models.UsageTypeUpload).Return(uint64(0), nil)
		mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(config, nil)
		mockUsageManager.On("RecordUpload", userID, uint(100), uint64(500), "127.0.0.1").Return(nil)
		mockUsageManager.On("GetCurrentUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 500,
		}, nil)

		// Test successful upload recording
		err = enforcer.RecordUpload(userID, 100, 500, "127.0.0.1")
		assert.NoError(t, err)

		// Verify the usage was recorded
		usage, err := enforcer.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(500), usage.BytesUploaded)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestHardLimitsPolicyEnforcer_RecordUpload_ExceedsQuota_Integration_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)

		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		// Set up mock expectations
		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		uploadDailyLimit := int64(1000)
		uploadTotalLimit := int64(5000)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &uploadDailyLimit,
			uploadTotalLimit: &uploadTotalLimit,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 900,
		}, nil)
		// Note: GetUsageAggregator and GetAggregatedUsageByType are not called because daily limit check fails first
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(config, nil)

		// Test upload that exceeds quota
		err = enforcer.RecordUpload(userID, 101, 1500, "127.0.0.1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "upload blocked")

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestHardLimitsPolicyEnforcer_RecordDownload_SuccessfulDownloadRecording_Integration_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)

		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		// Set up mock expectations
		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		downloadDailyLimit := int64(2000)
		downloadTotalLimit := int64(10000)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			downloadDailyLimit: &downloadDailyLimit,
			downloadTotalLimit: &downloadTotalLimit,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
		mockQuotaService.On("GetTodayUsage", uint(userID)).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 0,
		}, nil)
		mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
		mockUsageAggregator.On("GetAggregatedUsageByType", userID, models.UsageTypeDownload).Return(uint64(0), nil)
		mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)
		mockUsageManager.On("RecordDownload", userID, uint(200), uint64(1000), "127.0.0.1").Return(nil)
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(config, nil)
		mockUsageManager.On("GetCurrentUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 1000,
		}, nil)

		// Test successful download recording
		err = enforcer.RecordDownload(userID, 200, 1000, "127.0.0.1")
		assert.NoError(t, err)

		// Verify the usage was recorded
		usage, err := enforcer.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(1000), usage.BytesDownloaded)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestHardLimitsPolicyEnforcer_RecordDownload_ExceedsQuota_Integration_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)

		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		// Set up mock expectations
		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		downloadDailyLimit := int64(1000)
		downloadTotalLimit := int64(5000)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			downloadDailyLimit: &downloadDailyLimit,
			downloadTotalLimit: &downloadTotalLimit,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(config, nil)
		mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 900,
		}, nil)
		// Note: GetUsageAggregator and GetAggregatedUsageByType are not called because daily limit check fails first

		// Test download that exceeds quota
		err = enforcer.RecordDownload(userID, 201, 1500, "127.0.0.1")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "download blocked")

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestHardLimitsPolicyEnforcer_RecordStorageChange_SuccessfulStorageRecording_Integration_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)

		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		// Set up mock expectations
		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		storageLimit := int64(5000)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			storageLimit: &storageLimit,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)
		mockQuotaService.On("GetTodayUsage", uint(userID)).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 0,
		}, nil)
		mockUsageManager.On("RecordStorageChange", userID, uint(300), int64(1500), "127.0.0.1").Return(nil)
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(config, nil)
		mockUsageManager.On("GetCurrentUsage", userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 1500,
		}, nil)

		// Test successful storage change recording
		err = enforcer.RecordStorageChange(userID, 300, 1500, "127.0.0.1")
		assert.NoError(t, err)

		// Verify the usage was recorded
		usage, err := enforcer.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(1500), usage.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
