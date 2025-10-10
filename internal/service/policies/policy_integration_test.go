package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestPolicyIntegration_PolicySwitching(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		uploadDailyLimit := uint64(1000)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			UploadDailyLimit: &uploadDailyLimit,
		})

		t.Run("Switch from hard limits to unlimited", func(t *testing.T) {
			// First test with hard limits policy
			hardLimitsEnforcer := NewHardLimitsPolicyEnforcer(ctx)
			config, err := hardLimitsEnforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Should be allowed within limit
			result, err := hardLimitsEnforcer.CheckUploadQuota(config, 500)
			require.NoError(t, err)
			assert.True(t, result.Allowed)

			// Should be blocked exceeding limit
			result, err = hardLimitsEnforcer.CheckUploadQuota(config, 1500)
			require.NoError(t, err)
			assert.False(t, result.Allowed)

			// Switch policy to unlimited
			config.EnforcementPolicy = models.EnforcementPolicyUnlimited
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Test with unlimited policy enforcer
			unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx)
			result, err = unlimitedEnforcer.CheckUploadQuota(config, 1500)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		})

		t.Run("Switch from unlimited to threshold", func(t *testing.T) {
			// First test with unlimited policy
			unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx)
			config, err := unlimitedEnforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Should always be allowed
			result, err := unlimitedEnforcer.CheckUploadQuota(config, 10000)
			require.NoError(t, err)
			assert.True(t, result.Allowed)

			// Switch policy to threshold
			uploadDailyLimit := uint64(1000)
			uploadThreshold := uint64(800)
			config.EnforcementPolicy = models.EnforcementPolicyThreshold
			config.UploadDailyLimit = &uploadDailyLimit
			config.UploadThreshold = &uploadThreshold
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			// Test with threshold policy enforcer
			thresholdEnforcer := NewThresholdPolicyEnforcer(ctx)
			result, err = thresholdEnforcer.CheckUploadQuota(userID, 10000)
			require.NoError(t, err)
			assert.False(t, result.Allowed) // Should be blocked now
		})
	}, testOptions())
}

func TestPolicyIntegration_MixedPolicies(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create users with different policies
		user1ID := uint(1)
		user2ID := uint(2)
		user3ID := uint(3)

		uploadDailyLimit := uint64(1000)
		createTestUser(t, ctx, user1ID, models.EnforcementPolicyHardLimits, &testUserLimits{
			UploadDailyLimit: &uploadDailyLimit,
		})

		createTestUser(t, ctx, user2ID, models.EnforcementPolicyUnlimited, &testUserLimits{})

		uploadThreshold := uint64(800)
		createTestUser(t, ctx, user3ID, models.EnforcementPolicyThreshold, &testUserLimits{
			UploadDailyLimit: &uploadDailyLimit,
			UploadThreshold:  &uploadThreshold,
		})

		t.Run("Each policy enforces correctly", func(t *testing.T) {
			// Hard limits user
			hardLimitsEnforcer := NewHardLimitsPolicyEnforcer(ctx)
			config1, err := hardLimitsEnforcer.getUserQuotaConfig(user1ID)
			require.NoError(t, err)

			result1, err := hardLimitsEnforcer.CheckUploadQuota(config1, 1500)
			require.NoError(t, err)
			assert.False(t, result1.Allowed) // Should be blocked

			// Unlimited user
			unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx)
			config2, err := unlimitedEnforcer.getUserQuotaConfig(user2ID)
			require.NoError(t, err)

			result2, err := unlimitedEnforcer.CheckUploadQuota(config2, 1500)
			require.NoError(t, err)
			assert.True(t, result2.Allowed) // Should be allowed

			// Threshold user
			thresholdEnforcer := NewThresholdPolicyEnforcer(ctx)
			_, err = thresholdEnforcer.getUserQuotaConfig(user3ID)
			require.NoError(t, err)

			result3, err3 := thresholdEnforcer.CheckUploadQuota(user3ID, 1500)
			require.NoError(t, err3)
			assert.False(t, result3.Allowed) // Should be blocked
		})
	}, testOptions())
}

func TestPolicyIntegration_ValidationConsistency(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		uploadDailyLimit := uint64(1000)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			UploadDailyLimit: &uploadDailyLimit,
		})

		t.Run("All policies validate user ID consistently", func(t *testing.T) {
			hardLimitsEnforcer := NewHardLimitsPolicyEnforcer(ctx)
			unlimitedEnforcer := NewUnlimitedPolicyEnforcer(ctx)
			thresholdEnforcer := NewThresholdPolicyEnforcer(ctx)
			mockGrantManager := createMockGrantManager(t)

			// Set up mock expectations for valid user ID
			mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeUpload).Return([]*models.AllowanceGrant{}, nil)
			mockGrantManager.On("CalculateAvailableBytes", []*models.AllowanceGrant{}).Return(uint64(0), nil)

			// Set up mock expectations for invalid user ID (0)
			// Note: This expectation should not be called because validation should fail first
			mockGrantManager.On("GetActiveGrantsByType", uint(0), models.GrantTypeUpload).Return([]*models.AllowanceGrant{}, models.ErrInvalidUserID).Maybe()

			allowanceEnforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Test with valid user ID
			config, err := hardLimitsEnforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			_, err1 := hardLimitsEnforcer.CheckUploadQuota(config, 100)
			_, err2 := unlimitedEnforcer.CheckUploadQuota(config, 100)
			_, err3 := thresholdEnforcer.CheckUploadQuota(userID, 100)
			_, err4 := allowanceEnforcer.CheckUploadQuota(config, 100)

			// All should succeed with valid user ID
			assert.NoError(t, err1)
			assert.NoError(t, err2)
			assert.NoError(t, err3)
			assert.NoError(t, err4)

			// Test with invalid user ID
			invalidConfig := &models.UserQuotaConfig{
				UserID:            0,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
			}

			_, err1 = hardLimitsEnforcer.CheckUploadQuota(invalidConfig, 100)
			_, err2 = unlimitedEnforcer.CheckUploadQuota(invalidConfig, 100)
			_, err3 = thresholdEnforcer.CheckUploadQuota(0, 100)
			_, err4 = allowanceEnforcer.CheckUploadQuota(invalidConfig, 100)

			// All should return the same error for invalid user ID
			assert.Error(t, err1)
			assert.Error(t, err2)
			assert.Error(t, err3)
			assert.Error(t, err4)

			expectedError := "user_id must be greater than 0"
			assert.Contains(t, err1.Error(), expectedError)
			assert.Contains(t, err2.Error(), expectedError)
			assert.Contains(t, err3.Error(), expectedError)
			assert.Contains(t, err4.Error(), expectedError)
		})
	}, testOptions())
}
