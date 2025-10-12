package policies

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// TestThresholdPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed tests upload quota within daily limit
func TestThresholdPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadThreshold:   lo.ToPtr(int64(800)),
	}

	// Mock current usage
	mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:        1,
		BytesUploaded: 200,
	}, nil).Maybe()

	result, err := enforcer.CheckUploadQuota(config, uint64(500))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), result.Details.Policy)
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_ExceedingDailyLimit_Integration_Blocked tests upload quota exceeding daily limit
func TestThresholdPolicyEnforcer_CheckUploadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadThreshold:   lo.ToPtr(int64(800)),
	}

	// Mock current usage that's close to daily limit
	mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:        2,
		BytesUploaded: 800,
	}, nil).Maybe()

	result, err := enforcer.CheckUploadQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_AtThresholdWarningLevel_Integration_Warning tests upload quota at threshold warning level
func TestThresholdPolicyEnforcer_CheckUploadQuota_AtThresholdWarningLevel_Integration_Warning(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadThreshold:   lo.ToPtr(int64(800)),
	}

	// Mock current usage that's at threshold
	mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:        3,
		BytesUploaded: 700,
	}, nil).Maybe()

	result, err := enforcer.CheckUploadQuota(config, uint64(200))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	assert.NotNil(t, result.Details.Threshold)
	assert.Equal(t, uint64(800), *result.Details.Threshold)
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Integration_Allowed tests download quota within daily limit
func TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:             1,
		EnforcementPolicy:  models.EnforcementPolicyThreshold,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadThreshold:  lo.ToPtr(int64(1500)),
	}

	// Mock current usage
	mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:          1,
		BytesDownloaded: 500,
	}, nil).Maybe()

	result, err := enforcer.CheckDownloadQuota(config, uint64(900))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), result.Details.Policy)
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Integration_Blocked tests download quota exceeding daily limit
func TestThresholdPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:             2,
		EnforcementPolicy:  models.EnforcementPolicyThreshold,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadThreshold:  lo.ToPtr(int64(1500)),
	}

	// Mock current usage that's close to daily limit
	mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:          2,
		BytesDownloaded: 1800,
	}, nil).Maybe()

	result, err := enforcer.CheckDownloadQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_AtThresholdWarningLevel_Integration_Warning tests download quota at threshold warning level
func TestThresholdPolicyEnforcer_CheckDownloadQuota_AtThresholdWarningLevel_Integration_Warning(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:             3,
		EnforcementPolicy:  models.EnforcementPolicyThreshold,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadThreshold:  lo.ToPtr(int64(1500)),
	}

	// Mock current usage that's at threshold
	mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:          3,
		BytesDownloaded: 1400,
	}, nil).Maybe()

	result, err := enforcer.CheckDownloadQuota(config, uint64(200))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	assert.NotNil(t, result.Details.Threshold)
	assert.Equal(t, uint64(1500), *result.Details.Threshold)
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_WithinStorageLimit_Integration_Allowed tests storage quota within storage limit
func TestThresholdPolicyEnforcer_CheckStorageQuota_WithinStorageLimit_Integration_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		StorageLimit:      lo.ToPtr(int64(3000)),
		StorageThreshold:  lo.ToPtr(int64(2000)),
	}

	// Mock current usage
	mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:      1,
		BytesStored: 1000,
	}, nil).Maybe()

	result, err := enforcer.CheckStorageQuota(config, uint64(900))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), result.Details.Policy)
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_ExceedingStorageLimit_Integration_Blocked tests storage quota exceeding storage limit
func TestThresholdPolicyEnforcer_CheckStorageQuota_ExceedingStorageLimit_Integration_Blocked(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		StorageLimit:      lo.ToPtr(int64(3000)),
		StorageThreshold:  lo.ToPtr(int64(2000)),
	}

	// Mock current usage that's close to storage limit
	mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:      2,
		BytesStored: 2800,
	}, nil).Maybe()

	result, err := enforcer.CheckStorageQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_AtThresholdWarningLevel_Integration_Warning tests storage quota at threshold warning level
func TestThresholdPolicyEnforcer_CheckStorageQuota_AtThresholdWarningLevel_Integration_Warning(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		StorageLimit:      lo.ToPtr(int64(3000)),
		StorageThreshold:  lo.ToPtr(int64(2000)),
	}

	// Mock current usage that's at threshold
	mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:      3,
		BytesStored: 1900,
	}, nil).Maybe()

	result, err := enforcer.CheckStorageQuota(config, uint64(200))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	assert.NotNil(t, result.Details.Threshold)
	assert.Equal(t, uint64(2000), *result.Details.Threshold)
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_WithinLimit_Unit_Allowed tests the CheckUploadQuota method with mocks
func TestThresholdPolicyEnforcer_CheckUploadQuota_WithinLimit_Unit_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	// Setup base mocks that will be used by all tests
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()
	mockUsageManager.On("GetTotalBytesByType", mock.Anything, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
	mockUsageManager.On("GetTotalBytesByType", mock.Anything, models.UsageTypeDownload).Return(uint64(0), nil).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Within limit below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadTotalLimit:  lo.ToPtr(int64(5000)),
			UploadThreshold:   lo.ToPtr(int64(800)), // 80% threshold
		}

		// Mock current usage below threshold
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 300,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(200))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Within limit above threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadTotalLimit:  lo.ToPtr(int64(5000)),
			UploadThreshold:   lo.ToPtr(int64(800)), // 80% threshold
		}

		// Mock current usage above threshold but below limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 750,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		assert.NotNil(t, result.Details.Threshold)
		assert.Equal(t, uint64(800), *result.Details.Threshold)
	})

	t.Run("Exceeding limit", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadTotalLimit:  lo.ToPtr(int64(5000)),
			UploadThreshold:   lo.ToPtr(int64(800)),
		}

		// Mock current usage close to limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 950,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("No threshold configured", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadTotalLimit:  lo.ToPtr(int64(5000)),
			// No threshold configured
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 300,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(200))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinLimit_Unit_Allowed tests the CheckDownloadQuota method with mocks
func TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinLimit_Unit_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Within limit below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadDailyLimit: lo.ToPtr(int64(2000)),
			DownloadThreshold:  lo.ToPtr(int64(1600)), // 80% threshold
		}

		// Mock current usage below threshold
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 500,
		}, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(1000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason) // Should be OK since 500+1000=1500 < 1600 threshold
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Within limit above threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadDailyLimit: lo.ToPtr(int64(2000)),
			DownloadThreshold:  lo.ToPtr(int64(1600)), // 80% threshold
		}

		// Mock current usage above threshold but below limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 1500,
		}, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		assert.NotNil(t, result.Details.Threshold)
		assert.Equal(t, uint64(1600), *result.Details.Threshold)
	})

	t.Run("Exceeding limit", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadDailyLimit: lo.ToPtr(int64(2000)),
			DownloadThreshold:  lo.ToPtr(int64(1600)),
		}

		// Mock current usage close to limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 1900,
		}, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(200))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("No threshold configured", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadDailyLimit: lo.ToPtr(int64(2000)),
			// No threshold configured
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 500,
		}, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(1000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_WithinLimit_Unit_Allowed tests the CheckStorageQuota method with mocks
func TestThresholdPolicyEnforcer_CheckStorageQuota_WithinLimit_Unit_Allowed(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Within limit below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimit:      lo.ToPtr(int64(3000)),
			StorageThreshold:  lo.ToPtr(int64(2400)), // 80% threshold
		}

		// Mock current usage below threshold (1000 + 500 = 1500 < 2400)
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 1000,
		}, nil).Once()

		result, err := enforcer.CheckStorageQuota(config, uint64(500))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Within limit above threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimit:      lo.ToPtr(int64(3000)),
			StorageThreshold:  lo.ToPtr(int64(2400)), // 80% threshold
		}

		// Mock current usage just below threshold (2300 + 200 = 2500 > 2400)
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 2300,
		}, nil).Once()

		result, err := enforcer.CheckStorageQuota(config, uint64(200))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		assert.NotNil(t, result.Details.Threshold)
		assert.Equal(t, uint64(2400), *result.Details.Threshold)
	})

	t.Run("Exceeding limit", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimit:      lo.ToPtr(int64(3000)),
			StorageThreshold:  lo.ToPtr(int64(2400)),
		}

		// Mock current usage close to limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 2900,
		}, nil)

		result, err := enforcer.CheckStorageQuota(config, uint64(200))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_AtThreshold_Unit_Warning tests threshold warning behavior
func TestThresholdPolicyEnforcer_CheckUploadQuota_AtThreshold_Unit_Warning(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Warning at exactly threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadThreshold:   lo.ToPtr(int64(800)),
		}

		// Mock current usage exactly at threshold
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 800,
		}, nil)

		result, err := enforcer.CheckUploadQuota(config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	})

	t.Run("No warning below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadThreshold:   lo.ToPtr(int64(800)),
		}

		// Mock current usage just below threshold
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 799,
		}, nil)

		result, err := enforcer.CheckUploadQuota(config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	})

	t.Run("Warning with zero threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000)),
			UploadThreshold:   lo.ToPtr(int64(0)),
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 100,
		}, nil)

		result, err := enforcer.CheckUploadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		assert.NotNil(t, result.Details.Threshold)
		assert.Equal(t, uint64(0), *result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_ResolveEffectiveLimits_CustomLimits_Unit_Success tests limit resolution logic
func TestThresholdPolicyEnforcer_ResolveEffectiveLimits_CustomLimits_Unit_Success(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Custom limits with thresholds", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			StorageLimit:       lo.ToPtr(int64(1000)),
			UploadDailyLimit:   lo.ToPtr(int64(500)),
			DownloadDailyLimit: lo.ToPtr(int64(750)),
			StorageThreshold:   lo.ToPtr(int64(800)),
			UploadThreshold:    lo.ToPtr(int64(400)),
			DownloadThreshold:  lo.ToPtr(int64(600)),
		}

		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
		assert.NoError(t, err)
		assert.Equal(t, userID, limits.UserID)
		assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), limits.EnforcementPolicy)
		assert.Equal(t, uint64(1000), *limits.StorageLimit)
		assert.Equal(t, uint64(500), *limits.UploadDailyLimit)
		assert.Equal(t, uint64(750), *limits.DownloadDailyLimit)
		assert.Equal(t, uint64(800), *limits.StorageThreshold)
		assert.Equal(t, uint64(400), *limits.UploadThreshold)
		assert.Equal(t, uint64(600), *limits.DownloadThreshold)
	})

	t.Run("Nil thresholds", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimit:      lo.ToPtr(int64(1000)),
			UploadDailyLimit:  lo.ToPtr(int64(500)),
			// Thresholds are nil
		}

		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
		assert.NoError(t, err)
		assert.Equal(t, userID, limits.UserID)
		assert.Equal(t, uint64(1000), *limits.StorageLimit)
		assert.Equal(t, uint64(500), *limits.UploadDailyLimit)
		assert.Nil(t, limits.StorageThreshold)
		assert.Nil(t, limits.UploadThreshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
