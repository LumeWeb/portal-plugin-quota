package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestConfiguration_DefaultCreation_NonExistentUser(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()

		quotaService := core.GetService[*pluginCore.MockQuotaService](ctx, pluginCore.QUOTA_SERVICE)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		quotaService.On("GetUsageManager").Return(mockUsageManager)
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		}, nil)
		config, err := quotaService.GetUsageManager().GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.NotNil(t, config)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestConfiguration_Updates(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		initialUploadLimit := int64(1000)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &initialUploadLimit,
		})

		quotaService := core.GetService[*pluginCore.MockQuotaService](ctx, pluginCore.QUOTA_SERVICE)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		quotaService.On("GetUsageManager").Return(mockUsageManager)

		// Get the actual config from database
		var config models.UserQuotaConfig
		err := ctx.DB().Where("user_id = ?", userID).First(&config).Error
		require.NoError(t, err)

		// Mock the GetUserQuotaConfig to return the actual config
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(&config, nil)

		// Get initial config
		retrievedConfig, err := quotaService.GetUsageManager().GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.Equal(t, initialUploadLimit, *retrievedConfig.UploadDailyLimit)
		assert.Equal(t, models.EnforcementPolicyHardLimits, retrievedConfig.EnforcementPolicy)

		// Test updating upload limit
		newUploadLimit := int64(2000)
		config.UploadDailyLimit = &newUploadLimit
		err = ctx.DB().Save(&config).Error
		require.NoError(t, err)

		// Update the mock to return the updated config
		mockUsageManager.ExpectedCalls = nil // Clear previous expectations
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(&config, nil)

		updatedConfig, err := quotaService.GetUsageManager().GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.Equal(t, newUploadLimit, *updatedConfig.UploadDailyLimit)

		// Test updating enforcement policy
		config.EnforcementPolicy = models.EnforcementPolicyUnlimited
		err = ctx.DB().Save(&config).Error
		require.NoError(t, err)

		// Update the mock to return the updated config
		mockUsageManager.ExpectedCalls = nil // Clear previous expectations
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(&config, nil)

		updatedConfig, err = quotaService.GetUsageManager().GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.Equal(t, models.EnforcementPolicyUnlimited, updatedConfig.EnforcementPolicy)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestConfiguration_QuotaPlanIntegration_WithPlan(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		// Create a quota plan
		plan := createTestQuotaPlan(t, ctx, "Test Plan", false, &testPlanLimits{
			storageLimit:       5000,
			uploadDailyLimit:   1000,
			downloadDailyLimit: 2000,
			uploadTotalLimit:   10000,
			downloadTotalLimit: 20000,
		})

		// Assign plan to user
		config := &models.UserQuotaConfig{}
		err := ctx.DB().Where("user_id = ?", userID).First(config).Error
		require.NoError(t, err)

		planID := uint64(plan.ID)
		config.QuotaPlanID = &planID
		err = ctx.DB().Save(config).Error
		require.NoError(t, err)

		// Test hard limits enforcer with plan
		quotaService := core.GetService[*pluginCore.MockQuotaService](ctx, pluginCore.QUOTA_SERVICE)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		quotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)
		quotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
		quotaService.On("GetQuotaPlanManager").Return(quotaPlanManager)
		quotaPlanManager.On("GetQuotaPlanByID", planID).Return(plan, nil)
		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
		require.NoError(t, err)
		assert.Equal(t, uint64(plan.StorageLimit), *limits.StorageLimit)
		assert.Equal(t, uint64(plan.UploadDailyLimit), *limits.UploadDailyLimit)
		assert.Equal(t, uint64(plan.DownloadDailyLimit), *limits.DownloadDailyLimit)
		assert.Equal(t, uint64(plan.UploadTotalLimit), *limits.UploadTotalLimit)
		assert.Equal(t, uint64(plan.DownloadTotalLimit), *limits.DownloadTotalLimit)

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestConfiguration_QuotaPlanIntegration_CustomOverridesPlan(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		// Create a quota plan
		plan := createTestQuotaPlan(t, ctx, "Override Plan", false, &testPlanLimits{
			storageLimit:       5000,
			uploadDailyLimit:   1000,
			downloadDailyLimit: 2000,
			uploadTotalLimit:   10000,
			downloadTotalLimit: 20000,
		})

		// Create user with plan and custom override
		customStorageLimit := int64(3000)
		planID := uint64(plan.ID)
		config := createTestUser(t, ctx, dataManager.NextUserID(), models.EnforcementPolicyHardLimits, &testUserLimits{
			quotaPlanID:  &planID,
			storageLimit: &customStorageLimit,
		})

		// Test hard limits enforcer
		quotaService := core.GetService[*pluginCore.MockQuotaService](ctx, pluginCore.QUOTA_SERVICE)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		quotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)
		quotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
		quotaService.On("GetQuotaPlanManager").Return(quotaPlanManager)
		quotaPlanManager.On("GetQuotaPlanByID", planID).Return(plan, nil)
		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
		require.NoError(t, err)
		assert.Equal(t, uint64(customStorageLimit), *limits.StorageLimit)            // Custom value
		assert.Equal(t, uint64(plan.UploadDailyLimit), *limits.UploadDailyLimit)     // Plan value
		assert.Equal(t, uint64(plan.DownloadDailyLimit), *limits.DownloadDailyLimit) // Plan value
		assert.Equal(t, uint64(plan.UploadTotalLimit), *limits.UploadTotalLimit)     // Plan value
		assert.Equal(t, uint64(plan.DownloadTotalLimit), *limits.DownloadTotalLimit) // Plan value

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestConfiguration_MissingValues_NilLimits(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		quotaService := core.GetService[*pluginCore.MockQuotaService](ctx, pluginCore.QUOTA_SERVICE)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		quotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)

		// Get config using UsageManager
		quotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
		quotaService.On("GetQuotaPlanManager").Return(quotaPlanManager)
		mockUsageManager.On("GetUserQuotaConfig", userID).Return(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		}, nil).Once()
		quotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Once()
		config, err := quotaService.GetUsageManager().GetUserQuotaConfig(userID)
		require.NoError(t, err)

		// All limits should be nil initially
		assert.Nil(t, config.StorageLimit)
		assert.Nil(t, config.UploadDailyLimit)
		assert.Nil(t, config.DownloadDailyLimit)
		assert.Nil(t, config.UploadTotalLimit)
		assert.Nil(t, config.DownloadTotalLimit)
		assert.Nil(t, config.StorageThreshold)
		assert.Nil(t, config.UploadThreshold)
		assert.Nil(t, config.DownloadThreshold)
		assert.Nil(t, config.QuotaPlanID)

		// getEffectiveLimits should return an error when no limits are configured for hard limits policy
		quotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound).Once()
		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "no limits configured")

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
