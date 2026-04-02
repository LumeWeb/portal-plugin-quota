package policies

import (
	"errors"
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

// newHardLimitsTest creates a test harness with all required mocks for hard limits policy tests
func newHardLimitsTest(t *testing.T) (coreTesting.TestContext, *pluginCore.MockQuotaService, *pluginCore.MockUsageManager, *pluginCore.MockQuotaPlanManager, *pluginCore.MockReservationManager, *HardLimitsPolicyEnforcer) {
	ctx, _ := coreTesting.NewTestContext(t)
	qs := pluginCore.NewMockQuotaService(t)
	um := pluginCore.NewMockUsageManager(t)
	qpm := pluginCore.NewMockQuotaPlanManager(t)
	rm := pluginCore.NewMockReservationManager(t)

	qs.EXPECT().GetUsageManager().Return(um).Maybe()
	qs.EXPECT().GetQuotaPlanManager().Return(qpm).Maybe()
	qs.EXPECT().GetReservationManager().Return(rm).Maybe()

	enforcer := NewHardLimitsPolicyEnforcer(ctx, qs)
	return ctx, qs, um, qpm, rm, enforcer
}

// TestHardLimitsPolicyEnforcer_CheckUploadQuota_NilConfig_Unit_Error tests upload with nil config
func TestHardLimitsPolicyEnforcer_CheckUploadQuota_NilConfig_Unit_Error(t *testing.T) {
	ctx, _, um, _, rm, enforcer := newHardLimitsTest(t)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()
	um.EXPECT().RecordUpload(ctx, uint(0), uint(0), uint64(0), "").Return(nil).Maybe()

	result, err := enforcer.CheckUploadQuota(ctx, nil, uint64(500))
	assert.Error(t, err)
	assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)
}

// TestHardLimitsPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Unit_Allowed tests the CheckDownloadQuota method with mocks - within daily limit case
func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_WithinDailyLimit_Unit_Allowed(t *testing.T) {
	ctx, _, um, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)

	// Use fixed test user ID
	userID := dataManager.NextUserID()
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:             userID,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadLimitBytes: uint64(2000),
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	// Mock quota plan manager calls
	qpm.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, userID, models.UsageTypeDownload).Return(int64(0))

	// Mock usage aggregator to return window-based usage
	now := time.Now()
	windowStart := now.Add(-86400 * time.Second)
	um.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeDownload, mock.Anything).Return(uint64(500), windowStart, now, nil)

	result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(1000))
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Unit_Blocked tests the CheckDownloadQuota method with mocks - exceeding daily limit case
func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_ExceedingDailyLimit_Unit_Blocked(t *testing.T) {
	// Setup all mocks
	ctx, _, um, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)

	// Generate unique user ID per test
	userID := dataManager.NextUserID()

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:             userID,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadLimitBytes: uint64(2000),
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	// Mock default quota plan lookup to return not found
	qpm.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, userID, models.UsageTypeDownload).Return(int64(0))

	// Mock usage aggregator to return window-based usage
	now := time.Now()
	windowStart := now.Add(-86400 * time.Second)
	um.EXPECT().GetUsageForWindow(mock.Anything, userID, models.UsageTypeDownload, mock.Anything).Return(uint64(1800), windowStart, now, nil)

	result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(300))
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_CheckDownloadQuota_InvalidBytes_Unit_Error tests the CheckDownloadQuota method with mocks - invalid bytes case
func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_InvalidBytes_Unit_Error(t *testing.T) {
	ctx, _, _, _, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	config := &models.UserQuotaConfig{
		UserID:             dataManager.NextUserID(),
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		DownloadLimitBytes: uint64(2000),
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	result, err := enforcer.CheckDownloadQuota(ctx, config, uint64(0))
	assert.Error(t, err)
	assert.Equal(t, models.ErrInvalidBytes, err)
	assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_CheckDownloadQuota_NilConfig_Unit_Error tests the CheckDownloadQuota method with mocks - nil config case
func TestHardLimitsPolicyEnforcer_CheckDownloadQuota_NilConfig_Unit_Error(t *testing.T) {
	ctx, _, _, _, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	result, err := enforcer.CheckDownloadQuota(ctx, nil, uint64(500))
	assert.Error(t, err)
	assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_QuotaPlanWithNilLimits_Unit_Success tests the getEffectiveLimits method with quota plan that has nil limits
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_QuotaPlanWithNilLimits_Unit_Success(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	planID := uint64(1)
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		QuotaPlanID:       &planID,
	}

	// Plan with some zero limits (can't use nil for int64 fields)
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimitBytes:  5000,
		UploadLimitBytes:   1000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 0, // zero means disabled
	}

	qpm.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(5000), limits.StorageLimitConfig.Bytes)
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(1000), limits.UploadLimitConfig.Bytes)
	assert.Nil(t, limits.DownloadLimitConfig) // Should be nil

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_QuotaPlanLimits_Unit_Success tests the getEffectiveLimits method with quota plan limits
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_QuotaPlanLimits_Unit_Success(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	planID := uint64(1)
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		QuotaPlanID:       &planID,
	}

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimitBytes:  5000,
		UploadLimitBytes:   1000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 2000,
		IsActive:           lo.ToPtr(true),
	}

	qpm.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(5000), limits.StorageLimitConfig.Bytes)
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(1000), limits.UploadLimitConfig.Bytes)
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(2000), limits.DownloadLimitConfig.Bytes)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_CustomOverridesPlan_Unit_Success tests the getEffectiveLimits method with custom limits overriding plan limits
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_CustomOverridesPlan_Unit_Success(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	planID := uint64(1)
	customStorageLimit := uint64(3000)

	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		StorageLimitBytes: customStorageLimit,
		QuotaPlanID:       &planID,
	}

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimitBytes:  5000,
		UploadLimitBytes:   1000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 2000,
	}

	qpm.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(3000), limits.StorageLimitConfig.Bytes) // Custom value
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(1000), limits.UploadLimitConfig.Bytes) // Plan value
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(2000), limits.DownloadLimitConfig.Bytes) // Plan value

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_DefaultPlan_Unit_Success tests the getEffectiveLimits method with default plan when no custom plan
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_DefaultPlan_Unit_Success(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimitBytes:  5000,
		UploadLimitBytes:   1000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 2000,
	}

	// Mock default quota plan lookup
	qpm.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(plan, nil)

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(5000), limits.StorageLimitConfig.Bytes)
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(1000), limits.UploadLimitConfig.Bytes)
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(2000), limits.DownloadLimitConfig.Bytes)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_InactiveDefaultPlan_Unit_Error tests the getEffectiveLimits method with inactive default plan
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_InactiveDefaultPlan_Unit_Error(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	// Create inactive default plan
	inactivePlan := &models.QuotaPlan{
		Model:     gorm.Model{ID: 2},
		IsDefault: true,
		IsActive:  lo.ToPtr(false),
	}

	qpm.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(inactivePlan, nil)

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.Error(t, err)
	assert.Nil(t, limits)
	assert.Contains(t, err.Error(), "quota plan is inactive")

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_NoLimitsConfigured_Unit_Error tests the getEffectiveLimits method when no limits are configured
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_NoLimitsConfigured_Unit_Error(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	qpm.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, errors.New("not found"))

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.Error(t, err)
	assert.Nil(t, limits)
	assert.Contains(t, err.Error(), "failed to retrieve default quota plan")

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_PlanNotFound_Unit_Error tests the getEffectiveLimits method when quota plan is not found
func TestHardLimitsPolicyEnforcer_ResolveEffectiveLimits_PlanNotFound_Unit_Error(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	planID := uint64(999) // Non-existent plan ID
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		QuotaPlanID:       &planID,
	}

	qpm.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(nil, gorm.ErrRecordNotFound)

	limits, err := enforcer.limitResolver.ResolveEffectiveLimits(ctx, config, models.EnforcementPolicyHardLimits)
	assert.Error(t, err)
	assert.Nil(t, limits)
	assert.Contains(t, err.Error(), "failed to retrieve quota plan")

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_CheckUploadQuota_PlanNotFound_Unit_Error tests upload quota check when plan is not found
func TestHardLimitsPolicyEnforcer_CheckUploadQuota_PlanNotFound_Unit_Error(t *testing.T) {
	ctx, _, _, qpm, rm, enforcer := newHardLimitsTest(t)
	dataManager := testdata.NewTestDataManager(ctx)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()

	userID := dataManager.NextUserID()
	planID := uint64(999) // Non-existent plan ID
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		QuotaPlanID:       &planID,
	}

	// Mock quota plan manager to return not found
	qpm.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(nil, gorm.ErrRecordNotFound)

	result, err := enforcer.CheckUploadQuota(ctx, config, uint64(500))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to retrieve quota plan")
	assert.Equal(t, pluginCore.QuotaCheckReason(""), result.Reason)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_GetDetailedUsage_Unit_Success tests the GetDetailedUsage method with mocks
func TestHardLimitsPolicyEnforcer_GetDetailedUsage_Unit_Success(t *testing.T) {
	ctx, _, um, _, rm, enforcer := newHardLimitsTest(t)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()
	dataManager := testdata.NewTestDataManager(ctx)

	userID := dataManager.NextUserID()
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(time.Hour)

	expectedDetails := []*models.UserUsageDetail{
		{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      models.UsageTypeUpload,
			Bytes:     100,
			IP:        models.IPAddr("192.168.1.1"),
			Timestamp: time.Now(),
		},
		{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      models.UsageTypeDownload,
			Bytes:     200,
			IP:        models.IPAddr("192.168.1.2"),
			Timestamp: time.Now().Add(-30 * time.Minute),
		},
	}

	um.EXPECT().GetDetailedUsage(mock.Anything, userID, start, end).Return(expectedDetails, nil)

	details, err := enforcer.GetDetailedUsage(ctx, userID, start, end)
	assert.NoError(t, err)
	assert.Len(t, details, 2)

	// Verify the records are returned in descending order by timestamp
	assert.True(t, details[0].Timestamp.After(details[1].Timestamp) || details[0].Timestamp.Equal(details[1].Timestamp))

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_GetCurrentUsage_Unit_Success tests the GetCurrentUsage method with mocks
func TestHardLimitsPolicyEnforcer_GetCurrentUsage_Unit_Success(t *testing.T) {
	ctx, _, um, _, rm, enforcer := newHardLimitsTest(t)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()
	dataManager := testdata.NewTestDataManager(ctx)

	userID := dataManager.NextUserID()

	expectedUsage := &pluginCore.Usage{
		UserID:          userID,
		BytesUploaded:   100,
		BytesDownloaded: 200,
		BytesStored:     300,
		LastUpdated:     time.Now(),
	}

	um.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(expectedUsage, nil)

	usage, err := enforcer.GetCurrentUsage(ctx, userID)
	assert.NoError(t, err)
	assert.Equal(t, userID, usage.UserID)
	assert.Equal(t, uint64(100), usage.BytesUploaded)
	assert.Equal(t, uint64(200), usage.BytesDownloaded)
	assert.Equal(t, uint64(300), usage.BytesStored)

	dataManager.Cleanup()
}

// TestHardLimitsPolicyEnforcer_GetUsageHistory_Unit_Success tests the GetUsageHistory method with mocks
func TestHardLimitsPolicyEnforcer_GetUsageHistory_Unit_Success(t *testing.T) {
	ctx, _, um, _, rm, enforcer := newHardLimitsTest(t)
	rm.EXPECT().SumPendingBytesForUser(mock.Anything, mock.AnythingOfType("uint"), mock.AnythingOfType("models.UsageType")).Return(int64(0)).Maybe()
	dataManager := testdata.NewTestDataManager(ctx)

	userID := dataManager.NextUserID()
	period := 7
	usageType := models.UsageTypeUpload

	expectedHistory := []*pluginCore.UsagePoint{
		{
			Date:  time.Now().Add(-24 * time.Hour),
			Bytes: 100,
			Type:  models.UsageTypeUpload,
		},
		{
			Date:  time.Now(),
			Bytes: 200,
			Type:  models.UsageTypeUpload,
		},
	}

	um.EXPECT().GetUsageHistory(mock.Anything, userID, period, usageType).Return(expectedHistory, nil)

	history, err := enforcer.GetUsageHistory(ctx, userID, period, usageType)
	assert.NoError(t, err)
	assert.Len(t, history, 2)

	dataManager.Cleanup()
}
