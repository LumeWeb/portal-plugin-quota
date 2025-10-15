package policies

import (
	"errors"
	"strings"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// TestHardLimitsPolicyEnforcer_InvalidLimitValues tests invalid limit values for hard limits policy
func TestHardLimitsPolicyEnforcer_InvalidLimitValues(t *testing.T) {
	tests := []struct {
		name          string
		dailyLimit    *int64
		totalLimit    *int64
		expectedError string
	}{
		{
			name:          "Invalid daily limit",
			dailyLimit:    lo.ToPtr(int64(-2)),
			totalLimit:    lo.ToPtr(int64(5000)),
			expectedError: "invalid upload daily limit in user config",
		},
		{
			name:          "Invalid total limit",
			dailyLimit:    lo.ToPtr(int64(1000)),
			totalLimit:    lo.ToPtr(int64(-2)),
			expectedError: "invalid upload total limit in user config",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
			mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound)

			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

			config := &models.UserQuotaConfig{
				UserID:            2,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadDailyLimit:  test.dailyLimit,
				UploadTotalLimit:  test.totalLimit,
			}

			result, err := enforcer.CheckUploadQuota(config, uint64(500))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedError)
			assert.Equal(t, models.QuotaCheckReason(""), result.Reason)
		})
	}
}

// TestThresholdPolicyEnforcer_InvalidThresholdValues tests invalid threshold values for threshold policy
func TestThresholdPolicyEnforcer_InvalidThresholdValues(t *testing.T) {
	tests := []struct {
		name             string
		dailyLimit       int64
		threshold        *int64
		expectedError    string
		errorShouldBeNil bool
	}{
		{
			name:          "Invalid threshold value",
			dailyLimit:    1000,
			threshold:     lo.ToPtr(int64(-2)),
			expectedError: "invalid upload threshold value",
		},
		{
			name:          "Threshold exceeds limit",
			dailyLimit:    1000,
			threshold:     lo.ToPtr(int64(1500)),
			expectedError: "threshold cannot exceed limit",
		},
		{
			name:             "Nil threshold should work normally",
			dailyLimit:       1000,
			threshold:        nil,
			expectedError:    "",
			errorShouldBeNil: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
			mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound)

			// Add the missing mock expectation for GetTodayUsage
			if test.errorShouldBeNil {
				mockQuotaService.On("GetTodayUsage", uint(2)).Return(&pluginCore.Usage{
					UserID:        2,
					BytesUploaded: 0,
				}, nil)
			}

			enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

			config := &models.UserQuotaConfig{
				UserID:            2,
				EnforcementPolicy: models.EnforcementPolicyThreshold,
				UploadDailyLimit:  lo.ToPtr(test.dailyLimit),
				UploadThreshold:   test.threshold,
			}

			result, err := enforcer.CheckUploadQuota(config, uint64(500))

			if test.errorShouldBeNil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				assert.Equal(t, models.QuotaCheckReason(""), result.Reason)
			}
		})
	}
}

// TestAllowancePolicyEnforcer_ErrorHandling tests error handling for allowance policy
func TestAllowancePolicyEnforcer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*pluginCore.MockGrantManager)
		testFunc      func(*AllowancePolicyEnforcer) error
		expectedError string
	}{
		{
			name: "CheckUploadQuota - GetActiveGrants error",
			setupMocks: func(mockGrantManager *pluginCore.MockGrantManager) {
				mockGrantManager.On("GetActiveGrantsByType", uint(1), models.GrantTypeUpload).Return(nil, errors.New("grant manager error"))
			},
			testFunc: func(enforcer *AllowancePolicyEnforcer) error {
				config := &models.UserQuotaConfig{
					UserID:            1,
					EnforcementPolicy: models.EnforcementPolicyAllowance,
				}
				_, err := enforcer.CheckUploadQuota(config, uint64(500))
				return err
			},
			expectedError: "grant manager error",
		},
		{
			name: "RecordUpload - ConsumeFromGrants returns generic error",
			setupMocks: func(mockGrantManager *pluginCore.MockGrantManager) {
				mockGrantManager.On("ConsumeFromGrants", uint(1), models.GrantTypeUpload, uint64(100), mock.AnythingOfType("uint"), (*gorm.DB)(nil)).Return(nil, errors.New("grant manager error"))
			},
			testFunc: func(enforcer *AllowancePolicyEnforcer) error {
				return enforcer.RecordUpload(uint(1), uint(1), uint64(100), "192.168.1.1")
			},
			expectedError: "grant manager error",
		},
		{
			name: "RecordUpload - ConsumeFromGrants wraps as failed to consume upload allowance",
			setupMocks: func(mockGrantManager *pluginCore.MockGrantManager) {
				mockGrantManager.On("ConsumeFromGrants", uint(1), models.GrantTypeUpload, uint64(100), mock.AnythingOfType("uint"), (*gorm.DB)(nil)).Return(nil, errors.New("consumption error"))
			},
			testFunc: func(enforcer *AllowancePolicyEnforcer) error {
				return enforcer.RecordUpload(uint(1), uint(1), uint64(100), "192.168.1.1")
			},
			expectedError: "failed to consume upload allowance",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			mockGrantManager := pluginCore.NewMockGrantManager(t)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
			
			// Add mock expectation for RecordUserUsageDetail for RecordUpload tests
			// We identify RecordUpload tests by checking if the test function name contains "RecordUpload"
			if test.testFunc != nil && strings.Contains(test.name, "RecordUpload") {
				mockUsageManager.On("RecordUserUsageDetail", mock.AnythingOfType("*models.UserUsageDetail")).Return(nil)
			}

			test.setupMocks(mockGrantManager)

			enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

			err := test.testFunc(enforcer)
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
		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

		_, err := enforcer.limitResolver.ResolveEffectiveLimits(nil, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
	})
}
