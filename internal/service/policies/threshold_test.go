package policies

import (
	"math"
	"testing"
	"time"

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
	mockReservationManager *pluginCore.MockReservationManager
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
	mockReservationManager := pluginCore.NewMockReservationManager(t)

	// Setup base mock expectations
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Cleanup(func() {
		dataManager.Cleanup()
	})

	return &thresholdTestSetup{
		ctx:                  ctx,
		mockQuotaService:     mockQuotaService,
		mockUsageManager:     mockUsageManager,
		mockQuotaPlanManager: mockQuotaPlanManager,
		mockReservationManager: mockReservationManager,
		enforcer:             enforcer,
		dataManager:          dataManager,
	}
}

// thresholdSubTestMocks holds mocks for subtests (no shared state)
type thresholdSubTestMocks struct {
	mockQuotaService     *pluginCore.MockQuotaService
	mockUsageManager     *pluginCore.MockUsageManager
	mockQuotaPlanManager *pluginCore.MockQuotaPlanManager
	mockReservationManager *pluginCore.MockReservationManager
}

// setupThresholdSubTest creates fresh mocks for a subtest (ensures no shared state)
func setupThresholdSubTest(t *testing.T) *thresholdSubTestMocks {
	return &thresholdSubTestMocks{
		mockQuotaService:     pluginCore.NewMockQuotaService(t),
		mockUsageManager:     pluginCore.NewMockUsageManager(t),
		mockQuotaPlanManager: pluginCore.NewMockQuotaPlanManager(t),
		mockReservationManager: pluginCore.NewMockReservationManager(t),
	}
}

// TestThresholdPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed tests upload quota within daily limit
func TestThresholdPolicyEnforcer_CheckUploadQuota_WithinDailyLimit_Integration_Allowed(t *testing.T) {
	setup := setupThresholdTest(t)

	config := &models.UserQuotaConfig{
		UserID:            1,
		EnforcementPolicy: models.EnforcementPolicyThreshold,
		UploadLimitBytes: uint64(1000),
		UploadThreshold:   lo.ToPtr(int64(800)),
		WindowType:        models.WindowTypeCalendarDay,
	}

	// Mock current usage
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(200), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckUploadQuota(setup.ctx, config, uint64(500))
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
		UploadLimitBytes: uint64(1000),
		UploadThreshold:   		lo.ToPtr(int64(800)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage that's close to daily limit
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(800), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckUploadQuota(setup.ctx, config, uint64(300))
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
		UploadLimitBytes: uint64(1000),
		UploadThreshold:   		lo.ToPtr(int64(800)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage that's at threshold
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(700), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckUploadQuota(setup.ctx, config, uint64(200))
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
		DownloadLimitBytes: uint64(2000),
		DownloadThreshold:   		lo.ToPtr(int64(1500)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(500), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckDownloadQuota(setup.ctx, config, uint64(900))
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
		DownloadLimitBytes: uint64(2000),
		DownloadThreshold:   		lo.ToPtr(int64(1500)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage that's close to daily limit
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1800), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckDownloadQuota(setup.ctx, config, uint64(300))
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
		DownloadLimitBytes: uint64(2000),
		DownloadThreshold:   		lo.ToPtr(int64(1500)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage that's at threshold
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1400), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckDownloadQuota(setup.ctx, config, uint64(200))
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
		StorageLimitBytes:		uint64(3000),
		StorageThreshold:  		lo.ToPtr(int64(2000)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1000), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckStorageQuota(setup.ctx, config, uint64(900))
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
		StorageLimitBytes:		uint64(3000),
		StorageThreshold:  		lo.ToPtr(int64(2000)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage that's close to storage limit
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(2800), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckStorageQuota(setup.ctx, config, uint64(300))
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
		StorageLimitBytes:		uint64(3000),
		StorageThreshold:  		lo.ToPtr(int64(2000)),
		WindowType: models.WindowTypeCalendarDay,
	}

	// Mock current usage that's at threshold
	// Mock GetUsageForWindow
	setup.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1900), time.Now(), time.Now(), nil)


	result, err := setup.enforcer.CheckStorageQuota(setup.ctx, config, uint64(200))
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

	t.Run("Within limit below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(1000),
			WindowType:		models.WindowTypeLifetime,
			UploadThreshold:   		lo.ToPtr(int64(800)), // 80% threshold
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(300), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(200))
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
			UploadLimitBytes: uint64(1000),
			WindowType:		models.WindowTypeLifetime,
			UploadThreshold:   		lo.ToPtr(int64(800)), // 80% threshold
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(750), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(100))
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
			UploadLimitBytes: uint64(1000),
			WindowType:		models.WindowTypeLifetime,
			UploadThreshold:   		lo.ToPtr(int64(800)),
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(950), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("No threshold configured", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(1000),
			WindowType:		models.WindowTypeLifetime,
			// No threshold configured
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(300), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(200))
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
// TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinLimit_Unit_Allowed tests the CheckDownloadQuota method with mocks
func TestThresholdPolicyEnforcer_CheckDownloadQuota_WithinLimit_Unit_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Within limit below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadLimitBytes: uint64(2000),
			DownloadThreshold:   		lo.ToPtr(int64(1600)), // 80% threshold
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(500), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(1000))
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
			DownloadLimitBytes: uint64(2000),
			DownloadThreshold:   		lo.ToPtr(int64(1600)), // 80% threshold
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1500), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(100))
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
			DownloadLimitBytes: uint64(2000),
			DownloadThreshold:   		lo.ToPtr(int64(1600)),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1900), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(200))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("No threshold configured", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadLimitBytes: uint64(2000),
			WindowType: models.WindowTypeCalendarDay,
			// No threshold configured
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(500), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(1000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
func TestThresholdPolicyEnforcer_CheckStorageQuota_WithinLimit_Unit_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Within limit below threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimitBytes:		uint64(3000),
			StorageThreshold:  		lo.ToPtr(int64(2400)), // 80% threshold
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckStorageQuota(ctx, config, uint64(500))
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
			StorageLimitBytes:		uint64(3000),
			StorageThreshold:  		lo.ToPtr(int64(2400)), // 80% threshold
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(2300), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckStorageQuota(ctx, config, uint64(200))
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
			StorageLimitBytes:		uint64(3000),
			StorageThreshold:  		lo.ToPtr(int64(2400)),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(2900), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckStorageQuota(ctx, config, uint64(200))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("No threshold configured", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimitBytes:		uint64(3000),
			WindowType: models.WindowTypeCalendarDay,
			// No threshold configured
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(1000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckStorageQuota(ctx, config, uint64(500))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
func TestThresholdPolicyEnforcer_CheckUploadQuota_AtThreshold_Unit_Warning(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Warning at exactly threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(1000),
			UploadThreshold:   		lo.ToPtr(int64(800)),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(800), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	})

	t.Run("Warning when reaching threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(1000),
			UploadThreshold:   		lo.ToPtr(int64(800)),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(799), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	})

	t.Run("Warning with zero threshold", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(1000),
			UploadThreshold:   		lo.ToPtr(int64(0)),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(100), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(100))
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
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Only daily limit exists - reports daily usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(10000),
			UploadThreshold:   nil,
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(5000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(config.UploadLimitBytes), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Lifetime window limit exists - reports lifetime usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(50000),
			UploadThreshold:   nil,
			WindowType: models.WindowTypeLifetime,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(25000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(25000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(config.UploadLimitBytes), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("No limit configured - returns zero usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks - no GetUsageManager call expected when no limit is configured
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(0), result.Details.CurrentUsage)
		assert.Nil(t, result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_DownloadSuccessDimensionAware tests dimension-aware success responses for downloads
func TestThresholdPolicyEnforcer_DownloadSuccessDimensionAware(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Only daily limit exists - reports daily usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadLimitBytes: uint64(10000),
			DownloadThreshold:  nil,
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(5000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(config.DownloadLimitBytes), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("Lifetime window limit exists - reports lifetime usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			DownloadLimitBytes: uint64(50000),
			DownloadThreshold:  nil,
			WindowType: models.WindowTypeLifetime,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(25000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(25000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(config.DownloadLimitBytes), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("No limit configured - returns zero usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks - no GetUsageManager call expected when no limit is configured
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(0), result.Details.CurrentUsage)
		assert.Nil(t, result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestThresholdPolicyEnforcer_StorageSuccessDimensionAware tests dimension-aware success responses for storage
func TestThresholdPolicyEnforcer_StorageSuccessDimensionAware(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Storage limit exists - reports storage usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimitBytes:		uint64(10000),
			StorageThreshold:  nil,
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(5000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckStorageQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(5000), result.Details.CurrentUsage)
		assert.Equal(t, uint64(config.StorageLimitBytes), *result.Details.Limit)
		assert.Nil(t, result.Details.Threshold)
	})

	t.Run("No storage limit configured - returns zero usage", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks - no limit configured in config
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckStorageQuota(ctx, config, uint64(100))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, uint64(0), result.Details.CurrentUsage)
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
	mockReservationManager := pluginCore.NewMockReservationManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager).Maybe()
	mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()

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
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	t.Run("Current usage exactly equals limit", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(10000),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(10000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	})

	t.Run("Requested bytes exactly equals remaining capacity", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(10000),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(9999), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	})

	t.Run("Small requested bytes", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(10000),
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(5000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	})

	t.Run("Large limits and usage values", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(1000000000), // Large but valid int64
			WindowType: models.WindowTypeCalendarDay,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(500000000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1000000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	})

	t.Run("Mixed daily and total limit scenarios - daily limit exceeded", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadLimitBytes: uint64(5000),
			WindowType:		models.WindowTypeLifetime,
		}

		// Create fresh mocks for this subtest
		mocks := setupThresholdSubTest(t)

		// Setup mocks
		mocks.mockQuotaService.EXPECT().GetReservationManager().Return(mocks.mockReservationManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetUsageManager().Return(mocks.mockUsageManager)
		mocks.mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mocks.mockQuotaPlanManager)
		mocks.mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
		mocks.mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(uint64(5000), time.Now(), time.Now(), nil)

		enforcer := NewThresholdPolicyEnforcer(ctx, mocks.mockQuotaService)

		result, err := enforcer.CheckUploadQuota(ctx, config, uint64(1))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
		assert.Equal(t, uint64(config.UploadLimitBytes), *result.Details.Limit)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
func TestThresholdPolicyEnforcer_ResolveEffectiveLimits_CustomLimits_Unit_Success(t *testing.T) {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockReservationManager := pluginCore.NewMockReservationManager(t)

	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()

	enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

	t.Run("Custom limits with thresholds", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyThreshold,
			StorageLimitBytes:		uint64(1000),
			UploadLimitBytes: uint64(500),
			DownloadLimitBytes: uint64(750),
			StorageThreshold:  		lo.ToPtr(int64(800)),
			UploadThreshold:   		lo.ToPtr(int64(400)),
			DownloadThreshold:   		lo.ToPtr(int64(600)),
			WindowType: models.WindowTypeCalendarDay,
		}

		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyThreshold)
		assert.NoError(t, err)
		assert.Equal(t, userID, limits.UserID)
		assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyThreshold), limits.EnforcementPolicy)
		assert.Equal(t, uint64(1000), limits.StorageLimitConfig.Bytes)
		assert.Equal(t, uint64(500), limits.UploadLimitConfig.Bytes)
		assert.Equal(t, uint64(750), limits.DownloadLimitConfig.Bytes)
		assert.Equal(t, uint64(800), *limits.StorageThreshold)
		assert.Equal(t, uint64(400), *limits.UploadThreshold)
		assert.Equal(t, uint64(600), *limits.DownloadThreshold)
	})

	t.Run("Nil thresholds", func(t *testing.T) {
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			StorageLimitBytes:		uint64(1000),
			UploadLimitBytes: uint64(500),
			// Thresholds are nil
			WindowType: models.WindowTypeCalendarDay,
		}

		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyThreshold)
		assert.NoError(t, err)
		assert.Equal(t, userID, limits.UserID)
		assert.Equal(t, uint64(1000), limits.StorageLimitConfig.Bytes)
		assert.Equal(t, uint64(500), limits.UploadLimitConfig.Bytes)
		assert.Nil(t, limits.StorageThreshold)
		assert.Nil(t, limits.UploadThreshold)
	})

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
