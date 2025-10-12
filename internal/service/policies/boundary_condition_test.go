package policies

import (
	"errors"
	"math"
	"testing"

	"github.com/docker/go-units"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// TestHardLimitsPolicyEnforcer_BoundaryConditions tests boundary conditions for hard limits policy
func TestHardLimitsPolicyEnforcer_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name            string
		userID          uint
		dailyLimit      int64
		totalLimit      int64
		currentUsage    uint64
		requestBytes    uint64
		expectedAllowed bool
		expectedReason  models.QuotaCheckReason
	}{
		{
			name:            "Zero limit (disabled)",
			userID:          1,
			dailyLimit:      0, // Disabled
			totalLimit:      5000,
			currentUsage:    100,
			requestBytes:    500,
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
		},
		{
			name:            "Negative one limit (unlimited)",
			userID:          2,
			dailyLimit:      -1, // Unlimited
			totalLimit:      -1, // Unlimited
			currentUsage:    100,
			requestBytes:    500,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Exactly at limit",
			userID:          3,
			dailyLimit:      1000,
			totalLimit:      5000,
			currentUsage:    1000, // Exactly at daily limit
			requestBytes:    1,
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
		},
		{
			name:            "One byte under limit",
			userID:          4,
			dailyLimit:      1000,
			totalLimit:      5000,
			currentUsage:    999, // One byte under daily limit
			requestBytes:    1,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Maximum uint64 bytes",
			userID:          5,
			dailyLimit:      -1, // Unlimited
			totalLimit:      -1, // Unlimited
			currentUsage:    100,
			requestBytes:    ^uint64(0), // Maximum uint64
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Mixed limit types (zero and unlimited)",
			userID:          6,
			dailyLimit:      0,  // Disabled
			totalLimit:      -1, // Unlimited
			currentUsage:    100,
			requestBytes:    500,
			expectedAllowed: false, // Daily limit is disabled (0)
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadDailyLimit:  lo.ToPtr(test.dailyLimit),
				UploadTotalLimit:  lo.ToPtr(test.totalLimit),
			}

			mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
				UserID:        userID,
				BytesUploaded: test.currentUsage,
			}, nil)
			mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
			mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(&models.QuotaPlan{}, nil)

			// For tests that need usage aggregator - only set it up when daily limit check would pass
			if test.dailyLimit != 0 && test.dailyLimit != -1 && (test.currentUsage+test.requestBytes <= uint64(test.dailyLimit) || test.dailyLimit == -1) {
				mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)
				mockQuotaService.On("GetUsageAggregator").Return(mockUsageAggregator)
				mockUsageAggregator.On("GetAggregatedUsageByType", userID, models.UsageTypeUpload).Return(uint64(0), nil)
			}

			result, err := enforcer.CheckUploadQuota(config, test.requestBytes)
			require.NoError(t, err)
			assert.Equal(t, test.expectedAllowed, result.Allowed)
			assert.Equal(t, test.expectedReason, result.Reason)

			dataManager.Cleanup()
		})
	}
}

// TestThresholdPolicyEnforcer_BoundaryConditions tests boundary conditions for threshold policy
func TestThresholdPolicyEnforcer_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name            string
		userID          uint
		dailyLimit      int64
		threshold       *int64
		currentUsage    uint64
		requestBytes    uint64
		expectedAllowed bool
		expectedReason  models.QuotaCheckReason
	}{
		{
			name:            "Zero threshold always warns",
			userID:          1,
			dailyLimit:      1000,
			threshold:       lo.ToPtr(int64(0)), // Always warn
			currentUsage:    100,
			requestBytes:    100,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonWarningThreshold,
		},
		{
			name:            "Below threshold no warning",
			userID:          2,
			dailyLimit:      2000,
			threshold:       lo.ToPtr(int64(1000)),
			currentUsage:    100, // Current usage: 100
			requestBytes:    50,  // Request: 50
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Crossing threshold should warn",
			userID:          3,
			dailyLimit:      2000,
			threshold:       lo.ToPtr(int64(1000)),
			currentUsage:    950, // Current usage: 950
			requestBytes:    100, // Request: 100
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonWarningThreshold,
		},
		{
			name:            "Exactly at threshold should warn",
			userID:          4,
			dailyLimit:      2000,
			threshold:       lo.ToPtr(int64(1000)),
			currentUsage:    999, // Current usage: 999
			requestBytes:    1,   // Request: 1
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonWarningThreshold,
		},
		{
			name:            "Above threshold but within limit should warn",
			userID:          5,
			dailyLimit:      2000,
			threshold:       lo.ToPtr(int64(1000)),
			currentUsage:    1200, // Current usage: 1200
			requestBytes:    100,  // Request: 100
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonWarningThreshold,
		},
		{
			name:            "Exceed limit should block even if threshold crossed",
			userID:          6,
			dailyLimit:      2000,
			threshold:       lo.ToPtr(int64(1000)),
			currentUsage:    1900, // Current usage: 1900
			requestBytes:    200,  // Request: 200
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
		},
		{
			name:            "Nil threshold should work normally",
			userID:          7,
			dailyLimit:      1000,
			threshold:       nil, // No threshold
			currentUsage:    900,
			requestBytes:    100,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Large threshold value",
			userID:          8,
			dailyLimit:      int64(units.TiB),           // 1 TiB
			threshold:       lo.ToPtr(int64(units.TiB)), // 1 TiB - Large but reasonable
			currentUsage:    0,
			requestBytes:    1,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyThreshold,
				UploadDailyLimit:  lo.ToPtr(test.dailyLimit),
				UploadThreshold:   test.threshold,
			}

			mockQuotaService.On("GetTodayUsage", userID).Return(&pluginCore.Usage{
				UserID:        userID,
				BytesUploaded: test.currentUsage,
			}, nil)
			mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
			mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound)

			result, err := enforcer.CheckUploadQuota(config, test.requestBytes)
			require.NoError(t, err)
			assert.Equal(t, test.expectedAllowed, result.Allowed)
			assert.Equal(t, test.expectedReason, result.Reason)

			dataManager.Cleanup()
		})
	}
}

// TestAllowancePolicyEnforcer_BoundaryConditions tests boundary conditions for allowance policy
func TestAllowancePolicyEnforcer_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name              string
		userID            uint
		grants            []*models.AllowanceGrant
		requestBytes      uint64
		expectedAllowed   bool
		expectedReason    models.QuotaCheckReason
		expectedAllowance *uint64
	}{
		{
			name:              "Zero allowance",
			userID:            1,
			grants:            []*models.AllowanceGrant{}, // No grants
			requestBytes:      1,
			expectedAllowed:   false,
			expectedReason:    models.QuotaCheckReasonAllowanceDepleted,
			expectedAllowance: lo.ToPtr(uint64(0)),
		},
		{
			name:   "Exactly at allowance limit",
			userID: 2,
			grants: []*models.AllowanceGrant{
				{
					UserID:         2,
					Type:           models.GrantTypeUpload,
					Source:         models.GrantSourcePAYGAddon,
					Bytes:          1000,
					BytesUsed:      0,
					BytesRemaining: 1000,
					IsActive:       true,
				},
			},
			requestBytes:      1000, // Exactly matches allowance
			expectedAllowed:   true,
			expectedReason:    models.QuotaCheckReasonOK,
			expectedAllowance: lo.ToPtr(uint64(1000)),
		},
		{
			name:   "One byte over allowance",
			userID: 3,
			grants: []*models.AllowanceGrant{
				{
					UserID:         3,
					Type:           models.GrantTypeUpload,
					Source:         models.GrantSourcePAYGAddon,
					Bytes:          1000,
					BytesUsed:      0,
					BytesRemaining: 1000,
					IsActive:       true,
				},
			},
			requestBytes:      1001, // One byte over allowance
			expectedAllowed:   false,
			expectedReason:    models.QuotaCheckReasonAllowanceDepleted,
			expectedAllowance: lo.ToPtr(uint64(1000)),
		},
		{
			name:   "Maximum uint64 allowance",
			userID: 4,
			grants: []*models.AllowanceGrant{
				{
					UserID:         4,
					Type:           models.GrantTypeUpload,
					Source:         models.GrantSourcePAYGAddon,
					Bytes:          ^uint64(0), // Maximum uint64
					BytesUsed:      0,
					BytesRemaining: ^uint64(0),
					IsActive:       true,
				},
			},
			requestBytes:      ^uint64(0), // Maximum uint64
			expectedAllowed:   true,
			expectedReason:    models.QuotaCheckReasonOK,
			expectedAllowance: lo.ToPtr(^uint64(0)),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockGrantManager := pluginCore.NewMockGrantManager(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyAllowance,
			}

			// Set up mock expectations
			mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
			mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeUpload).Return(test.grants, nil)
			if len(test.grants) > 0 {
				mockGrantManager.On("CalculateAvailableBytes", test.grants).Return(*test.expectedAllowance)
			} else {
				mockGrantManager.On("CalculateAvailableBytes", test.grants).Return(uint64(0))
			}

			result, err := enforcer.CheckUploadQuota(config, test.requestBytes)
			require.NoError(t, err)
			assert.Equal(t, test.expectedAllowed, result.Allowed)
			assert.Equal(t, test.expectedReason, result.Reason)
			assert.Equal(t, test.expectedAllowance, result.Details.Allowance)

			dataManager.Cleanup()
		})
	}
}

// TestUnlimitedPolicyEnforcer_BoundaryConditions tests boundary conditions for unlimited policy
func TestUnlimitedPolicyEnforcer_BoundaryConditions(t *testing.T) {
	t.Run("RecordUpload with usage manager error", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)

		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := uint64(100)
		ip := "192.168.1.1"

		// Set up mock expectations
		mockUsageManager.On("RecordUpload", userID, uploadID, bytes, ip).Return(errors.New("usage manager error"))

		err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "usage manager error")

		dataManager.Cleanup()
	})

	t.Run("CheckUploadQuota with maximum uint64 bytes request", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)

		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyUnlimited,
		}

		result, err := enforcer.CheckUploadQuota(config, uint64(^uint64(0))) // Maximum uint64
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)

		dataManager.Cleanup()
	})

	t.Run("RecordStorageChange with storage removal", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)

		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := int64(-100) // Negative bytes (removal)
		ip := "192.168.1.1"

		mockUsageManager.On("RecordStorageChange", userID, uploadID, bytes, ip).Return(nil)

		err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
		assert.NoError(t, err)
		mockUsageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, bytes, ip)

		dataManager.Cleanup()
	})

	t.Run("RecordStorageChange with maximum int64 storage change", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)

		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := int64(math.MaxInt64) // Maximum int64
		ip := "192.168.1.1"

		mockUsageManager.On("RecordStorageChange", userID, uploadID, bytes, ip).Return(nil)

		err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
		assert.NoError(t, err)
		mockUsageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, bytes, ip)

		dataManager.Cleanup()
	})

	t.Run("RecordStorageChange with minimum int64 storage change", func(t *testing.T) {
		ctx, _ := coreTesting.NewTestContext(t)
		dataManager := testdata.NewTestDataManager(ctx)
		mockQuotaService := pluginCore.NewMockQuotaService(t)
		mockUsageManager := pluginCore.NewMockUsageManager(t)

		mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
		enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

		userID := dataManager.NextUserID()
		uploadID := dataManager.NextUploadID()
		bytes := int64(math.MinInt64) // Minimum int64
		ip := "192.168.1.1"

		mockUsageManager.On("RecordStorageChange", userID, uploadID, bytes, ip).Return(nil)

		err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
		assert.NoError(t, err)
		mockUsageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, bytes, ip)

		dataManager.Cleanup()
	})
}
