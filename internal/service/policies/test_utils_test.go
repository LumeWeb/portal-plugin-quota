package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// testUserLimits represents quota limits for a test user
type testUserLimits struct {
	StorageLimit       *int64
	UploadDailyLimit   *int64
	UploadTotalLimit   *int64
	DownloadDailyLimit *int64
	DownloadTotalLimit *int64
	StorageThreshold   *int64
	UploadThreshold    *int64
	DownloadThreshold  *int64
	QuotaPlanID        *uint64
}

// testPlanLimits represents quota limits for a test quota plan
type testPlanLimits struct {
	StorageLimit       int64
	UploadDailyLimit   int64
	DownloadDailyLimit int64
	UploadTotalLimit   int64
	DownloadTotalLimit int64
	StorageThreshold   *int64
	UploadThreshold    *int64
	DownloadThreshold  *int64
}

// createTestUser creates a test user in the database
func createTestUser(t *testing.T, ctx coreTesting.TestContext, userID uint, policy models.EnforcementPolicy, limits *testUserLimits) *models.UserQuotaConfig {
	// Create user quota config
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: policy,
	}

	if limits.StorageLimit != nil {
		config.StorageLimit = limits.StorageLimit
	}
	if limits.UploadDailyLimit != nil {
		config.UploadDailyLimit = limits.UploadDailyLimit
	}
	if limits.DownloadDailyLimit != nil {
		config.DownloadDailyLimit = limits.DownloadDailyLimit
	}
	if limits.UploadTotalLimit != nil {
		config.UploadTotalLimit = limits.UploadTotalLimit
	}
	if limits.DownloadTotalLimit != nil {
		config.DownloadTotalLimit = limits.DownloadTotalLimit
	}
	if limits.StorageThreshold != nil {
		config.StorageThreshold = limits.StorageThreshold
	}
	if limits.UploadThreshold != nil {
		config.UploadThreshold = limits.UploadThreshold
	}
	if limits.DownloadThreshold != nil {
		config.DownloadThreshold = limits.DownloadThreshold
	}
	if limits.QuotaPlanID != nil {
		config.QuotaPlanID = limits.QuotaPlanID
	}

	err := ctx.DB().Create(config).Error
	require.NoError(t, err, "Failed to create user quota config")
	return config
}

// createTestQuotaPlan creates a test quota plan in the database
func createTestQuotaPlan(t *testing.T, ctx coreTesting.TestContext, name string, isDefault bool, limits *testPlanLimits) *models.QuotaPlan {
	plan := &models.QuotaPlan{
		Name:               name,
		Description:        "Test plan",
		StorageLimit:       limits.StorageLimit,
		UploadDailyLimit:   limits.UploadDailyLimit,
		DownloadDailyLimit: limits.DownloadDailyLimit,
		UploadTotalLimit:   limits.UploadTotalLimit,
		DownloadTotalLimit: limits.DownloadTotalLimit,
		StorageThreshold:   limits.StorageThreshold,
		UploadThreshold:    limits.UploadThreshold,
		DownloadThreshold:  limits.DownloadThreshold,
		IsDefault:          isDefault,
		IsActive:           true,
	}

	err := ctx.DB().Create(plan).Error
	require.NoError(t, err, "Failed to create quota plan")
	return plan
}

// createTestAllowanceGrant creates a test allowance grant in the database
func createTestAllowanceGrant(t *testing.T, ctx coreTesting.TestContext, userID uint, grantType models.GrantType, bytes uint64) *models.AllowanceGrant {
	grant := &models.AllowanceGrant{
		UserID:         userID,
		Type:           grantType,
		Source:         models.GrantSourcePAYGAddon,
		Bytes:          bytes,
		BytesUsed:      0,
		BytesRemaining: bytes,
		IsActive:       true,
	}

	err := ctx.DB().Create(grant).Error
	require.NoError(t, err, "Failed to create allowance grant")
	return grant
}

// createTestUsageRecord creates a test usage record in the database
func createTestUsageRecord(t *testing.T, ctx coreTesting.TestContext, userID uint, usageType models.UsageType, bytes uint64) {
	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  1,
		Type:      usageType,
		Bytes:     bytes,
		IP:        "192.168.1.1",
		Timestamp: time.Now(),
	}

	err := ctx.DB().Create(detail).Error
	require.NoError(t, err, "Failed to create usage record")

	// Also create/update the daily quota record so getCurrentUsage can find aggregated data
	today := time.Now().Truncate(24 * time.Hour)
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
		}

		err = ctx.DB().Save(&dailyQuota).Error
		require.NoError(t, err, "Failed to update daily quota record")
	}
}

// assertQuotaCheckResult asserts that a quota check result matches expected values
func assertQuotaCheckResult(t *testing.T, result pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason pluginCore.QuotaCheckReason, expectedPolicy pluginCore.EnforcementPolicy) {
	assert.Equal(t, expectedAllowed, result.Allowed, "Quota check result allowed status mismatch")
	assert.Equal(t, expectedReason, result.Reason, "Quota check result reason mismatch")
	assert.Equal(t, expectedPolicy, result.Details.Policy, "Quota check result policy mismatch")
}

// assertQuotaCheckResultWithDetails asserts that a quota check result matches expected values including details
func assertQuotaCheckResultWithDetails(t *testing.T, result pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason pluginCore.QuotaCheckReason, expectedPolicy pluginCore.EnforcementPolicy, expectedCurrentUsage, expectedLimit uint64) {
	assertQuotaCheckResult(t, result, expectedAllowed, expectedReason, expectedPolicy)
	assert.Equal(t, expectedCurrentUsage, result.Details.CurrentUsage, "Quota check result current usage mismatch")
	assert.NotNil(t, result.Details.Limit, "Quota check result limit should not be nil")
	assert.Equal(t, expectedLimit, *result.Details.Limit, "Quota check result limit mismatch")
}

// assertQuotaCheckResultWithAllowance asserts that a quota check result matches expected values for allowance policy
func assertQuotaCheckResultWithAllowance(t *testing.T, result pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason pluginCore.QuotaCheckReason, expectedPolicy pluginCore.EnforcementPolicy, expectedAllowance, expectedAllowanceUsed uint64) {
	assertQuotaCheckResult(t, result, expectedAllowed, expectedReason, expectedPolicy)
	assert.NotNil(t, result.Details.Allowance, "Quota check result allowance should not be nil")
	assert.Equal(t, expectedAllowance, *result.Details.Allowance, "Quota check result allowance mismatch")
	assert.NotNil(t, result.Details.AllowanceUsed, "Quota check result allowance used should not be nil")
	assert.Equal(t, expectedAllowanceUsed, *result.Details.AllowanceUsed, "Quota check result allowance used mismatch")
}

// assertQuotaCheckResultWithThreshold asserts that a quota check result matches expected values for threshold policy
func assertQuotaCheckResultWithThreshold(t *testing.T, result pluginCore.QuotaCheckResult, expectedAllowed bool, expectedReason pluginCore.QuotaCheckReason, expectedPolicy pluginCore.EnforcementPolicy, expectedCurrentUsage, expectedThreshold, expectedLimit uint64) {
	assertQuotaCheckResult(t, result, expectedAllowed, expectedReason, expectedPolicy)
	assert.Equal(t, expectedCurrentUsage, result.Details.CurrentUsage, "Quota check result current usage mismatch")
	assert.NotNil(t, result.Details.Threshold, "Quota check result threshold should not be nil")
	assert.Equal(t, expectedThreshold, *result.Details.Threshold, "Quota check result threshold mismatch")
	assert.NotNil(t, result.Details.Limit, "Quota check result limit should not be nil")
	assert.Equal(t, expectedLimit, *result.Details.Limit, "Quota check result limit mismatch")
}

// createMockGrantManager creates a mock grant manager for testing
func createMockGrantManager(t *testing.T) *pluginCore.MockGrantManager {
	return pluginCore.NewMockGrantManager(t)
}
