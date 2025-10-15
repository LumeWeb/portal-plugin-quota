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
	"gorm.io/gorm"
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

		mockUsageManager.On("RecordUpload", userID, uploadID, bytes, ip).Return(nil)

		err := quotaService.RecordUpload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		mockUsageManager.AssertExpectations(t)
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
		mockUsageManager.On("RecordUpload", userID, uploadID, bytes, ip).Return(expectedErr)

		err := quotaService.RecordUpload(userID, uploadID, bytes, ip)
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)

		mockUsageManager.AssertExpectations(t)
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

		err := quotaService.RecordUpload(userID, uploadID, bytes, ip)
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

		mockUsageManager.On("RecordDownload", userID, uploadID, bytes, ip).Return(nil)

		err := quotaService.RecordDownload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		mockUsageManager.AssertExpectations(t)
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

		mockUsageManager.On("RecordStorageChange", userID, uploadID, bytes, ip).Return(nil)

		err := quotaService.RecordStorageChange(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		mockUsageManager.AssertExpectations(t)
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

		mockConfigManager.On("GetUserQuotaConfig", userID).Return(userConfig, nil)
		mockConfigManager.On("GetPolicyEnforcer", userID).Return(mockPolicyEnforcer, nil)
		mockPolicyEnforcer.On("CheckUploadQuota", userConfig, requestedBytes).Return(expectedResult, nil)

		result, err := quotaService.CheckUploadQuota(userID, requestedBytes)
		require.NoError(t, err)
		assert.Equal(t, expectedResult, result)

		mockConfigManager.AssertExpectations(t)
		mockPolicyEnforcer.AssertExpectations(t)
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

		mockUsageManager.On("GetCurrentUsage", userID).Return(expectedUsage, nil)

		usage, err := quotaService.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, expectedUsage, usage)

		mockUsageManager.AssertExpectations(t)
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

		mockUsageManager.On("GetUsageHistory", userID, period, usageType).Return(expectedPoints, nil)

		points, err := quotaService.GetUsageHistory(userID, period, usageType)
		require.NoError(t, err)
		assert.Equal(t, expectedPoints, points)

		mockUsageManager.AssertExpectations(t)
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

		err := quotaService.CreateQuotaPlan(plan)
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

		retrievedPlan, err := quotaService.GetQuotaPlan(plan.ID)
		require.NoError(t, err)
		assert.Equal(t, plan.ID, retrievedPlan.ID)
		assert.Equal(t, plan.Name, retrievedPlan.Name)
		assert.Equal(t, plan.Description, retrievedPlan.Description)
	}, testOptions())
}

// TestQuotaServiceDefault_GetQuotaPlan_NotFound tests quota plan retrieval when plan doesn't exist
func TestQuotaServiceDefault_GetQuotaPlan_NotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		plan, err := quotaService.GetQuotaPlan(testInvalidPlanID)
		assert.Error(t, err)
		assert.Nil(t, plan)
		assert.Contains(t, err.Error(), "quota plan not found")
	}, testOptions())
}

// TestQuotaServiceDefault_AddBonusAllowance_Success tests successful bonus allowance addition
func TestQuotaServiceDefault_AddBonusAllowance_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockGrantManager := pluginCore.NewMockGrantManager(tb)
		quotaService.(*QuotaServiceDefault).grantManager = mockGrantManager

		userID := uint(testUserID)
		storage := uint64(testBytesLarge)
		upload := uint64(testBytesMedium)
		download := uint64(testBytesSmall)

		mockGrantManager.On("CreateAllowanceGrant", userID, mock.AnythingOfType("*models.AllowanceGrant")).Return(nil)

		err := quotaService.AddBonusAllowance(userID, storage, upload, download)
		require.NoError(t, err)

		mockGrantManager.AssertExpectations(t)
	}, testOptions())
}

// TestQuotaServiceDefault_GetAllowanceBalance_Success tests successful allowance balance retrieval
func TestQuotaServiceDefault_GetAllowanceBalance_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		quotaService.(*QuotaServiceDefault).configManager = mockConfigManager

		userID := uint(testUserID)

		grants := []*pluginModels.AllowanceGrant{
			{
				Type:           pluginModels.GrantTypeStorage,
				Bytes:          testBytesLarge,
				BytesUsed:      testBytesSmall,
				BytesRemaining: testBytesLarge - testBytesSmall,
			},
			{
				Type:           pluginModels.GrantTypeUpload,
				Bytes:          testBytesMedium,
				BytesUsed:      testBytesSmall,
				BytesRemaining: testBytesMedium - testBytesSmall,
			},
		}

		expectedBalance := &pluginCore.AllowanceBalance{
			StorageAllowance:  testBytesLarge,
			StorageUsed:       testBytesSmall,
			StorageRemaining:  testBytesLarge - testBytesSmall,
			UploadAllowance:   testBytesMedium,
			UploadUsed:        testBytesSmall,
			UploadRemaining:   testBytesMedium - testBytesSmall,
			DownloadAllowance: 0,
			DownloadUsed:      0,
			DownloadRemaining: 0,
		}

		mockConfigManager.On("GetUserAllowanceGrants", userID).Return(grants, nil)

		balance, err := quotaService.GetAllowanceBalance(userID)
		require.NoError(t, err)
		assert.Equal(t, expectedBalance, balance)

		mockConfigManager.AssertExpectations(t)
	}, testOptions())
}

// TestQuotaServiceDefault_ResetAllowance_Success tests successful allowance reset
func TestQuotaServiceDefault_ResetAllowance_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockGrantManager := pluginCore.NewMockGrantManager(tb)
		quotaService.(*QuotaServiceDefault).grantManager = mockGrantManager

		userID := uint(testUserID)

		grants := []*pluginModels.AllowanceGrant{
			{
				Model: gorm.Model{
					ID: 1,
				},

				IsActive: true,
			},
			{

				Model: gorm.Model{
					ID: 2,
				},
				IsActive: true,
			},
		}

		mockGrantManager.On("GetActiveGrants", userID).Return(grants, nil)
		mockGrantManager.On("DeactivateGrant", uint(1)).Return(nil)
		mockGrantManager.On("DeactivateGrant", uint(2)).Return(nil)

		err := quotaService.ResetAllowance(userID)
		require.NoError(t, err)

		mockGrantManager.AssertExpectations(t)
	}, testOptions())
}

// TestQuotaServiceDefault_CleanupOldRecords_Success tests successful cleanup of old records
func TestQuotaServiceDefault_CleanupOldRecords_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		retentionDays := 30

		err := quotaService.CleanupOldRecords(retentionDays)
		require.NoError(t, err)
	}, testOptions())
}

// TestQuotaServiceDefault_Getters tests the various getter methods
func TestQuotaServiceDefault_Getters(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		mockUsageManager := pluginCore.NewMockUsageManager(tb)
		mockGrantManager := pluginCore.NewMockGrantManager(tb)
		mockUsageAggregator := pluginCore.NewMockUsageAggregator(tb)
		mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockConfigManager := pluginCore.NewMockConfigManager(tb)

		quotaService.(*QuotaServiceDefault).usageManager = mockUsageManager
		quotaService.(*QuotaServiceDefault).grantManager = mockGrantManager
		quotaService.(*QuotaServiceDefault).usageAggregator = mockUsageAggregator
		quotaService.(*QuotaServiceDefault).planManager = mockQuotaPlanManager
		quotaService.(*QuotaServiceDefault).configManager = mockConfigManager

		assert.Equal(t, mockUsageManager, quotaService.GetUsageManager())
		assert.Equal(t, mockGrantManager, quotaService.GetGrantManager())
		assert.Equal(t, mockUsageAggregator, quotaService.GetUsageAggregator())
		assert.Equal(t, mockQuotaPlanManager, quotaService.GetQuotaPlanManager())
		assert.Equal(t, mockConfigManager, quotaService.GetConfigManager())
	}, testOptions())
}

// TestQuotaServiceDefault_Config tests the Config method
func TestQuotaServiceDefault_Config(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		cfg, err := quotaService.Config()
		require.NoError(t, err)
		assert.IsType(t, &config.QuotaConfig{}, cfg)
	}, testOptions())
}
