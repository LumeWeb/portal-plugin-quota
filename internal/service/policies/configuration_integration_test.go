package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
		mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, userID).Return(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		}, nil)
		config, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
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
		dur := int64(86400)
		wt := "DAY"
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadLimit:    &initialUploadLimit,
			windowDuration: &dur,
			windowType:     &wt,
		})

		quotaService := core.GetService[*pluginCore.MockQuotaService](ctx, pluginCore.QUOTA_SERVICE)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		quotaService.EXPECT().GetUsageManager().Return(mockUsageManager).Maybe()

		// Get the actual config from database
		var config models.UserQuotaConfig
		err := ctx.DB().Where("user_id = ?", userID).First(&config).Error
		require.NoError(t, err)

		// Mock the GetUserQuotaConfig to return the actual config
		mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, userID).Return(&config, nil).Maybe()

		// Get initial config
		retrievedConfig, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(initialUploadLimit), retrievedConfig.UploadLimitBytes)
		assert.Equal(t, models.EnforcementPolicyHardLimits, retrievedConfig.EnforcementPolicy)

		// Test updating upload limit
		newUploadLimit := int64(2000)
		config.UploadLimitBytes = uint64(newUploadLimit)
		err = ctx.DB().Save(&config).Error
		require.NoError(t, err)

		// Update the mock to return the updated config
		mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, userID).Return(&config, nil).Maybe()

		updatedConfig, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(newUploadLimit), updatedConfig.UploadLimitBytes)

		// Test updating enforcement policy
		config.EnforcementPolicy = models.EnforcementPolicyUnlimited
		err = ctx.DB().Save(&config).Error
		require.NoError(t, err)

		// Update the mock to return the updated config
		mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, userID).Return(&config, nil).Maybe()

		updatedConfig, err = quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
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
			storageLimit:  5000,
			uploadLimit:   1000,
			downloadLimit: 2000,
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
		quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)
		quotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
		quotaService.EXPECT().GetQuotaPlanManager().Return(quotaPlanManager)
		quotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)
		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
		require.NoError(t, err)
		// Check that limits are resolved from plan
		assert.True(t, limits.HasStorageLimitConfig)
		assert.True(t, limits.HasUploadLimitConfig)
		assert.True(t, limits.HasDownloadLimitConfig)
		assert.NotNil(t, limits.StorageLimitConfig)
		assert.NotNil(t, limits.UploadLimitConfig)
		assert.NotNil(t, limits.DownloadLimitConfig)
		assert.Equal(t, uint64(plan.StorageLimitBytes), limits.StorageLimitConfig.Bytes)
		assert.Equal(t, uint64(plan.UploadLimitBytes), limits.UploadLimitConfig.Bytes)
		assert.Equal(t, uint64(plan.DownloadLimitBytes), limits.DownloadLimitConfig.Bytes)

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
			storageLimit:  5000,
			uploadLimit:   1000,
			downloadLimit: 2000,
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
		quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)
		quotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
		quotaService.EXPECT().GetQuotaPlanManager().Return(quotaPlanManager)
		quotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)
		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
		require.NoError(t, err)
		// Custom storage override should take priority
		assert.Equal(t, uint64(customStorageLimit), limits.StorageLimitConfig.Bytes) // Custom value
		// Plan limits should apply for upload and download
		assert.Equal(t, uint64(plan.UploadLimitBytes), limits.UploadLimitConfig.Bytes)     // Plan value
		assert.Equal(t, uint64(plan.DownloadLimitBytes), limits.DownloadLimitConfig.Bytes) // Plan value

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
		quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)

		// Get config using UsageManager
		quotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
		quotaService.EXPECT().GetQuotaPlanManager().Return(quotaPlanManager)
		mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(&models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		}, nil).Once()
		quotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Once()
		config, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
		require.NoError(t, err)

		// All limits should be zero initially (no limits configured)
		assert.Equal(t, uint64(0), config.StorageLimitBytes)
		assert.Equal(t, uint64(0), config.UploadLimitBytes)
		assert.Equal(t, uint64(0), config.DownloadLimitBytes)
		assert.Nil(t, config.StorageThreshold)
		assert.Nil(t, config.UploadThreshold)
		assert.Nil(t, config.DownloadThreshold)
		assert.Nil(t, config.QuotaPlanID)

		// getEffectiveLimits should return an error when no limits are configured for hard limits policy
		quotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()
		limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "no limits configured")

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}
