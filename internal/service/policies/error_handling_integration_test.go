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
)

func TestErrorHandling_ZeroValues(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)

		t.Run("Zero user ID in quota check", func(t *testing.T) {
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
			config := &models.UserQuotaConfig{
				UserID:            0,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
			}

			_, err := enforcer.CheckUploadQuota(config, 100)
			assert.Error(t, err)
			assert.ErrorIs(t, err, models.ErrInvalidUserID)
		})

		t.Run("Zero bytes in quota check", func(t *testing.T) {
			userID := dataManager.NextUserID()
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})
			
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
			}

			_, err := enforcer.CheckUploadQuota(config, 0)
			assert.Error(t, err)
			assert.ErrorIs(t, err, models.ErrInvalidBytes)
		})

		dataManager.Cleanup()
	}, pluginTesting.TestOptions())
}

func TestErrorHandling_DatabaseFailures(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("Database connection closed", func(t *testing.T) {
			// Cleanup test data before closing the database
			dataManager.Cleanup()

			// Close the database connection to simulate failure
			db, err := ctx.DB().DB()
			require.NoError(t, err)
			err = db.Close()
			assert.NoError(t, err)

			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			mockUsageManager.On("GetCurrentUsage", userID).Return(nil, assert.AnError)
			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)
			_, err = enforcer.GetCurrentUsage(userID)
			assert.Error(t, err)
		})
	}, pluginTesting.TestOptions())
}
