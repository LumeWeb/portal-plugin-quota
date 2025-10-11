package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestConfiguration_DefaultCreation(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		t.Run("Non-existent user gets default config", func(t *testing.T) {
			userID := uint(9999) // Non-existent user

			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)
			assert.NotNil(t, config)
			assert.Equal(t, userID, config.UserID)
			assert.Equal(t, models.EnforcementPolicyHardLimits, config.EnforcementPolicy)
			assert.Nil(t, config.StorageLimit)
			assert.Nil(t, config.UploadDailyLimit)
			assert.Nil(t, config.DownloadDailyLimit)
			assert.Nil(t, config.UploadTotalLimit)
			assert.Nil(t, config.DownloadTotalLimit)
			assert.Nil(t, config.StorageThreshold)
			assert.Nil(t, config.UploadThreshold)
			assert.Nil(t, config.DownloadThreshold)
			assert.Nil(t, config.QuotaPlanID)
		})
	}, pluginTesting.TestOptions())
}

func TestConfiguration_Updates(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		initialUploadLimit := int64(1000)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			UploadDailyLimit: &initialUploadLimit,
		})

		t.Run("Update configuration values", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Verify initial value
			assert.Equal(t, initialUploadLimit, *config.UploadDailyLimit)

			// Update the limit
			newUploadLimit := int64(2000)
			config.UploadDailyLimit = &newUploadLimit
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Verify updated value
			updatedConfig, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)
			assert.Equal(t, newUploadLimit, *updatedConfig.UploadDailyLimit)
		})

		t.Run("Update enforcement policy", func(t *testing.T) {
			enforcer := NewBasePolicyEnforcer(ctx)
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Update policy
			config.EnforcementPolicy = models.EnforcementPolicyUnlimited
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Verify updated policy
			updatedConfig, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)
			assert.Equal(t, models.EnforcementPolicyUnlimited, updatedConfig.EnforcementPolicy)
		})
	}, pluginTesting.TestOptions())
}

func TestConfiguration_QuotaPlanIntegration(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("User with quota plan gets plan limits", func(t *testing.T) {
			// Create a quota plan
			plan := createTestQuotaPlan(t, ctx, "Test Plan", false, &testPlanLimits{
				StorageLimit:       5000,
				UploadDailyLimit:   1000,
				DownloadDailyLimit: 2000,
				UploadTotalLimit:   10000,
				DownloadTotalLimit: 20000,
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
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			limits, err := enforcer.getEffectiveLimits(config)
			require.NoError(t, err)
			assert.Equal(t, uint64(plan.StorageLimit), *limits.StorageLimit)
			assert.Equal(t, uint64(plan.UploadDailyLimit), *limits.UploadDailyLimit)
			assert.Equal(t, uint64(plan.DownloadDailyLimit), *limits.DownloadDailyLimit)
			assert.Equal(t, uint64(plan.UploadTotalLimit), *limits.UploadTotalLimit)
			assert.Equal(t, uint64(plan.DownloadTotalLimit), *limits.DownloadTotalLimit)
		})

		t.Run("User with custom limits overrides plan limits", func(t *testing.T) {
			// Create a quota plan
			plan := createTestQuotaPlan(t, ctx, "Override Plan", false, &testPlanLimits{
				StorageLimit:       5000,
				UploadDailyLimit:   1000,
				DownloadDailyLimit: 2000,
				UploadTotalLimit:   10000,
				DownloadTotalLimit: 20000,
			})

			// Create user with plan and custom override
			customStorageLimit := int64(3000)
			planID := uint64(plan.ID)
			config := createTestUser(t, ctx, userID+1, models.EnforcementPolicyHardLimits, &testUserLimits{
				QuotaPlanID:  &planID,
				StorageLimit: &customStorageLimit,
			})

			// Test hard limits enforcer
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			limits, err := enforcer.getEffectiveLimits(config)
			require.NoError(t, err)
			assert.Equal(t, uint64(customStorageLimit), *limits.StorageLimit)            // Custom value
			assert.Equal(t, uint64(plan.UploadDailyLimit), *limits.UploadDailyLimit)     // Plan value
			assert.Equal(t, uint64(plan.DownloadDailyLimit), *limits.DownloadDailyLimit) // Plan value
			assert.Equal(t, uint64(plan.UploadTotalLimit), *limits.UploadTotalLimit)     // Plan value
			assert.Equal(t, uint64(plan.DownloadTotalLimit), *limits.DownloadTotalLimit) // Plan value
		})
	}, pluginTesting.TestOptions())
}

func TestConfiguration_MissingValues(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("Configuration with nil limits", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			config := &models.UserQuotaConfig{}
			err := ctx.DB().Where("user_id = ?", userID).First(config).Error
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
			limits, err := enforcer.getEffectiveLimits(config)
			assert.Error(t, err)
			assert.Nil(t, limits)
			assert.Contains(t, err.Error(), "no limits configured")
		})
	}, pluginTesting.TestOptions())
}
