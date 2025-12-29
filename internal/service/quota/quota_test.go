package quota

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/migrations"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// Test constants
const (
	testUserID        = 1
	testUploadID      = 100
	testBytesSmall    = 100
	testBytesMedium   = 500
	testBytesLarge    = 1000
	testValidPlanID   = 1
	testInvalidPlanID = 999
)

// testOptions creates test options without mocking the quota service itself
func testOptions() coreTesting.TestContextBuilderOption {
	return coreTesting.CombineOptions(coreTesting.NewMockPluginBuilder(internal.PLUGIN_NAME).
		WithMigrations(core.DBMigration{core.DB_TYPE_SQLITE: migrations.GetSQLite()}).
		WithService(pluginCore.QUOTA_SERVICE, NewQuotaService).BuilderOption(),
		coreTesting.WithServiceConfig(internal.PLUGIN_NAME, pluginCore.QUOTA_SERVICE, &config.QuotaConfig{
			SharedUsagePrecision: 2,
		}),
	)
}

// TestQuotaServiceDefault_NewQuotaService tests the NewQuotaService function
func TestQuotaServiceDefault_NewQuotaService(t *testing.T) {
	service, options, err := NewQuotaService()
	require.NoError(t, err)
	assert.NotNil(t, service)
	assert.NotNil(t, options)
	assert.Equal(t, pluginCore.QUOTA_SERVICE, service.ID())
}

// TestQuotaServiceDefault_RecordUpload_Success tests successful upload recording
func TestQuotaServiceDefault_RecordUpload_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager

		userID := uint(testUserID)
		uploadID := uint(testUploadID)
		bytes := uint64(testBytesMedium)
		ip := "192.168.1.1"

		mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

		err := quotaService.RecordUpload(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)
	}, testOptions())
}

// TestQuotaServiceDefault_RecordUpload_Error tests upload recording error handling
func TestQuotaServiceDefault_RecordUpload_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager

		userID := uint(testUserID)
		uploadID := uint(testUploadID)
		bytes := uint64(testBytesMedium)
		ip := "192.168.1.1"

		expectedErr := fmt.Errorf("upload recording failed")
		mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uploadID, bytes, ip).Return(expectedErr)

		err := quotaService.RecordUpload(ctx, userID, uploadID, bytes, ip)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
	}, testOptions())
}

// TestQuotaServiceDefault_RecordUpload_Uninitialized tests upload recording when usage manager is not initialized
func TestQuotaServiceDefault_RecordUpload_Uninitialized(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		quotaService.(*QuotaServiceDefault).usageManager = nil

		userID := uint(testUserID)
		uploadID := uint(testUploadID)
		bytes := uint64(testBytesMedium)
		ip := "192.168.1.1"

		err := quotaService.RecordUpload(ctx, userID, uploadID, bytes, ip)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "usage manager not initialized")
	}, testOptions())
}

// TestQuotaServiceDefault_RecordDownload_Success tests successful download recording
func TestQuotaServiceDefault_RecordDownload_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager

		userID := uint(testUserID)
		uploadID := uint(testUploadID)
		bytes := uint64(testBytesMedium)
		ip := "192.168.1.1"

		mockUsageManager.EXPECT().RecordDownload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

		err := quotaService.RecordDownload(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)
	}, testOptions())
}

// TestQuotaServiceDefault_RecordStorageChange_Success tests successful storage change recording
func TestQuotaServiceDefault_RecordStorageChange_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager

		userID := uint(testUserID)
		uploadID := uint(testUploadID)
		bytes := int64(testBytesMedium)
		ip := "192.168.1.1"

		mockUsageManager.EXPECT().RecordStorageChange(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

		err := quotaService.RecordStorageChange(ctx, userID, uploadID, bytes, ip)
		require.NoError(t, err)
	}, testOptions())
}

// TestQuotaServiceDefault_CheckUploadQuota_Success tests successful upload quota checking
func TestQuotaServiceDefault_CheckUploadQuota_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		quotaService.(*QuotaServiceDefault).configManager = mockConfigManager

		userID := uint(testUserID)
		requestedBytes := uint64(testBytesMedium)

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		expectedResult := pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  "within quota limits",
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil)
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil)
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, userConfig, requestedBytes).Return(expectedResult, nil)

		result, err := quotaService.CheckUploadQuota(ctx, userID, requestedBytes)
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)
	}, testOptions())
}

// TestQuotaServiceDefault_GetCurrentUsage_Success tests successful current usage retrieval
func TestQuotaServiceDefault_GetCurrentUsage_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager

		userID := uint(testUserID)

		expectedUsage := &pluginCore.Usage{
			UserID:          userID,
			BytesUploaded:   testBytesSmall,
			BytesDownloaded: testBytesMedium,
			BytesStored:     testBytesLarge,
			LastUpdated:     time.Now().UTC(),
		}

		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(expectedUsage, nil)

		usage, err := quotaService.GetCurrentUsage(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedUsage, usage)
	}, testOptions())
}

// TestQuotaServiceDefault_GetUsageHistory_Success tests successful usage history retrieval
func TestQuotaServiceDefault_GetUsageHistory_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager

		userID := uint(testUserID)
		period := 7
		usageType := pluginCore.UsageTypeUpload

		expectedPoints := []*pluginCore.UsagePoint{
			{
				Date:   time.Now().UTC().AddDate(0, 0, -2),
				Bytes:  testBytesSmall,
				Type:   usageType,
				UserID: userID,
			},
			{
				Date:   time.Now().UTC().AddDate(0, 0, -1),
				Bytes:  testBytesMedium,
				Type:   usageType,
				UserID: userID,
			},
		}

		mockUsageManager.EXPECT().GetUsageHistory(mock.Anything, userID, period, usageType).Return(expectedPoints, nil)

		points, err := quotaService.GetUsageHistory(ctx, userID, period, usageType)
		require.NoError(t, err)
		assert.Equal(t, expectedPoints, points)
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_Success tests successful quota plan creation
func TestQuotaServiceDefault_CreateQuotaPlan_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		plan := &pluginModels.QuotaPlan{
			Name:        "test_plan",
			Description: "Test quota plan",
		}

		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)

		// Verify plan was created in DB
		var savedPlan pluginModels.QuotaPlan
		err = ctx.DB().Where("name = ?", "test_plan").First(&savedPlan).Error
		require.NoError(t, err)
		assert.Equal(t, "test_plan", savedPlan.Name)
		assert.Equal(t, "Test quota plan", savedPlan.Description)
	}, testOptions())
}

// TestQuotaServiceDefault_GetQuotaPlan_Success tests successful quota plan retrieval
func TestQuotaServiceDefault_GetQuotaPlan_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Create a plan first
		plan := &pluginModels.QuotaPlan{
			Name:        "test_plan",
			Description: "Test quota plan",
		}
		err := ctx.DB().Create(plan).Error
		require.NoError(t, err)

		retrievedPlan, err := quotaService.GetQuotaPlan(ctx, plan.ID)
		require.NoError(t, err)
		assert.Equal(t, plan.ID, retrievedPlan.ID)
		assert.Equal(t, plan.Name, retrievedPlan.Name)
		assert.Equal(t, plan.Description, retrievedPlan.Description)
	}, testOptions())
}
