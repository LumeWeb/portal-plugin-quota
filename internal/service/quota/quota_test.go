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
	return coreTesting.CombineOptions(coreTesting.NewMockPluginBuilder(internal.PluginName).
		WithMigrations(core.DBMigration{core.DB_TYPE_SQLITE: migrations.GetSQLite()}).
		WithService(pluginCore.QUOTA_SERVICE, NewQuotaService).BuilderOption(),
		coreTesting.WithServiceConfig(internal.PluginName, pluginCore.QUOTA_SERVICE, &config.QuotaConfig{
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

// TestQuotaServiceDefault_CreateQuotaPlan_DuplicateName tests that duplicate plan names are rejected
func TestQuotaServiceDefault_CreateQuotaPlan_DuplicateName(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Create first plan
		plan1 := &pluginModels.QuotaPlan{
			Name:        "Enterprise Plan",
			Description: "First enterprise plan",
			StorageLimit:       1000,
			UploadDailyLimit:   500,
			DownloadDailyLimit: 750,
		}
		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)
		require.NotZero(t, plan1.ID)

		// Try to create second plan with same name
		plan2 := &pluginModels.QuotaPlan{
			Name:        "Enterprise Plan", // Duplicate name
			Description: "Second enterprise plan",
			StorageLimit:       2000,
			UploadDailyLimit:   1000,
			DownloadDailyLimit: 1500,
		}
		err = quotaService.CreateQuotaPlan(ctx, plan2)

		// Verify error is returned
		require.Error(t, err)
		assert.ErrorIs(t, err, pluginModels.ErrQuotaPlanNameExists)

		// Verify second plan was not created in DB
		var count int64
		err = ctx.DB().Model(&pluginModels.QuotaPlan{}).Where("name = ?", "Enterprise Plan").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Should only have one plan with this name")

		// Verify first plan is still intact
		var savedPlan pluginModels.QuotaPlan
		err = ctx.DB().Where("name = ?", "Enterprise Plan").First(&savedPlan).Error
		require.NoError(t, err)
		assert.Equal(t, plan1.ID, savedPlan.ID)
		assert.Equal(t, "First enterprise plan", savedPlan.Description)
		assert.Equal(t, int64(1000), savedPlan.StorageLimit)
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_DuplicatePlanNames tests multiple duplicate attempts
func TestQuotaServiceDefault_CreateQuotaPlan_DuplicatePlanNames(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Create first plan
		plan1 := &pluginModels.QuotaPlan{
			Name:        "Standard Plan",
			Description: "Standard tier",
		}
		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)

		// Attempt to create duplicate multiple times
		for i := 0; i < 3; i++ {
			duplicatePlan := &pluginModels.QuotaPlan{
				Name:        "Standard Plan",
				Description: fmt.Sprintf("Duplicate attempt %d", i+1),
			}
			err = quotaService.CreateQuotaPlan(ctx, duplicatePlan)
			require.Error(t, err)
			assert.ErrorIs(t, err, pluginModels.ErrQuotaPlanNameExists)
		}

		// Verify only one plan exists
		var count int64
		err = ctx.DB().Model(&pluginModels.QuotaPlan{}).Where("name = ?", "Standard Plan").Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_SimilarNames tests that similar but different names are allowed
func TestQuotaServiceDefault_CreateQuotaPlan_SimilarNames(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Create plans with similar but different names
		planNames := []string{
			"Basic Plan",
			"Basic Plan Pro",
			"Basic Plan Premium",
			"basIC PLAN", // Case-sensitive test
		}

		for _, name := range planNames {
			plan := &pluginModels.QuotaPlan{
				Name:        name,
				Description: fmt.Sprintf("Plan for %s", name),
			}
			err := quotaService.CreateQuotaPlan(ctx, plan)
			require.NoError(t, err, "Should be able to create plan with name: %s", name)
		}

		// Verify all plans were created
		var count int64
		err := ctx.DB().Model(&pluginModels.QuotaPlan{}).Where("name IN ?", planNames).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(len(planNames)), count)
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_CaseSensitive tests that name comparison is case-sensitive
func TestQuotaServiceDefault_CreateQuotaPlan_CaseSensitive(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Create plan with specific case
		plan1 := &pluginModels.QuotaPlan{
			Name:        "Enterprise Plan",
			Description: "Original enterprise plan",
		}
		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)

		// Try to create plan with different case - should succeed due to case-sensitivity
		plan2 := &pluginModels.QuotaPlan{
			Name:        "enterprise plan", // Different case
			Description: "Different case plan",
		}
		err = quotaService.CreateQuotaPlan(ctx, plan2)
		require.NoError(t, err)

		// Verify both plans exist
		var count int64
		err = ctx.DB().Model(&pluginModels.QuotaPlan{}).Where("name IN ?", []string{"Enterprise Plan", "enterprise plan"}).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(2), count)

		// Verify they are different plans
		assert.NotEqual(t, plan1.ID, plan2.ID)
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

// TestQuotaServiceDefault_GetSystemStats_Success tests successful system stats retrieval
func TestQuotaServiceDefault_GetSystemStats_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)
		db := ctx.DB()

		// Create test data: users
		var userConfigs []*pluginModels.UserQuotaConfig
		for i := 1; i <= 5; i++ {
			config := &pluginModels.UserQuotaConfig{
				UserID:            uint(i),
				EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
			}
			err := db.Create(config).Error
			require.NoError(t, err)
			userConfigs = append(userConfigs, config)
		}

		today := time.Now().UTC().Truncate(24 * time.Hour)

		// Create test data: quota plans
		plans := []*pluginModels.QuotaPlan{
			{Name: "Plan 1", Description: "Test Plan 1", IsDefault: false, IsActive: &[]bool{true}[0]},
			{Name: "Plan 2", Description: "Test Plan 2", IsDefault: true, IsActive: &[]bool{true}[0]},
			{Name: "Plan 3", Description: "Test Plan 3", IsDefault: false, IsActive: &[]bool{false}[0]},
		}
		for _, plan := range plans {
			err := db.Create(plan).Error
			require.NoError(t, err)
		}

		// Create test data: grants
		grants := []*pluginModels.AllowanceGrant{
			{UserID: 1, Type: pluginModels.GrantTypeStorage, Source: pluginModels.GrantSourceBonus, Bytes: 1000, BytesUsed: 0, BytesRemaining: 1000, IsActive: true},
			{UserID: 2, Type: pluginModels.GrantTypeUpload, Source: pluginModels.GrantSourcePromo, Bytes: 500, BytesUsed: 100, BytesRemaining: 400, IsActive: true},
			{UserID: 3, Type: pluginModels.GrantTypeDownload, Source: pluginModels.GrantSourceSubscription, Bytes: 2000, BytesUsed: 1500, BytesRemaining: 500, IsActive: false},
		}
		for _, grant := range grants {
			err := db.Create(grant).Error
			require.NoError(t, err)
		}

		// Create test data: user quota usage
		userQuotas := []*pluginModels.UserQuota{
			{UserID: 1, Date: today, BytesUploaded: 1000, BytesDownloaded: 500, BytesStored: 2000},
			{UserID: 2, Date: today, BytesUploaded: 2000, BytesDownloaded: 1000, BytesStored: 3000},
			{UserID: 3, Date: today, BytesUploaded: 3000, BytesDownloaded: 1500, BytesStored: 4000},
		}
		for _, quota := range userQuotas {
			err := db.Create(quota).Error
			require.NoError(t, err)
		}

		// Act
		stats, err := quotaService.GetSystemStats(ctx)
		
		// Assert
		require.NoError(t, err)
		assert.NotNil(t, stats)

		// Verify user counts
		assert.Equal(t, int64(5), stats.TotalUsers)
		assert.Equal(t, int64(5), stats.ActiveUsers)

		// Verify plan counts
		assert.Equal(t, int64(3), stats.TotalPlans)
		assert.Equal(t, int64(2), stats.ActivePlans)

		// Verify grant counts
		assert.Equal(t, int64(3), stats.TotalGrants)
		assert.Equal(t, int64(2), stats.ActiveGrants)

		// Verify usage stats (sum of all user quotas)
		expectedUpload := uint64(6000)  // 1000 + 2000 + 3000
		expectedDownload := uint64(3000) // 500 + 1000 + 1500
		expectedStorage := uint64(9000)  // 2000 + 3000 + 4000
		expectedTotal := expectedUpload + expectedDownload + expectedStorage

		assert.Equal(t, expectedUpload, stats.CurrentUsage.BytesUploaded)
		assert.Equal(t, expectedDownload, stats.CurrentUsage.BytesDownloaded)
		assert.Equal(t, expectedStorage, stats.CurrentUsage.BytesStored)
		assert.Equal(t, expectedTotal, stats.TotalUsageBytes)
	}, testOptions())
}

// TestQuotaServiceDefault_GetSystemStats_EmptyDatabase tests system stats with empty database
func TestQuotaServiceDefault_GetSystemStats_EmptyDatabase(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Act
		stats, err := quotaService.GetSystemStats(ctx)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, stats)

		// All counts should be zero
		assert.Equal(t, int64(0), stats.TotalUsers)
		assert.Equal(t, int64(0), stats.ActiveUsers)
		assert.Equal(t, int64(0), stats.TotalPlans)
		assert.Equal(t, int64(0), stats.ActivePlans)
		assert.Equal(t, int64(0), stats.TotalGrants)
		assert.Equal(t, int64(0), stats.ActiveGrants)
		assert.Equal(t, uint64(0), stats.CurrentUsage.BytesUploaded)
		assert.Equal(t, uint64(0), stats.CurrentUsage.BytesDownloaded)
		assert.Equal(t, uint64(0), stats.CurrentUsage.BytesStored)
		assert.Equal(t, uint64(0), stats.TotalUsageBytes)
	}, testOptions())
}

// TestQuotaServiceDefault_GetSystemStats_PartialData tests system stats with partial data
func TestQuotaServiceDefault_GetSystemStats_PartialData(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)
		db := ctx.DB()

		// Create only users
		for i := 1; i <= 3; i++ {
			config := &pluginModels.UserQuotaConfig{
				UserID:            uint(i),
				EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
			}
			err := db.Create(config).Error
			require.NoError(t, err)
		}

		// Create only one user quota with data
		today := time.Now().UTC().Truncate(24 * time.Hour)
		userQuota := &pluginModels.UserQuota{
			UserID:        1,
			Date:          today,
			BytesUploaded: 500,
			BytesDownloaded: 250,
			BytesStored:   1000,
		}
		err := db.Create(userQuota).Error
		require.NoError(t, err)

		// Act
		stats, err := quotaService.GetSystemStats(ctx)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, stats)

		// Verify user counts
		assert.Equal(t, int64(3), stats.TotalUsers)
		assert.Equal(t, int64(3), stats.ActiveUsers)

		// Other counts should be zero
		assert.Equal(t, int64(0), stats.TotalPlans)
		assert.Equal(t, int64(0), stats.ActivePlans)
		assert.Equal(t, int64(0), stats.TotalGrants)
		assert.Equal(t, int64(0), stats.ActiveGrants)

		// Verify usage stats from the one user quota
		assert.Equal(t, uint64(500), stats.CurrentUsage.BytesUploaded)
		assert.Equal(t, uint64(250), stats.CurrentUsage.BytesDownloaded)
		assert.Equal(t, uint64(1000), stats.CurrentUsage.BytesStored)
		assert.Equal(t, uint64(1750), stats.TotalUsageBytes)
	}, testOptions())
}

