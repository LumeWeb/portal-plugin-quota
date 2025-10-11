package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestModelValidation_EnforcementPolicy(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {

		t.Run("Valid enforcement policies", func(t *testing.T) {
			validPolicies := []models.EnforcementPolicy{
				models.EnforcementPolicyHardLimits,
				models.EnforcementPolicyUnlimited,
				models.EnforcementPolicyThreshold,
				models.EnforcementPolicyAllowance,
			}

			for i, policy := range validPolicies {
				userID := uint(i + 1) // Use different user ID for each test
				config := &models.UserQuotaConfig{
					UserID:            userID,
					EnforcementPolicy: policy,
				}

				err := ctx.DB().Create(config).Error
				assert.NoError(t, err)

				// Clean up
				err = ctx.DB().Delete(config).Error
				assert.NoError(t, err)
			}
		})

		t.Run("Invalid enforcement policy", func(t *testing.T) {
			userID := uint(999) // Use a different user ID to avoid conflicts
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: "INVALID_POLICY",
			}

			// Test that model validation prevents creating invalid configs
			err := ctx.DB().Create(config).Error
			// The model's BeforeCreate hook should reject invalid enforcement policies
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "enforcement policy is invalid")
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_UsageType(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(100) // Use a different base user ID to avoid conflicts

		t.Run("Valid usage types", func(t *testing.T) {
			validTypes := []models.UsageType{
				models.UsageTypeUpload,
				models.UsageTypeDownload,
				models.UsageTypeStorageAdd,
				models.UsageTypeStorageRemove,
			}

			for i, usageType := range validTypes {
				userID := baseUserID + uint(i) // Use different user ID for each test
				detail := &models.UserUsageDetail{
					UserID:    userID,
					UploadID:  1,
					Type:      usageType,
					Bytes:     100,
					IP:        "192.168.1.1",
					Timestamp: time.Now(),
				}

				err := ctx.DB().Create(detail).Error
				assert.NoError(t, err)

				// Clean up
				err = ctx.DB().Delete(detail).Error
				assert.NoError(t, err)
			}
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantType(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(200) // Use a different base user ID to avoid conflicts

		t.Run("Valid grant types", func(t *testing.T) {
			validTypes := []models.GrantType{
				models.GrantTypeStorage,
				models.GrantTypeUpload,
				models.GrantTypeDownload,
			}

			for i, grantType := range validTypes {
				userID := baseUserID + uint(i) // Use different user ID for each test
				grant := &models.AllowanceGrant{
					UserID:         userID,
					Type:           grantType,
					Source:         models.GrantSourcePAYGAddon,
					Bytes:          1000,
					BytesUsed:      0,
					BytesRemaining: 1000,
					IsActive:       true,
				}

				err := ctx.DB().Create(grant).Error
				assert.NoError(t, err)

				// Clean up
				err = ctx.DB().Delete(grant).Error
				assert.NoError(t, err)
			}
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantSource(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		baseUserID := uint(300) // Use a different base user ID to avoid conflicts

		t.Run("Valid grant sources", func(t *testing.T) {
			validSources := []models.GrantSource{
				models.GrantSourceSubscription,
				models.GrantSourcePAYGAddon,
				models.GrantSourceBonus,
				models.GrantSourcePromo,
			}

			for i, source := range validSources {
				userID := baseUserID + uint(i) // Use different user ID for each test
				grant := &models.AllowanceGrant{
					UserID:         userID,
					Type:           models.GrantTypeUpload,
					Source:         source,
					Bytes:          1000,
					BytesUsed:      0,
					BytesRemaining: 1000,
					IsActive:       true,
				}

				err := ctx.DB().Create(grant).Error
				assert.NoError(t, err)

				// Clean up
				err = ctx.DB().Delete(grant).Error
				assert.NoError(t, err)
			}
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_QuotaCheckReason(t *testing.T) {
	t.Run("Valid quota check reasons", func(t *testing.T) {
		validReasons := []models.QuotaCheckReason{
			models.QuotaCheckReasonOK,
			models.QuotaCheckReasonLimitExceeded,
			models.QuotaCheckReasonAllowanceDepleted,
			models.QuotaCheckReasonWarningThreshold,
		}

		for _, reason := range validReasons {
			// Just verify they exist as constants
			assert.NotEmpty(t, string(reason))
		}
	})
}
