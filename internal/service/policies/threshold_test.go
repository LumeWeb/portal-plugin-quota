package policies

import (
	"math"
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

// thresholdTestSetup holds common test setup components
type thresholdTestSetup struct {
	ctx                  coreTesting.TestContext
	mockQuotaService     *pluginCore.MockQuotaService
	mockUsageManager     *pluginCore.MockUsageManager
	mockQuotaPlanManager *pluginCore.MockQuotaPlanManager
	enforcer             *ThresholdPolicyEnforcer
	dataManager          *testdata.TestDataManager
}

// setupThresholdTest creates a new test setup with mocked dependencies
func setupThresholdTest(t *testing.T) *thresholdTestSetup {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	// Setup base mock expectations
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Cleanup(func() {
		dataManager.Cleanup()
	})

	return &thresholdTestSetup{
		ctx:                  ctx,
		mockQuotaService:     mockQuotaService,
		mockUsageManager:     mockUsageManager,
		mockQuotaPlanManager: mockQuotaPlanManager,
		enforcer:             enforcer,
		dataManager:          dataManager,
	}
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed tests upload quota within daily limit
func TestThresholdPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadThreshold:   lo.ToPtr(int64(800)),
	}

	// Mock current usage
	setup.mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:        1,
		BytesUploaded: 200,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckUploadQuota(config, uint64(500))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), result.Details.Policy)
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_ExceedingDailyLimit_Integration_Blocked tests upload quota exceeding daily limit
func TestThresholdPolicyEnforcer_CheckUploadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadThreshold:   lo.ToPtr(int64(800)),
	}

	// Mock current usage that's close to daily limit
	setup.mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:        2,
		BytesUploaded: 800,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckUploadQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_AtThresholdWarningLevel_Integration_Warning tests upload quota at threshold warning level
func TestThresholdPolicyEnforcer_CheckUploadQuota_AtThresholdWarningLevel_Integration_Warning(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadDailyLimit:  lo.ToPtr(int64(1000)),
		UploadThreshold:   lo.ToPtr(int64(800)),
	}

	// Mock current usage that's at threshold
	setup.mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:        3,
		BytesUploaded: 700,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckUploadQuota(config, uint64(200))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	assert.NotNil(t, result.Details.Threshold)
	assert.Equal(t, uint64(800), *result.Details.Threshold)
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Integration_Allowed tests download quota within daily limit
func TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:             1,
		EnforcementPolicy:  models.EnforcementPolicyThreshold,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadThreshold:  lo.ToPtr(int64(1500)),
	}

	// Mock current usage
	setup.mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:          1,
		BytesDownloaded: 500,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckDownloadQuota(config, uint64(900))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), result.Details.Policy)
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Integration_Blocked tests download quota exceeding daily limit
func TestThresholdPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Integration_Blocked(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:             2,
		EnforcementPolicy:  models.EnforcementPolicyThreshold,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadThreshold:  lo.ToPtr(int64(1500)),
	}

	// Mock current usage that's close to daily limit
	setup.mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:          2,
		BytesDownloaded: 1800,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckDownloadQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

// TestThresholdPolicyEnforcer_CheckDownloadQuota_AtThresholdWarningLevel_Integration_Warning tests download quota at threshold warning level
func TestThresholdPolicyEnforcer_CheckDownloadQuota_AtThresholdWarningLevel_Integration_Warning(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:             3,
		EnforcementPolicy:  models.EnforcementPolicyThreshold,
		DownloadDailyLimit: lo.ToPtr(int64(2000)),
		DownloadThreshold:  lo.ToPtr(int64(1500)),
	}

	// Mock current usage that's at threshold
	setup.mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:          3,
		BytesDownloaded: 1400,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckDownloadQuota(config, uint64(200))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	assert.NotNil(t, result.Details.Threshold)
	assert.Equal(t, uint64(1500), *result.Details.Threshold)
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_WithinStorageLimit_Integration_Allowed tests storage quota within storage limit
func TestThresholdPolicyEnforcer_CheckStorageQuota_WithinStorageLimit_Integration_Allowed(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		StorageLimit:      lo.ToPtr(int64(3000)),
		StorageThreshold:  lo.ToPtr(int64(2000)),
	}

	// Mock current usage
	setup.mockQuotaService.On("GetTodayUsage", uint(1)).Return(&pluginCore.Usage{
		UserID:      1,
		BytesStored: 1000,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckStorageQuota(config, uint64(900))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), result.Details.Policy)
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_ExceedingStorageLimit_Integration_Blocked tests storage quota exceeding storage limit
func TestThresholdPolicyEnforcer_CheckStorageQuota_ExceedingStorageLimit_Integration_Blocked(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		StorageLimit:      lo.ToPtr(int64(3000)),
		StorageThreshold:  lo.ToPtr(int64(2000)),
	}

	// Mock current usage that's close to storage limit
	setup.mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
		UserID:      2,
		BytesStored: 2800,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckStorageQuota(config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
}

// TestThresholdPolicyEnforcer_CheckStorageQuota_AtThresholdWarningLevel_Integration_Warning tests storage quota at threshold warning level
func TestThresholdPolicyEnforcer_CheckStorageQuota_AtThresholdWarningLevel_Integration_Warning(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		StorageLimit:      lo.ToPtr(int64(3000)),
		StorageThreshold:  lo.ToPtr(int64(2000)),
	}

	// Mock current usage that's at threshold
	setup.mockQuotaService.On("GetTodayUsage", uint(3)).Return(&pluginCore.Usage{
		UserID:      3,
		BytesStored: 1900,
	}, nil).Maybe()

	result, err := setup.enforcer.CheckStorageQuota(config, uint64(200))
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
	mockUsageManager.On("GetTotalBytesByType", mock.Anything, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
	mockUsageManager.On("GetTotalBytesByType", mock.Anything, models.UsageTypeDownload).Return(uint64(0), nil).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager).Maybe()
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

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

	t.Run("Warning when reaching threshold", func(t *testing.T) {
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

// TestThresholdPolicyEnforcer_UploadSuccessDimensionAware tests dimension-aware success responses for uploads
func TestThresholdPolicyEnforcer_UploadSuccessDimensionAware(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager).Maybe()
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Only daily limit exists - reports daily usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(10000)),
			UploadThreshold:   nil,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 5000,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(*config.UploadDailyLimit), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Only total limit exists - reports total usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadTotalLimit:  lo.ToPtr(int64(50000)),
			UploadThreshold:   nil,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 5000,
		}, nil).Once()

		totalUsage := uint64(25000)
		// Only one call expected during quota check
		mockUsageManager.On("GetTotalBytesByType", userID, models.UsageTypeUpload).Return(totalUsage, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, totalUsage, result.Details.CurrentUsage)
		assert.Equal(t, uint64(*config.UploadTotalLimit), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Neither daily nor total limit exists", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 5000,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Nil(t, result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_DownloadSuccessDimensionAware tests dimension-aware success responses for downloads
func TestThresholdPolicyEnforcer_DownloadSuccessDimensionAware(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager).Maybe()
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Only daily limit exists - reports daily usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadDailyLimit: lo.ToPtr(int64(10000)),
			DownloadThreshold:  nil,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 5000,
		}, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(*config.DownloadDailyLimit), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Only total limit exists - reports total usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadTotalLimit: lo.ToPtr(int64(50000)),
			DownloadThreshold:  nil,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 5000,
		}, nil).Once()

		totalUsage := uint64(25000)
		// Only one call expected during quota check
		mockUsageManager.On("GetTotalBytesByType", userID, models.UsageTypeDownload).Return(totalUsage, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, totalUsage, result.Details.CurrentUsage)
		assert.Equal(t, uint64(*config.DownloadTotalLimit), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Neither daily nor total limit exists", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:          userID,
			BytesDownloaded: 5000,
		}, nil).Once()

		result, err := enforcer.CheckDownloadQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Nil(t, result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_StorageSuccessDimensionAware tests dimension-aware success responses for storage
func TestThresholdPolicyEnforcer_StorageSuccessDimensionAware(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()
	mockUsageManager.On("GetTotalBytesByType", mock.Anything, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
	mockUsageManager.On("GetTotalBytesByType", mock.Anything, models.UsageTypeDownload).Return(uint64(0), nil).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Storage limit exists - reports storage usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimit:      lo.ToPtr(int64(10000)),
			StorageThreshold:  nil,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 5000,
		}, nil).Once()

		result, err := enforcer.CheckStorageQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(*config.StorageLimit), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("No storage limit exists", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:      userID,
			BytesStored: 5000,
		}, nil).Once()

		result, err := enforcer.CheckStorageQuota(config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Nil(t, result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_OverflowPrevention tests uint64 overflow prevention
func TestThresholdPolicyEnforcer_OverflowPrevention(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager).Maybe()
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Current usage near max uint64", func(t *testing.T) {
		currentUsage := uint64(math.MaxUint64 - 5) // Max uint64 - 5
		requestedBytes := uint64(10)
		limit := uint64(math.MaxUint64) // Max uint64
		threshold := uint64(math.MaxUint64 - 15)
		policy := models.EnforcementPolicyThreshold

		result := enforcer.checkThresholdWithLimit(currentUsage, requestedBytes, &threshold, limit, policy)
		assert.NotNil(t, result)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("Requested bytes near max uint64", func(t *testing.T) {
		currentUsage := uint64(1000)
		requestedBytes := uint64(math.MaxUint64 - 1000) // Large but valid uint64
		limit := uint64(10000)
		threshold := uint64(5000)
		policy := models.EnforcementPolicyThreshold

		result := enforcer.checkThresholdWithLimit(currentUsage, requestedBytes, &threshold, limit, policy)
		assert.NotNil(t, result)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("Current usage equals limit exactly", func(t *testing.T) {
		currentUsage := uint64(10000)
		requestedBytes := uint64(1)
		limit := uint64(10000)
		threshold := uint64(5000)
		policy := models.EnforcementPolicyThreshold

		result := enforcer.checkThresholdWithLimit(currentUsage, requestedBytes, &threshold, limit, policy)
		assert.NotNil(t, result)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("Valid operation without overflow", func(t *testing.T) {
		currentUsage := uint64(5000)
		requestedBytes := uint64(1000)
		limit := uint64(10000)
		threshold := uint64(7500)
		policy := models.EnforcementPolicyThreshold

		result := enforcer.checkThresholdWithLimit(currentUsage, requestedBytes, &threshold, limit, policy)
		assert.Nil(t, result) // No action needed - below threshold
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_EdgeCases tests edge cases and boundary conditions
func TestThresholdPolicyEnforcer_EdgeCases(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager).Maybe()
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Current usage exactly equals limit", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(10000)),
		}

		// Mock current usage exactly at limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 10000,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(1))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("Requested bytes exactly equals remaining capacity", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(10000)),
		}

		// Mock current usage one byte under limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 9999,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	})

	t.Run("Small requested bytes", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(10000)),
		}

		// Mock current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 5000,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	})

	t.Run("Large limits and usage values", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(1000000000)), // Large but valid int64
		}

		// Mock large current usage
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 500000000,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(1000000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	})

	t.Run("Mixed daily and total limit scenarios - daily limit exceeded", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  lo.ToPtr(int64(5000)),
			UploadTotalLimit:  lo.ToPtr(int64(50000)),
		}

		// Mock current usage exactly at daily limit
		mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 5000,
		}, nil).Once()

		result, err := enforcer.CheckUploadQuota(config, uint64(1))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
		assert.Equal(t, uint64(*config.UploadDailyLimit), *result.Details.Limit)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
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
