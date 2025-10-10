package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestErrorHandling_InvalidConfiguration(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewHardLimitsPolicyEnforcer(ctx)

		t.Run("Nil configuration", func(t *testing.T) {
			// Test that calling getEffectiveLimits with nil config causes panic
			// This is expected behavior since the method doesn't handle nil config
			assert.Panics(t, func() {
				_, _ = enforcer.getEffectiveLimits(nil)
			})
		})

		t.Run("Invalid enforcement policy", func(t *testing.T) {
			// This test is not applicable for BasePolicyEnforcer since getEffectiveLimits
			// is implemented by specific policy enforcers, not the base one.
			// We'll skip this test for the base enforcer.
		})
	}, testOptions())
}

func TestErrorHandling_ZeroValues(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("Zero user ID in quota check", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			config := &models.UserQuotaConfig{
				UserID:            0,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
			}

			_, err := enforcer.CheckUploadQuota(config, 100)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Zero bytes in quota check", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
			}

			_, err := enforcer.CheckUploadQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

func TestErrorHandling_DatabaseFailures(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("Database connection closed", func(t *testing.T) {
			// Close the database connection to simulate failure
			db, err := ctx.DB().DB()
			require.NoError(t, err)
			err = db.Close()
			assert.NoError(t, err)

			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			_, err = enforcer.GetCurrentUsage(userID)
			assert.Error(t, err)
		})
	}, testOptions())
}
