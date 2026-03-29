package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		UploadLimitBytes:  uint64(1000),
		WindowType:        models.WindowTypeCalendarDay,
		WindowDuration:    &windowDuration,
		WindowStartHour:   &windowStartHour,
		WindowTimezone:    &timezone,
	}

	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

	// Mock usage for the window
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(1), models.UsageTypeUpload, window).Return(uint64(200), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckUploadQuota(ctx, config, uint64(500))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestHardLimitsPolicyEnforcer_CheckUploadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		UploadLimitBytes:  uint64(1000),
		WindowType:        models.WindowTypeCalendarDay,
		WindowDuration:    &windowDuration,
		WindowStartHour:   &windowStartHour,
		WindowTimezone:    &timezone,
	}

	// Mock current usage that's close to daily limit
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(2), models.UsageTypeUpload, window).Return(uint64(800), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckUploadQuota(ctx, config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckUploadQuota_ExceedingTotalLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		UploadLimitBytes:  uint64(10000),
		WindowType:        models.WindowTypeCalendarDay,
		WindowDuration:    &windowDuration,
		WindowStartHour:   &windowStartHour,
		WindowTimezone:    &timezone,
	}

	// Mock current usage that's close to the limit
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(3), models.UsageTypeUpload, window).Return(uint64(9900), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckUploadQuota(ctx, config, uint64(200))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:             1,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadLimitBytes: uint64(2000),
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	// Mock current usage
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(1), models.UsageTypeDownload, window).Return(uint64(500), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(1000))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:             2,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadLimitBytes: uint64(2000),
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	// Mock current usage that's close to daily limit
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(2), models.UsageTypeDownload, window).Return(uint64(1800), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_ExceedingTotalLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:             3,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadLimitBytes: uint64(10000),
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	// Mock current usage that's close to total limit
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(3), models.UsageTypeDownload, window).Return(uint64(9900), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(200))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

func TestHardLimitsPolicyEnforcer_CheckStorageQuota_WithinStorageLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"
	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		StorageLimitBytes: uint64(3000),
		WindowType:        models.WindowTypeCalendarDay,
		WindowDuration:    &windowDuration,
		WindowStartHour:   &windowStartHour,
		WindowTimezone:    &timezone,
	}

	// Mock current usage for the window
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(1), models.UsageTypeStorageAdd, window).Return(uint64(500), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckStorageQuota(ctx, config, uint64(1500))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestHardLimitsPolicyEnforcer_CheckStorageQuota_ExceedingStorageLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	ctx, _ := coreTesting.NewTestContext(t)
	enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		StorageLimitBytes: uint64(3000),
		WindowType:        models.WindowTypeCalendarDay,
		WindowDuration:    &windowDuration,
		WindowStartHour:   &windowStartHour,
		WindowTimezone:    &timezone,
	}

	// Mock current usage close to the limit
	window := pluginCore.LimitWindow{
		Type:      pluginCore.WindowType(config.WindowType.String()),
		Duration:  config.WindowDuration,
		StartHour: config.WindowStartHour,
		Timezone:  config.WindowTimezone,
	}
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)
	mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(2), models.UsageTypeStorageAdd, window).Return(uint64(2800), time.Now(), time.Now(), nil)

	result, err := enforcer.CheckStorageQuota(ctx, config, uint64(300))
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
		mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		uploadLimit := int64(1000)
		windowDuration := int64(86400)
		windowType := string(models.WindowTypeCalendarDay)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadLimit: &uploadLimit,
			windowDuration: &windowDuration,
			windowType: &windowType,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
		mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

		// Mock usage for the window
		window := pluginCore.LimitWindow{
			Type:      pluginCore.WindowType(config.WindowType.String()),
			Duration:  config.WindowDuration,
			StartHour: config.WindowStartHour,
			Timezone:  config.WindowTimezone,
		}
		mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeUpload, window).Return(uint64(0), time.Now(), time.Now(), nil)

		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(config, nil)
		mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uint(100), uint64(500), "127.0.0.1").Return(nil)
		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 500,
		}, nil)

		// Test successful upload recording
		err = enforcer.RecordUpload(ctx, userID, 100, 500, "127.0.0.1")
		assert.NoError(t, err)

		// Verify the usage was recorded
		usage, err := enforcer.GetCurrentUsage(ctx, userID)
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
		mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		uploadLimit := int64(1000)
		windowDuration := int64(86400)
		windowType := string(models.WindowTypeCalendarDay)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadLimit: &uploadLimit,
			windowDuration: &windowDuration,
			windowType: &windowType,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
		mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

		// Mock usage for the window
		window := pluginCore.LimitWindow{
			Type:      pluginCore.WindowType(config.WindowType.String()),
			Duration:  config.WindowDuration,
			StartHour: config.WindowStartHour,
			Timezone:  config.WindowTimezone,
		}
		mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeUpload, window).Return(uint64(900), time.Now(), time.Now(), nil)

		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(config, nil)

		// Test upload that exceeds quota
		err = enforcer.RecordUpload(ctx, userID, 101, 1500, "127.0.0.1")
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
		mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		downloadLimit := int64(2000)
		windowDuration := int64(86400)
		windowType := string(models.WindowTypeCalendarDay)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			downloadLimit:  &downloadLimit,
			windowDuration: &windowDuration,
			windowType:     &windowType,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
		mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

		// Mock usage for the window
		window := pluginCore.LimitWindow{
			Type:      pluginCore.WindowType(config.WindowType.String()),
			Duration:  config.WindowDuration,
			StartHour: config.WindowStartHour,
			Timezone:  config.WindowTimezone,
		}
		mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeDownload, window).Return(uint64(0), time.Now(), time.Now(), nil)

		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(config, nil)
		mockUsageManager.EXPECT().RecordDownload(mock.Anything, userID, uint(200), uint64(1000), "127.0.0.1").Return(nil)
		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 1000,
		}, nil)

		// Test successful download recording
		err = enforcer.RecordDownload(ctx, userID, 200, 1000, "127.0.0.1")
		assert.NoError(t, err)

		// Verify the usage was recorded
		usage, err := enforcer.GetCurrentUsage(ctx, userID)
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
		mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		downloadLimit := int64(1000)
		windowDuration := int64(86400)
		windowType := string(models.WindowTypeCalendarDay)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			downloadLimit:  &downloadLimit,
			windowDuration: &windowDuration,
			windowType:     &windowType,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
		mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

		// Mock usage for the window
		window := pluginCore.LimitWindow{
			Type:      pluginCore.WindowType(config.WindowType.String()),
			Duration:  config.WindowDuration,
			StartHour: config.WindowStartHour,
			Timezone:  config.WindowTimezone,
		}
		mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeDownload, window).Return(uint64(900), time.Now(), time.Now(), nil)

		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(config, nil)

		// Test download that exceeds quota
		err = enforcer.RecordDownload(ctx, userID, 201, 1500, "127.0.0.1")
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
		mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		// Create a test user
		userID := dataManager.NextUserID()
		storageLimit := int64(5000)
		windowDuration := int64(86400)
		windowType := string(models.WindowTypeCalendarDay)

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			storageLimit:  &storageLimit,
			windowDuration: &windowDuration,
			windowType:     &windowType,
		})
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		// Set up remaining mock expectations
		mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
		mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

		// Mock usage for the window
		window := pluginCore.LimitWindow{
			Type:      pluginCore.WindowType(config.WindowType.String()),
			Duration:  config.WindowDuration,
			StartHour: config.WindowStartHour,
			Timezone:  config.WindowTimezone,
		}
		mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeStorageAdd, window).Return(uint64(0), time.Now(), time.Now(), nil)

		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(config, nil)
		mockUsageManager.EXPECT().RecordStorageChange(mock.Anything, userID, uint(300), int64(1500), "127.0.0.1").Return(nil)
		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(config, nil)
		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 1500,
		}, nil)

		// Test successful storage change recording
		err = enforcer.RecordStorageChange(ctx, userID, 300, 1500, "127.0.0.1")
		assert.NoError(t, err)

		// Verify the usage was recorded
		usage, err := enforcer.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(1500), usage.BytesStored)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
