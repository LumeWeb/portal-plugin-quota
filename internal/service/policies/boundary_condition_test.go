package policies

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		expectError     bool
	}{
		{
			name:            "One byte under limit",
			userID:          1,
			dailyLimit:      1000,
			totalLimit:      5000,
			currentUsage:    999, // One byte under daily limit
			requestBytes:    1,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
			expectError:     false,
		},
		{
			name:            "Exactly at limit",
			userID:          2,
			dailyLimit:      1000,
			totalLimit:      5000,
			currentUsage:    1000, // Exactly at daily limit
			requestBytes:    1,
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
			expectError:     false,
		},
		{
			name:            "One byte over limit",
			userID:          3,
			dailyLimit:      1000,
			totalLimit:      5000,
			currentUsage:    1000, // At daily limit
			requestBytes:    2,    // One byte over
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
			expectError:     false,
		},
		{
			name:            "Large request within limit",
			userID:          4,
			dailyLimit:      5000,
			totalLimit:      10000,
			currentUsage:    0,
			requestBytes:    4000,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
			expectError:     false,
		},
		{
			name:            "Large request exceeds limit",
			userID:          5,
			dailyLimit:      5000,
			totalLimit:      10000,
			currentUsage:    1000,
			requestBytes:    5000,
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonLimitExceeded,
			expectError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			// Set limit based on dailyLimit (using daily window type)
			// Note: New design uses single limit with window type
			uploadLimit := uint64(test.dailyLimit)
			windowType := models.WindowTypeCalendarDay // Default to daily window
			windowDuration := int64(86400)             // 1 day in seconds
			
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadLimitBytes:  uploadLimit,
				WindowType:        windowType,
				WindowDuration:    &windowDuration,
			}

			mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

			// Build window from config parameters
			var window pluginCore.LimitWindow
			if windowType == models.WindowTypeLifetime {
				window = pluginCore.LimitWindow{
					Type:     pluginCore.WindowTypeLifetime,
					Duration: lo.ToPtr(int64(0)),
				}
			} else {
				window = pluginCore.LimitWindow{
					Type:     pluginCore.WindowTypeCalendarDay,
					Duration: lo.ToPtr(windowDuration),
				}
			}

			// Setup window-based usage queries
			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			now := time.Now()
			windowStart := now.Add(-24 * time.Hour)
			mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, pluginCore.UsageTypeUpload, window).Return(test.currentUsage, windowStart, now, nil)

			result, err := enforcer.CheckUploadQuota(ctx, config, test.requestBytes)
			if test.expectError {
				assert.Error(t, err)
			} else if test.requestBytes == 0 {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "bytes must be greater than 0")
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedAllowed, result.Allowed)
				assert.Equal(t, test.expectedReason, result.Reason)
			}

			dataManager.Cleanup()
		})
	}
}

// TestThresholdPolicyEnforcer_BoundaryConditions tests boundary conditions for threshold policy
func TestThresholdPolicyEnforcer_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name              string
		userID            uint
		dailyLimit        int64
		threshold         *int64
		currentUsage      uint64
		requestBytes      uint64
		expectedAllowed   bool
		expectedReason    models.QuotaCheckReason
		expectError       bool
		skipGetTodayUsage bool
	}{
		{
			name:            "Zero threshold always warns",
			userID:          1,
			dailyLimit:      1000,
			threshold:       lo.ToPtr(int64(0)), // Always warn
			currentUsage:    0,
			requestBytes:    1,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonWarningThreshold,
			expectError:     false,
		},
		{
			name:            "Threshold at limit warns but allows",
			userID:          2,
			dailyLimit:      1000,
			threshold:       lo.ToPtr(int64(1000)), // Warn at limit
			currentUsage:    900,
			requestBytes:    100,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonWarningThreshold,
			expectError:     false,
		},
		{
			name:            "Threshold nil means no warning",
			userID:          3,
			dailyLimit:      1000,
			threshold:       nil, // No threshold set
			currentUsage:    100,
			requestBytes:    500,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
			expectError:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager).Maybe()
			enforcer := NewThresholdPolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			// Set limit based on dailyLimit (using daily window type)
			uploadLimit := uint64(test.dailyLimit)
			windowType := models.WindowTypeCalendarDay
			windowDuration := int64(86400)
			
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyThreshold,
				UploadLimitBytes:  uploadLimit,
				WindowType:        windowType,
				WindowDuration:    &windowDuration,
				UploadThreshold:   test.threshold,
			}

			// Only set up mocks if the error doesn't happen early
			if !test.skipGetTodayUsage {
				mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
				mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(&models.QuotaPlan{}, nil)

				// Build window from config parameters
				var window pluginCore.LimitWindow
				if windowType == models.WindowTypeLifetime {
					window = pluginCore.LimitWindow{
						Type:     pluginCore.WindowTypeLifetime,
						Duration: lo.ToPtr(int64(0)),
					}
				} else {
					window = pluginCore.LimitWindow{
						Type:     pluginCore.WindowTypeCalendarDay,
						Duration: lo.ToPtr(windowDuration),
					}
				}

				// Setup window-based usage queries
				now := time.Now()
				windowStart := now.Add(-24 * time.Hour)
				mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, pluginCore.UsageTypeUpload, window).Return(test.currentUsage, windowStart, now, nil)
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, test.requestBytes)
			if test.requestBytes == 0 {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "bytes must be greater than 0")
			} else if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "threshold cannot exceed limit")
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedAllowed, result.Allowed)
				assert.Equal(t, test.expectedReason, result.Reason)
			}

			dataManager.Cleanup()
		})
	}
}

// TestAllowancePolicyEnforcer_BoundaryConditions tests boundary conditions for allowance policy
func TestAllowancePolicyEnforcer_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name            string
		userID          uint
		allowanceBytes  uint64
		usedBytes       uint64
		requestBytes    uint64
		expectedAllowed bool
		expectedReason  models.QuotaCheckReason
	}{
		{
			name:            "Zero allowance blocks all requests",
			userID:          1,
			allowanceBytes:  0,
			usedBytes:       0,
			requestBytes:    1,
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonAllowanceDepleted,
		},
		{
			name:            "Request exactly at allowance",
			userID:          2,
			allowanceBytes:  1000,
			usedBytes:       0,
			requestBytes:    1000,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Request one byte over allowance",
			userID:          3,
			allowanceBytes:  1000,
			usedBytes:       0,
			requestBytes:    1001,
			expectedAllowed: false,
			expectedReason:  models.QuotaCheckReasonAllowanceDepleted,
		},
		{
			name:            "Request when partially used",
			userID:          4,
			allowanceBytes:  1000,
			usedBytes:       500,
			requestBytes:    500,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Request when one byte remaining",
			userID:          5,
			allowanceBytes:  1000,
			usedBytes:       999,
			requestBytes:    1,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Maximum uint64 allowance",
			userID:          6,
			allowanceBytes:  math.MaxUint64,
			usedBytes:       0,
			requestBytes:    math.MaxUint64,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockGrantManager := pluginCore.NewMockGrantManager(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyAllowance,
			}

			grants := []*models.AllowanceGrant{
				{
					UserID:         userID,
					Type:           models.GrantTypeUpload,
					Source:         models.GrantSourceBonus,
					Bytes:          test.allowanceBytes,
					BytesUsed:      test.usedBytes,
					BytesRemaining: test.allowanceBytes - test.usedBytes,
					IsActive:       true,
				},
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeUpload).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(test.allowanceBytes - test.usedBytes)

			result, err := enforcer.CheckUploadQuota(ctx, config, test.requestBytes)
			if test.requestBytes == 0 {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "bytes must be greater than 0")
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedAllowed, result.Allowed)
				assert.Equal(t, test.expectedReason, result.Reason)
			}

			dataManager.Cleanup()
		})
	}
}

// TestAllowancePolicyEnforcer_RecordUpload_BoundaryConditions tests boundary conditions for recording uploads
func TestAllowancePolicyEnforcer_RecordUpload_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		uploadID      uint
		bytes         uint64
		ip            string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Zero bytes should fail validation",
			userID:        1,
			uploadID:      100,
			bytes:         0,
			ip:            "192.168.1.1",
			expectError:   true,
			errorContains: "bytes must be greater than 0",
		},
		{
			name:          "Valid upload should succeed",
			userID:        2,
			uploadID:      100,
			bytes:         1000,
			ip:            "192.168.1.1",
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "Maximum uint64 bytes should succeed",
			userID:        3,
			uploadID:      100,
			bytes:         math.MaxUint64,
			ip:            "192.168.1.1",
			expectError:   false,
			errorContains: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockGrantManager := pluginCore.NewMockGrantManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager).Maybe()

			enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			uploadID := dataManager.NextUploadID()

			if !test.expectError {
				mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.Anything, models.GrantTypeUpload, test.bytes).Return(nil)
				mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uploadID, test.bytes, test.ip).Return(nil)
			}

			err := enforcer.RecordUpload(ctx, userID, uploadID, test.bytes, test.ip)

			if test.expectError {
				assert.Error(t, err)
				if test.errorContains != "" {
					assert.Contains(t, err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			dataManager.Cleanup()
		})
	}
}

// TestAllowancePolicyEnforcer_RecordStorageChange_BoundaryConditions tests boundary conditions for recording storage changes
func TestAllowancePolicyEnforcer_RecordStorageChange_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint
		uploadID      uint
		bytes         int64
		ip            string
		expectError   bool
		errorContains string
	}{
		{
			name:          "Zero bytes should fail validation",
			userID:        1,
			uploadID:      100,
			bytes:         0,
			ip:            "192.168.1.1",
			expectError:   true,
			errorContains: "bytes must be greater than 0",
		},
		{
			name:          "Positive bytes should succeed",
			userID:        2,
			uploadID:      100,
			bytes:         1000,
			ip:            "192.168.1.1",
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "Negative bytes should succeed (storage removal)",
			userID:        3,
			uploadID:      100,
			bytes:         -500,
			ip:            "192.168.1.1",
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "Maximum positive bytes should succeed",
			userID:        4,
			uploadID:      100,
			bytes:         math.MaxInt64,
			ip:            "192.168.1.1",
			expectError:   false,
			errorContains: "",
		},
		{
			name:          "Minimum negative bytes should succeed",
			userID:        5,
			uploadID:      100,
			bytes:         math.MinInt64,
			ip:            "192.168.1.1",
			expectError:   false,
			errorContains: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockGrantManager := pluginCore.NewMockGrantManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager).Maybe()

			enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			uploadID := dataManager.NextUploadID()

			if !test.expectError {
				if test.bytes > 0 {
					mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.Anything, models.GrantTypeStorage, uint64(test.bytes)).Return(nil)
				} else {
					mockUsageManager.EXPECT().RecordUserUsageDetail(mock.Anything, mock.Anything, (*gorm.DB)(nil)).Return(nil)
				}
				mockUsageManager.EXPECT().RecordStorageChange(mock.Anything, userID, uploadID, test.bytes, test.ip).Return(nil)
			}

			err := enforcer.RecordStorageChange(ctx, userID, uploadID, test.bytes, test.ip)

			if test.expectError {
				assert.Error(t, err)
				if test.errorContains != "" {
					assert.Contains(t, err.Error(), test.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}

			dataManager.Cleanup()
		})
	}
}

// TestUnlimitedPolicyEnforcer_BoundaryConditions tests boundary conditions for unlimited policy
func TestUnlimitedPolicyEnforcer_BoundaryConditions(t *testing.T) {
	tests := []struct {
		name            string
		userID          uint
		requestBytes    uint64
		expectedAllowed bool
		expectedReason  models.QuotaCheckReason
	}{
		{
			name:            "Zero bytes should fail validation",
			userID:          1,
			requestBytes:    0,
			expectedAllowed: false,
			expectedReason:  "",
		},
		{
			name:            "Small request should be allowed",
			userID:          2,
			requestBytes:    100,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Large request should be allowed",
			userID:          3,
			requestBytes:    1000000,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
		{
			name:            "Maximum uint64 should be allowed",
			userID:          4,
			requestBytes:    math.MaxUint64,
			expectedAllowed: true,
			expectedReason:  models.QuotaCheckReasonOK,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

			userID := dataManager.NextUserID()
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyUnlimited,
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, test.requestBytes)
			if test.requestBytes == 0 {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "bytes must be greater than 0")
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedAllowed, result.Allowed)
				assert.Equal(t, test.expectedReason, result.Reason)
			}

			dataManager.Cleanup()
		})
	}
}

// TestHardLimitsPolicyEnforcer_ErrorHandling tests error handling for hard limits policy
func TestHardLimitsPolicyEnforcer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name         string
		setupMock    func(*pluginCore.MockQuotaService, coreTesting.TestContext)
		config       *models.UserQuotaConfig
		requestBytes uint64
		expectError  bool
	}{
		{
			name: "GetUsageManager error should propagate",
			setupMock: func(mqs *pluginCore.MockQuotaService, ctx coreTesting.TestContext) {
				mockUsageManager := pluginCore.NewMockUsageManager(t)
				mqs.EXPECT().GetUsageManager().Return(mockUsageManager)
				mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
				mqs.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
				mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()
				mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, uint(1), models.UsageTypeUpload, mock.Anything).Return(uint64(0), time.Now(), time.Now(), errors.New("usage fetch failed"))
			},
			config:       func() *models.UserQuotaConfig {
			dur := int64(86400)
			return &models.UserQuotaConfig{
				UserID:            1,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadLimitBytes:  uint64(1000),
				WindowType:        models.WindowTypeCalendarDay,
				WindowDuration:    &dur,
			}
		}(),
			requestBytes: 100,
			expectError:  true,
		},
		{
			name: "Nil config should fail",
			setupMock: func(mqs *pluginCore.MockQuotaService, ctx coreTesting.TestContext) {
				mqs.EXPECT().GetUsageManager().Return(nil)
			},
			config:       nil,
			requestBytes: 100,
			expectError:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := coreTesting.NewTestContext(t)
			dataManager := testdata.NewTestDataManager(ctx)
			mockQuotaService := pluginCore.NewMockQuotaService(t)

			test.setupMock(mockQuotaService, ctx)
			enforcer := NewHardLimitsPolicyEnforcer(ctx, mockQuotaService)

			_, err := enforcer.CheckUploadQuota(ctx.GetContext(), test.config, test.requestBytes)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			dataManager.Cleanup()
		})
	}
}
