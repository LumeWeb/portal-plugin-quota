package policies

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestBasePolicyEnforcer_CreateQuotaCheckResult(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	usageManager := pluginCore.NewMockUsageManager(t)
	reservationManager := pluginCore.NewMockReservationManager(t)
	enforcer := NewBasePolicyEnforcer(ctx, usageManager, reservationManager)

	t.Run("Success result", func(t *testing.T) {
		details := pluginCore.QuotaCheckDetails{
			CurrentUsage: 100,
			Policy:       models.EnforcementPolicyHardLimits,
		}

		result := enforcer.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyHardLimits, details)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, details, result.Details)
	})

	t.Run("Limit exceeded result", func(t *testing.T) {
		details := pluginCore.QuotaCheckDetails{
			CurrentUsage: 100,
			Limit:        lo.ToPtr(uint64(200)),
			Policy:       models.EnforcementPolicyHardLimits,
		}

		result := enforcer.createQuotaCheckResult(false, models.QuotaCheckReasonLimitExceeded, models.EnforcementPolicyHardLimits, details)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
		assert.Equal(t, details, result.Details)
	})
}

func TestBasePolicyEnforcer_CreateSuccessResult(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	usageManager := pluginCore.NewMockUsageManager(t)
	reservationManager := pluginCore.NewMockReservationManager(t)
	enforcer := NewBasePolicyEnforcer(ctx, usageManager, reservationManager)

	result := enforcer.createSuccessResult(models.EnforcementPolicyHardLimits)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestBasePolicyEnforcer_CreateFailureResult_LimitExceeded(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	usageManager := pluginCore.NewMockUsageManager(t)
	reservationManager := pluginCore.NewMockReservationManager(t)
	enforcer := NewBasePolicyEnforcer(ctx, usageManager, reservationManager)

	result := enforcer.createFailureResult(models.QuotaCheckReasonLimitExceeded, models.EnforcementPolicyHardLimits, pluginCore.QuotaCheckDetails{
		CurrentUsage: 150,
		Limit:        lo.ToPtr(uint64(200)),
		Policy:       models.EnforcementPolicyHardLimits,
	})
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
	assert.Equal(t, uint64(150), result.Details.CurrentUsage)
	assert.NotNil(t, result.Details.Limit)
	assert.Equal(t, uint64(200), *result.Details.Limit)
	assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
}

func TestBasePolicyEnforcer_CreateWarningResult(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	usageManager := pluginCore.NewMockUsageManager(t)
	reservationManager := pluginCore.NewMockReservationManager(t)
	enforcer := NewBasePolicyEnforcer(ctx, usageManager, reservationManager)

	result := enforcer.createWarningResult(
		models.EnforcementPolicyThreshold,
		150, // currentUsage
		120, // threshold
		200, // limit
	)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
	assert.Equal(t, uint64(150), result.Details.CurrentUsage)
	assert.NotNil(t, result.Details.Threshold)
	assert.Equal(t, uint64(120), *result.Details.Threshold)
	assert.NotNil(t, result.Details.Limit)
	assert.Equal(t, uint64(200), *result.Details.Limit)
	assert.Equal(t, models.EnforcementPolicyThreshold, result.Details.Policy)
}
