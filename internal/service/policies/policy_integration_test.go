package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestPolicyIntegration_PolicySwitching_HardLimitsToUnlimited(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadDailyLimit := int64(1000)

		// Create mock services
		quotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		quotaService.On("GetUsageManager").Return(mockUsageManager)
		quotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

		// Setup hard limits config
		hardLimitsConfig := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
			UploadDailyLimit:  &uploadDailyLimit,
		}

		// Mock usage for hard limits check
		quotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 0,
		}, nil).Once()

		// Test hard limits enforcer
		hardLimitsEnforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)
		result, err := hardLimitsEnforcer.CheckUploadQuota(hardLimitsConfig, uint64(1500))
		require.NoError(t, err)
		assert.False(t, result.Allowed, "hard limits should block when exceeding limit")

		// Switch to unlimited policy
		unlimitedConfig := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyUnlimited,
		}

		// Test unlimited enforcer
		unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx, quotaService)
		result, err = unlimitedEnforcer.CheckUploadQuota(unlimitedConfig, uint64(1500))
		require.NoError(t, err)
		assert.True(t, result.Allowed, "unlimited policy should allow the operation")

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestPolicyIntegration_PolicySwitching_UnlimitedToThreshold(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadDailyLimit := int64(1000)
		uploadThreshold := int64(800)

		// Setup mocks
		quotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

		quotaService.On("GetUsageManager").Return(mockUsageManager)
		quotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

		// Test unlimited policy first
		unlimitedConfig := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyUnlimited,
		}

		unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx, quotaService)
		result, err := unlimitedEnforcer.CheckUploadQuota(unlimitedConfig, uint64(10000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)

		// Switch to threshold policy
		thresholdConfig := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  &uploadDailyLimit,
			UploadThreshold:   &uploadThreshold,
		}

		// Mock usage below threshold
		quotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
			UserID:        userID,
			BytesUploaded: 0,
		}, nil).Once()

		thresholdEnforcer := NewThresholdPolicyEnforcer(ctx, quotaService)
		result, err = thresholdEnforcer.CheckUploadQuota(thresholdConfig, uint64(500))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestPolicyIntegration_MixedPolicies(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		// Create users with different policies
		user1ID := dataManager.NextUserID()
		user2ID := dataManager.NextUserID()
		user3ID := dataManager.NextUserID()

		uploadDailyLimit := int64(1000)

		// Delete any existing records to avoid UNIQUE constraint
		ctx.DB().Where("user_id IN (?, ?, ?)", user1ID, user2ID, user3ID).Delete(&models.UserQuotaConfig{})

		createTestUser(t, ctx, user1ID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &uploadDailyLimit,
		})

		createTestUser(t, ctx, user2ID, models.EnforcementPolicyUnlimited, &testUserLimits{})

		uploadThreshold := int64(800)
		createTestUser(t, ctx, user3ID, models.EnforcementPolicyThreshold, &testUserLimits{
			uploadDailyLimit: &uploadDailyLimit,
			uploadThreshold:  &uploadThreshold,
		})

		// Hard limits user
		quotaService1 := pluginCore.NewMockQuotaService(t)
		mockUsageManager1 := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager1 := pluginCore.NewMockQuotaPlanManager(t)
		quotaService1.On("GetUsageManager").Return(mockUsageManager1)
		quotaService1.On("GetQuotaPlanManager").Return(mockQuotaPlanManager1).Maybe()

		// Setup mocks for all three policy types
		mockUsageManager1.On("GetUserQuotaConfig", user1ID).Return(&models.UserQuotaConfig{
			UserID:            user1ID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
			UploadDailyLimit:  &uploadDailyLimit,
		}, nil).Once()

		// Mock GetDefaultQuotaPlan to avoid unexpected calls
		mockQuotaPlanManager1.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

		hardLimitsEnforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService1)
		config1, err := quotaService1.GetUsageManager().GetUserQuotaConfig(user1ID)
		require.NoError(t, err)

		// Mock usage aggregator for hard limits enforcer
		mockUsageAggregator1 := pluginCore.NewMockUsageAggregator(t)
		quotaService1.On("GetUsageAggregator").Return(mockUsageAggregator1).Maybe()
		mockUsageAggregator1.On("GetAggregatedUsageByType", user1ID, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()

		// Mock GetTodayUsage for hard limits enforcer
		quotaService1.On("GetTodayUsage", user1ID).Return(&pluginCore.Usage{
			UserID:        user1ID,
			BytesUploaded: 0,
		}, nil).Maybe()

		// Hard limits should block when exceeding daily limit
		result1, err := hardLimitsEnforcer.CheckUploadQuota(config1, uint64(1500))
		require.NoError(t, err)
		assert.False(t, result1.Allowed)

		// Unlimited user
		quotaService2 := pluginCore.NewMockQuotaService(t)
		mockUsageManager2 := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager2 := pluginCore.NewMockQuotaPlanManager(t)
		quotaService2.On("GetUsageManager").Return(mockUsageManager2)
		quotaService2.On("GetQuotaPlanManager").Return(mockQuotaPlanManager2).Maybe()

		// Mock GetDefaultQuotaPlan again for the unlimited enforcer
		mockQuotaPlanManager2.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

		unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx, quotaService2)

		mockUsageManager2.On("GetUserQuotaConfig", user2ID).Return(&models.UserQuotaConfig{
			UserID:            user2ID,
			EnforcementPolicy: models.EnforcementPolicyUnlimited,
		}, nil).Once()

		config2, err := quotaService2.GetUsageManager().GetUserQuotaConfig(user2ID)
		require.NoError(t, err)

		result2, err := unlimitedEnforcer.CheckUploadQuota(config2, uint64(1500))
		require.NoError(t, err)
		assert.True(t, result2.Allowed) // Should be allowed

		// Threshold user
		quotaService3 := pluginCore.NewMockQuotaService(t)
		mockUsageManager3 := pluginCore.NewMockUsageManager(t)
		mockQuotaPlanManager3 := pluginCore.NewMockQuotaPlanManager(t)
		quotaService3.On("GetUsageManager").Return(mockUsageManager3)
		quotaService3.On("GetQuotaPlanManager").Return(mockQuotaPlanManager3).Maybe()

		// Mock GetDefaultQuotaPlan again for the threshold enforcer
		mockQuotaPlanManager3.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

		// Mock GetTodayUsage for threshold enforcer
		quotaService3.On("GetTodayUsage", user3ID).Return(&pluginCore.Usage{
			UserID:        user3ID,
			BytesUploaded: 0,
		}, nil).Maybe()

		mockUsageManager3.On("GetUserQuotaConfig", user3ID).Return(&models.UserQuotaConfig{
			UserID:            user3ID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			UploadDailyLimit:  &uploadDailyLimit,
			UploadThreshold:   &uploadThreshold,
		}, nil).Once()

		config3, err := quotaService3.GetUsageManager().GetUserQuotaConfig(user3ID)
		require.NoError(t, err)

		thresholdEnforcer := NewThresholdPolicyEnforcer(ctx, quotaService3)
		// Threshold policy should block when exceeding daily limit
		result3, err := thresholdEnforcer.CheckUploadQuota(config3, uint64(1500))
		require.NoError(t, err)
		assert.False(t, result3.Allowed)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestPolicyIntegration_ValidationConsistency(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadDailyLimit := int64(1000)

		// Delete any existing record to avoid UNIQUE constraint
		ctx.DB().Where("user_id = ?", userID).Delete(&models.UserQuotaConfig{})

		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &uploadDailyLimit,
		})

		// Create simple mock services with .Maybe() expectations
		createMockService := func(t *testing.T) pluginCore.QuotaService {
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

			quotaService.On("GetUsageManager").Return(mockUsageManager).Maybe()
			quotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager).Maybe()
			mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Maybe()

			return quotaService
		}

		// Create enforcers with simple mocks
		hardLimitsEnforcer := NewHardLimitsPolicyEnforcer(ctx, createMockService(t))
		unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx, createMockService(t))
		thresholdEnforcer := NewThresholdPolicyEnforcer(ctx, createMockService(t))

		// For allowance enforcer, we need to mock grant manager too
		quotaService := pluginCore.NewMockQuotaService(t)
		quotaService.On("GetUsageManager").Return(pluginCore.NewMockUsageManager(t)).Maybe()
		quotaService.On("GetQuotaPlanManager").Return(pluginCore.NewMockQuotaPlanManager(t)).Maybe()
		quotaService.On("GetGrantManager").Return(pluginCore.NewMockGrantManager(t)).Maybe()
		allowanceEnforcer := NewAllowancePolicyEnforcer(ctx, quotaService)

		// Test with invalid user ID (0)
		invalidConfig := &models.UserQuotaConfig{
			UserID:            0,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		}

		_, err1 := hardLimitsEnforcer.CheckUploadQuota(invalidConfig, uint64(100))
		invalidConfig.EnforcementPolicy = models.EnforcementPolicyUnlimited
		_, err2 := unlimitedEnforcer.CheckUploadQuota(invalidConfig, uint64(100))
		invalidConfig.EnforcementPolicy = models.EnforcementPolicyThreshold
		_, err3 := thresholdEnforcer.CheckUploadQuota(invalidConfig, uint64(100))
		invalidConfig.EnforcementPolicy = models.EnforcementPolicyAllowance
		_, err4 := allowanceEnforcer.CheckUploadQuota(invalidConfig, uint64(100))

		// All should return the same error for invalid user ID
		for _, err := range []error{err1, err2, err3, err4} {
			assert.Error(t, err)
			assert.ErrorIs(t, err, models.ErrInvalidUserID)
		}

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
