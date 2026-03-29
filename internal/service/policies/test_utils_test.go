package policies

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// MockSetup holds common mock components for policy enforcer tests
type MockSetup struct {
	QuotaService     *pluginCore.MockQuotaService
	UsageManager     *pluginCore.MockUsageManager
	UsageAggregator  *pluginCore.MockUsageAggregator
	GrantManager     *pluginCore.MockGrantManager
	QuotaPlanManager *pluginCore.MockQuotaPlanManager
}

// testUserLimits represents test user quota limits (lowercase version)
type testUserLimits struct {
	storageLimit       *int64
	uploadDailyLimit   *int64
	uploadTotalLimit   *int64
	downloadDailyLimit *int64
	downloadTotalLimit *int64
	storageThreshold   *int64
	uploadThreshold    *int64
	downloadThreshold  *int64
	quotaPlanID        *uint64
}

// testPlanLimits represents quota limits for a test quota plan (lowercase version)
type testPlanLimits struct {
	storageLimit       int64
	uploadDailyLimit   int64
	downloadDailyLimit int64
	uploadTotalLimit   int64
	downloadTotalLimit int64
	storageThreshold   *int64
	uploadThreshold    *int64
	downloadThreshold  *int64
}

// SetupMocks creates and configures common mocks for policy enforcer tests
func SetupMocks(t *testing.T) *MockSetup {
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockGrantManager := pluginCore.NewMockGrantManager(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)

	// Setup base mock expectations
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager).Maybe()

	return &MockSetup{
		QuotaService:     mockQuotaService,
		UsageManager:     mockUsageManager,
		GrantManager:     mockGrantManager,
		QuotaPlanManager: mockQuotaPlanManager,
		UsageAggregator:  mockUsageAggregator,
	}
}

// createTestUser creates a test user in the database (lowercase version)
func createTestUser(t *testing.T, ctx coreTesting.TestContext, userID uint, policy models.EnforcementPolicy, limits *testUserLimits) *models.UserQuotaConfig {
	cfg := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: policy,
	}

	if limits != nil {
		cfg.StorageLimit = limits.storageLimit
		cfg.UploadDailyLimit = limits.uploadDailyLimit
		cfg.DownloadDailyLimit = limits.downloadDailyLimit
		cfg.UploadTotalLimit = limits.uploadTotalLimit
		cfg.DownloadTotalLimit = limits.downloadTotalLimit
		cfg.StorageThreshold = limits.storageThreshold
		cfg.UploadThreshold = limits.uploadThreshold
		cfg.DownloadThreshold = limits.downloadThreshold
		cfg.QuotaPlanID = limits.quotaPlanID
	}

	err := ctx.DB().Create(cfg).Error
	require.NoError(t, err, "Failed to create user quota config")
	return cfg
}

// CreateTestQuotaPlan creates a test QuotaPlan with the specified parameters
func CreateTestQuotaPlan(id uint, storageLimit, uploadDailyLimit, downloadDailyLimit, uploadTotalLimit, downloadTotalLimit int64, isActive *bool) *models.QuotaPlan {
	return &models.QuotaPlan{
		Model:              gorm.Model{ID: uint(id)},
		StorageLimit:       storageLimit,
		UploadDailyLimit:   uploadDailyLimit,
		DownloadDailyLimit: downloadDailyLimit,
		UploadTotalLimit:   uploadTotalLimit,
		DownloadTotalLimit: downloadTotalLimit,
		IsActive:           isActive,
	}
}

// createTestQuotaPlan creates a test QuotaPlan with the specified parameters (lowercase version)
func createTestQuotaPlan(t *testing.T, ctx coreTesting.TestContext, name string, isDefault bool, limits *testPlanLimits) *models.QuotaPlan {
	plan := &models.QuotaPlan{
		Name:               name,
		Description:        "Test plan",
		StorageLimit:       limits.storageLimit,
		UploadDailyLimit:   limits.uploadDailyLimit,
		DownloadDailyLimit: limits.downloadDailyLimit,
		UploadTotalLimit:   limits.uploadTotalLimit,
		DownloadTotalLimit: limits.downloadTotalLimit,
		StorageThreshold:   limits.storageThreshold,
		UploadThreshold:    limits.uploadThreshold,
		DownloadThreshold:  limits.downloadThreshold,
		IsDefault:          isDefault,
		IsActive:           lo.ToPtr(true),
	}

	err := ctx.DB().Create(plan).Error
	require.NoError(t, err, "Failed to create quota plan")
	return plan
}

// createTestUsageRecord creates a test usage record in the database using TestDataManager
func createTestUsageRecord(t *testing.T, ctx coreTesting.TestContext, userID uint, usageType models.UsageType, bytes uint64) {
	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  1,
		Type:      usageType,
		Bytes:     bytes,
		IP:        models.IPAddr("192.168.1.1"),
		Timestamp: time.Now(),
	}

	err := ctx.DB().Create(detail).Error
	require.NoError(t, err, "Failed to create usage record")

	// Also create/update the daily quota record so getCurrentUsage can find aggregated data
	today := time.Now().UTC().Truncate(24 * time.Hour)
	var dailyQuota models.UserQuota
	err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error

	if err == gorm.ErrRecordNotFound {
		// Create new daily quota record
		dailyQuota = models.UserQuota{
			UserID: userID,
			Date:   today,
		}

		// Set the appropriate field based on usage type
		switch usageType {
		case models.UsageTypeUpload:
			dailyQuota.BytesUploaded = bytes
		case models.UsageTypeDownload:
			dailyQuota.BytesDownloaded = bytes
		case models.UsageTypeStorageAdd:
			dailyQuota.BytesStored = bytes
		case models.UsageTypeStorageRemove:
			// For storage removal, we subtract bytes from the stored amount
			if bytes <= dailyQuota.BytesStored {
				dailyQuota.BytesStored -= bytes
			} else {
				dailyQuota.BytesStored = 0
			}
		}

		err = ctx.DB().Create(&dailyQuota).Error
		require.NoError(t, err, "Failed to create daily quota record")
	} else if err != nil {
		require.NoError(t, err, "Failed to query daily quota record")
	} else {
		// Update existing daily quota record
		switch usageType {
		case models.UsageTypeUpload:
			dailyQuota.BytesUploaded += bytes
		case models.UsageTypeDownload:
			dailyQuota.BytesDownloaded += bytes
		case models.UsageTypeStorageAdd:
			dailyQuota.BytesStored += bytes
		case models.UsageTypeStorageRemove:
			// For storage removal, we subtract bytes from the stored amount
			if bytes <= dailyQuota.BytesStored {
				dailyQuota.BytesStored -= bytes
			} else {
				dailyQuota.BytesStored = 0
			}
		}

		err = ctx.DB().Save(&dailyQuota).Error
		require.NoError(t, err, "Failed to update daily quota record")
	}
}

// CreateTestAllowanceGrant creates a test AllowanceGrant with the specified parameters
func CreateTestAllowanceGrant(userID uint, grantType models.GrantType, source models.GrantSource, bytes, bytesUsed, bytesRemaining uint64, isActive bool) *models.AllowanceGrant {
	return &models.AllowanceGrant{
		UserID:         userID,
		Type:           grantType,
		Source:         source,
		Bytes:          bytes,
		BytesUsed:      bytesUsed,
		BytesRemaining: bytesRemaining,
		IsActive:       isActive,
	}
}

// CreateTestUsage creates a test Usage with the specified parameters
func CreateTestUsage(userID uint, uploaded, downloaded, stored uint64) *pluginCore.Usage {
	return &pluginCore.Usage{
		UserID:          userID,
		BytesUploaded:   uploaded,
		BytesDownloaded: downloaded,
		BytesStored:     stored,
		LastUpdated:     time.Now(),
	}
}

// CreateTestUserUsageDetail creates a test UserUsageDetail with the specified parameters
func CreateTestUserUsageDetail(userID uint, uploadID uint, usageType models.UsageType, bytes uint64, ip string, timestamp time.Time) *models.UserUsageDetail {
	return &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      usageType,
		Bytes:     bytes,
		IP:        models.IPAddr(ip),
		Timestamp: timestamp,
	}
}

// CreateTestUsagePoint creates a test UsagePoint with the specified parameters
func CreateTestUsagePoint(date time.Time, bytes uint64, usageType models.UsageType) *pluginCore.UsagePoint {
	return &pluginCore.UsagePoint{
		Date:  date,
		Bytes: bytes,
		Type:  usageType,
	}
}

// AssertQuotaCheckResult asserts that a quota check result matches expected values
func AssertQuotaCheckResult(t *testing.T, result *pluginCore.QuotaCheckResult, allowed bool, reason models.QuotaCheckReason, policy models.EnforcementPolicy) {
	t.Helper()
	assert.Equal(t, allowed, result.Allowed)
	assert.Equal(t, reason, result.Reason)
	assert.Equal(t, policy, result.Details.Policy)
}

// AssertQuotaCheckResultWithDetails asserts that a quota check result matches expected values including details
func AssertQuotaCheckResultWithDetails(t *testing.T, result *pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason models.QuotaCheckReason, expectedPolicy models.EnforcementPolicy, expectedCurrentUsage, expectedLimit uint64) {
	t.Helper()
	AssertQuotaCheckResult(t, result, expectedAllowed, expectedReason, expectedPolicy)
	assert.Equal(t, expectedCurrentUsage, result.Details.CurrentUsage, "Quota check result current usage mismatch")
	assert.NotNil(t, result.Details.Limit, "Quota check result limit should not be nil")
	assert.Equal(t, expectedLimit, *result.Details.Limit, "Quota check result limit mismatch")
}

// AssertQuotaCheckResultWithAllowance asserts that a quota check result matches expected values for allowance policy
func AssertQuotaCheckResultWithAllowance(t *testing.T, result *pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason models.QuotaCheckReason, expectedPolicy models.EnforcementPolicy, expectedAllowance, expectedAllowanceUsed uint64) {
	t.Helper()
	AssertQuotaCheckResult(t, result, expectedAllowed, expectedReason, expectedPolicy)
	assert.NotNil(t, result.Details.Allowance, "Quota check result allowance should not be nil")
	assert.Equal(t, expectedAllowance, *result.Details.Allowance, "Quota check result allowance mismatch")
	assert.NotNil(t, result.Details.AllowanceUsed, "Quota check result allowance used should not be nil")
	assert.Equal(t, expectedAllowanceUsed, *result.Details.AllowanceUsed, "Quota check result allowance used mismatch")
}

// AssertQuotaCheckResultWithThreshold asserts that a quota check result matches expected values for threshold policy
func AssertQuotaCheckResultWithThreshold(t *testing.T, result *pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason models.QuotaCheckReason, expectedPolicy models.EnforcementPolicy, expectedCurrentUsage, expectedThreshold, expectedLimit uint64) {
	t.Helper()
	AssertQuotaCheckResult(t, result, expectedAllowed, expectedReason, expectedPolicy)
	assert.Equal(t, expectedCurrentUsage, result.Details.CurrentUsage, "Quota check result current usage mismatch")
	assert.NotNil(t, result.Details.Threshold, "Quota check result threshold should not be nil")
	assert.Equal(t, expectedThreshold, *result.Details.Threshold, "Quota check result threshold mismatch")
	assert.NotNil(t, result.Details.Limit, "Quota check result limit should not be nil")
	assert.Equal(t, expectedLimit, *result.Details.Limit, "Quota check result limit mismatch")
}

// AssertUsageRecorded asserts that usage was recorded with expected values
func AssertUsageRecorded(t *testing.T, usageManager *pluginCore.MockUsageManager, userID, uploadID uint, bytes uint64, ip string, usageType models.UsageType) {
	t.Helper()
	switch usageType {
	case models.UsageTypeUpload:
		usageManager.AssertCalled(t, "RecordUpload", userID, uploadID, bytes, ip)
	case models.UsageTypeDownload:
		usageManager.AssertCalled(t, "RecordDownload", userID, uploadID, bytes, ip)
	case models.UsageTypeStorageAdd:
		usageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, int64(bytes), ip)
	case models.UsageTypeStorageRemove:
		usageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, -int64(bytes), ip)
	}
}

// createMockGrantManager creates a mock grant manager for testing
func createMockGrantManager(t *testing.T) *pluginCore.MockGrantManager {
	return pluginCore.NewMockGrantManager(t)
}

// RunBoundaryConditionTests executes common boundary condition tests for quota policies
func RunBoundaryConditionTests(t *testing.T, ctx context.Context, enforcer pluginCore.PolicyEnforcer, policy models.EnforcementPolicy, usageAgg *pluginCore.MockUsageAggregator) {
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
			requestBytes:    math.MaxUint64, // Maximum uint64
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
			config := &models.UserQuotaConfig{
				UserID:            test.userID,
				EnforcementPolicy: policy,
				UploadDailyLimit:  lo.ToPtr(test.dailyLimit),
				UploadTotalLimit:  lo.ToPtr(test.totalLimit),
			}

			// Stub usage aggregation calls if MockUsageAggregator is provided
			if usageAgg != nil {
				usageAgg.EXPECT().GetAggregatedUsageByType(ctx, test.userID, models.UsageTypeUpload).Return(test.currentUsage, nil).Maybe()
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, test.requestBytes)
			require.NoError(t, err)
			assert.Equal(t, test.expectedAllowed, result.Allowed)
			assert.Equal(t, test.expectedReason, result.Reason)
		})
	}
}

// RunInvalidLimitValueTests tests invalid limit values handling
func RunInvalidLimitValueTests(t *testing.T, ctx context.Context, enforcer pluginCore.PolicyEnforcer, policy models.EnforcementPolicy) {
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
			expectedError: "invalid",
		},
		{
			name:          "Invalid total limit",
			dailyLimit:    lo.ToPtr(int64(1000)),
			totalLimit:    lo.ToPtr(int64(-2)),
			expectedError: "invalid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &models.UserQuotaConfig{
				UserID:            2,
				EnforcementPolicy: policy,
				UploadDailyLimit:  test.dailyLimit,
				UploadTotalLimit:  test.totalLimit,
			}

			result, err := enforcer.CheckUploadQuota(ctx, config, uint64(500))
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedError)
			assert.Equal(t, models.QuotaCheckReason(""), result.Reason)
		})
	}
}

// RunUsageRecordingTests tests usage recording functionality
func RunUsageRecordingTests(t *testing.T, ctx context.Context, enforcer pluginCore.PolicyEnforcer, usageManager *pluginCore.MockUsageManager, userID, uploadID uint, bytes uint64, ip string) {
	// Default unlimited config to allow recording without quota failures
	usageManager.
		On("GetUserQuotaConfig", ctx, userID).
		Return(&models.UserQuotaConfig{
			UserID:             userID,
			EnforcementPolicy:  models.EnforcementPolicyHardLimits,
			StorageLimit:       lo.ToPtr(int64(-1)),
			UploadDailyLimit:   lo.ToPtr(int64(-1)),
			DownloadDailyLimit: lo.ToPtr(int64(-1)),
			UploadTotalLimit:   lo.ToPtr(int64(-1)),
			DownloadTotalLimit: lo.ToPtr(int64(-1)),
		}, nil).
		Maybe()

	tests := []struct {
		name        string
		testFunc    func() error
		assertFunc  func()
		expectedErr bool
	}{
		{
			name: "RecordUpload",
			testFunc: func() error {
				return enforcer.RecordUpload(ctx, userID, uploadID, bytes, ip)
			},
			assertFunc: func() {
				usageManager.AssertCalled(t, "RecordUpload", ctx, userID, uploadID, bytes, ip)
			},
			expectedErr: false,
		},
		{
			name: "RecordDownload",
			testFunc: func() error {
				return enforcer.RecordDownload(ctx, userID, uploadID, bytes, ip)
			},
			assertFunc: func() {
				usageManager.AssertCalled(t, "RecordDownload", ctx, userID, uploadID, bytes, ip)
			},
			expectedErr: false,
		},
		{
			name: "RecordStorageChange",
			testFunc: func() error {
				return enforcer.RecordStorageChange(ctx, userID, uploadID, int64(bytes), ip)
			},
			assertFunc: func() {
				usageManager.AssertCalled(t, "RecordStorageChange", ctx, userID, uploadID, int64(bytes), ip)
			},
			expectedErr: false,
		},
		{
			name: "RecordStorageRemove",
			testFunc: func() error {
				return enforcer.RecordStorageChange(ctx, userID, uploadID, -int64(bytes), ip)
			},
			assertFunc: func() {
				usageManager.AssertCalled(t, "RecordStorageChange", ctx, userID, uploadID, -int64(bytes), ip)
			},
			expectedErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.testFunc()
			if test.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if test.assertFunc != nil {
				test.assertFunc()
			}
		})
	}
}
