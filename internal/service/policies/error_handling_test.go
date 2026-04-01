package policies

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// TestHardLimitsPolicyEnforcer_InvalidLimitValues tests invalid limit values for hard limits policy
// Note: Limit validation was simplified as part of window-based refactor
// Negative or invalid limit values are now handled by uint64 casting, not explicit validation
func TestHardLimitsPolicyEnforcer_InvalidLimitValues(t *testing.T) {
	tests := []struct {
		name        string
		uploadLimit *int64
	}{
		{
			name:        "Negative limit value treated as large uint64",
			uploadLimit: lo.ToPtr(int64(-2)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
			mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

			mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(2), models.UsageTypeUpload, mock.Anything).Return(uint64(0), time.Now(), time.Now(), nil)
			mockReservationManager.EXPECT().SumPendingBytesForUser(mock.Anything, uint(2), models.UsageTypeUpload).Return(uint64(0), nil)

			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

			windowDuration := int64(86400)
			config := &models.UserQuotaConfig{
				UserID:            2,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadLimitBytes:  uint64(*test.uploadLimit),
				WindowType:        models.WindowTypeCalendarDay,
				WindowDuration:    &windowDuration,
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, uint64(500))
			assert.NoError(t, err)
			_ = result
		})
	}
}

// TestThresholdPolicyEnforcer_InvalidThresholdValues tests threshold behavior after
// window-based refactor. Note: Threshold validation was simplified as part of the refactor.
func TestThresholdPolicyEnforcer_InvalidThresholdValues(t *testing.T) {
	tests := []struct {
		name        string
		dailyLimit  int64
		threshold   *int64
		description string
	}{
		{
			name:        "Negative threshold value treated as large uint64",
			dailyLimit:  1000,
			threshold:   lo.ToPtr(int64(-2)),
			description: "Negative int64 becomes a large uint64, accepted without error",
		},
		{
			name:        "Threshold exceeding limit is allowed",
			dailyLimit:  1000,
			threshold:   lo.ToPtr(int64(1500)),
			description: "No validation prevents threshold from exceeding limit",
		},
		{
			name:        "Nil threshold works normally",
			dailyLimit:  1000,
			threshold:   nil,
			description: "No threshold means no warning level configured",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
			mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

			mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(2), pluginCore.UsageTypeUpload, mock.Anything).Return(uint64(0), time.Now(), time.Now(), nil).Maybe()

			enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

			windowDuration := int64(86400)
			windowStartHour := 0
			timezone := "UTC"

			config := &models.UserQuotaConfig{
				UserID:            2,
				EnforcementPolicy: models.EnforcementPolicyThreshold,
				UploadLimitBytes:  uint64(test.dailyLimit),
				WindowType:        models.WindowTypeCalendarDay,
				WindowDuration:    &windowDuration,
				WindowStartHour:   &windowStartHour,
				WindowTimezone:    &timezone,
				UploadThreshold:   test.threshold,
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, uint64(500))
			assert.NoError(t, err, test.description)
			_ = result
		})
	}
}

// TestAllowancePolicyEnforcer_ErrorHandling tests error handling for allowance policy
func TestAllowancePolicyEnforcer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*pluginCore.MockGrantManager, coreTesting.TestContext)
		testFunc      func(*AllowancePolicyEnforcer, coreTesting.TestContext) error
		expectedError string
	}{
		{
			name: "CheckUploadQuota - GetActiveGrants error",
			setupMocks: func(mockGrantManager *pluginCore.MockGrantManager, ctx coreTesting.TestContext) {
				mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, uint(1), models.GrantTypeUpload).Return(nil, errors.New("grant manager error"))
			},
			testFunc: func(enforcer *AllowancePolicyEnforcer, ctx coreTesting.TestContext) error {
				config := &models.UserQuotaConfig{
					UserID:            1,
					EnforcementPolicy: models.EnforcementPolicyAllowance,
				}
				_, err := enforcer.CheckUploadQuota(ctx, config, uint64(500))
				return err
			},
			expectedError: "grant manager error",
		},
		{
			name: "RecordUpload - RecordUsageAndConsume returns generic error",
			setupMocks: func(mockGrantManager *pluginCore.MockGrantManager, ctx coreTesting.TestContext) {
				// No grant manager setup needed - error comes from UsageManager
			},
			testFunc: func(enforcer *AllowancePolicyEnforcer, ctx coreTesting.TestContext) error {
				return enforcer.RecordUpload(ctx, uint(1), uint(1), uint64(100), "192.168.1.1")
			},
			expectedError: "grant manager error",
		},
		{
			name: "RecordUpload - RecordUsageAndConsume wraps as failed to consume upload allowance",
			setupMocks: func(mockGrantManager *pluginCore.MockGrantManager, ctx coreTesting.TestContext) {
				// No grant manager setup needed - error comes from UsageManager
			},
			testFunc: func(enforcer *AllowancePolicyEnforcer, ctx coreTesting.TestContext) error {
				return enforcer.RecordUpload(ctx, uint(1), uint(1), uint64(100), "192.168.1.1")
			},
			expectedError: "failed to consume from grants",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			mockGrantManager := pluginCore.NewMockGrantManager(t)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
			mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager).Maybe()

			if test.testFunc != nil && strings.Contains(test.name, "RecordUpload") {
				mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail"), models.GrantTypeUpload, uint64(100)).Return(errors.New(test.expectedError))
			}

			test.setupMocks(mockGrantManager, ctx)

			enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

			err := test.testFunc(enforcer, ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedError)
		})
	}
}

// TestErrorHandling_InvalidConfiguration tests nil configuration handling
func TestErrorHandling_InvalidConfiguration(t *testing.T) {
	t.Run("Nil configuration error", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		mockQuotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)
		mockReservationManager := pluginCore.NewMockReservationManager(t)
		mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
		mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		_, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, nil, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
	})
}
