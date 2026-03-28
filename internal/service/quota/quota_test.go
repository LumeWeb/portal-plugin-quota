package quota

import (
	"errors"
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
	"go.lumeweb.com/queryutil"
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

// TestQuotaServiceDefault_UpdateQuotaPlan_NameChanged_Validation tests that
// when Name is actually being changed, validation still runs and rejects empty names.
func TestQuotaServiceDefault_UpdateQuotaPlan_NameChanged_Validation(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan first
		createPlan := &pluginModels.QuotaPlan{
			Name:               "Original Plan Name",
			Description:        "Original description",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}

		err := quotaService.CreateQuotaPlan(ctx, createPlan)
		require.NoError(t, err)
		require.NotZero(t, createPlan.ID)

		// Act - Try to change Name to empty string
		existingPlan, err := quotaService.GetQuotaPlan(ctx, createPlan.ID)
		require.NoError(t, err)

		existingPlan.Name = "" // Attempt to change Name to empty (invalid)
		existingPlan.Description = "Updated description"

		err = quotaService.UpdateQuotaPlan(ctx, createPlan.ID, existingPlan)

		// Assert - Update should fail with validation error
		assert.Error(t, err, "Update should fail when Name is changed to empty string")
		assert.Contains(t, err.Error(), "name must not be empty",
			"Error should indicate Name validation failed")
	}, testOptions())
}

// TestQuotaServiceDefault_UpdateQuotaPlan_PartialUpdate tests that updating a quota plan
// with some (but not all) fields works correctly and preserves existing values.
// This test verifies partial updates don't trigger validation errors for unchanged fields.
func TestQuotaServiceDefault_UpdateQuotaPlan_PartialUpdate(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan first
		createPlan := &pluginModels.QuotaPlan{
			Name:               "Original Plan Name",
			Description:        "Original description",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}

		err := quotaService.CreateQuotaPlan(ctx, createPlan)
		require.NoError(t, err)
		require.NotZero(t, createPlan.ID)

		// Arrange - Fetch the existing plan (this is what the handler does)
		existingPlan, err := quotaService.GetQuotaPlan(ctx, createPlan.ID)
		require.NoError(t, err)
		require.NotNil(t, existingPlan)

		// Act - Update the existing plan's fields (this is what the handler code does)
		// Simulating the E2E test request body with updated limits
		existingPlan.Description = "Updated description"
		existingPlan.StorageLimit = 21474836480
		existingPlan.UploadDailyLimit = 209715200
		existingPlan.DownloadDailyLimit = 1048576000
		existingPlan.UploadTotalLimit = 21474836480
		existingPlan.DownloadTotalLimit = 10737418240
		// Note: existingPlan.Name is NOT updated, so it retains "Original Plan Name"

		// Act - Perform the update with the SAME plan object that was fetched
		err = quotaService.UpdateQuotaPlan(ctx, createPlan.ID, existingPlan)

		// Assert - The update should succeed
		if err != nil {
			tb.Logf("UpdateQuotaPlan failed with error: %v", err)
		}
		require.NoError(t, err, "UpdateQuotaPlan should succeed even when name is preserved from existing plan")

		// Act - Fetch the updated plan
		var resultingPlan pluginModels.QuotaPlan
		err = ctx.DB().Where("id = ?", createPlan.ID).First(&resultingPlan).Error
		require.NoError(t, err)

		// Assert - Verify all fields were updated
		assert.Equal(t, "Original Plan Name", resultingPlan.Name,
			"Name should remain unchanged")
		assert.Equal(t, "Updated description", resultingPlan.Description,
			"Description should be updated")
		assert.Equal(t, int64(21474836480), resultingPlan.StorageLimit,
			"Storage limit should be updated")
		assert.Equal(t, int64(209715200), resultingPlan.UploadDailyLimit,
			"Upload daily limit should be updated")
		assert.Equal(t, int64(1048576000), resultingPlan.DownloadDailyLimit,
			"Download daily limit should be updated")
	}, testOptions())
}

// TestQuotaServiceDefault_UpdateQuotaPlan_PartialUpdate_InvalidLimits tests that during
// partial updates (when Name is unchanged), validation still enforces data integrity
// by rejecting invalid limit values such as negative numbers.
func TestQuotaServiceDefault_UpdateQuotaPlan_PartialUpdate_InvalidLimits(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan first
		createPlan := &pluginModels.QuotaPlan{
			Name:               "Original Plan Name",
			Description:        "Original description",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}

		err := quotaService.CreateQuotaPlan(ctx, createPlan)
		require.NoError(t, err)
		require.NotZero(t, createPlan.ID)

		// Arrange - Fetch the existing plan (this is what the handler does)
		existingPlan, err := quotaService.GetQuotaPlan(ctx, createPlan.ID)
		require.NoError(t, err)
		require.NotNil(t, existingPlan)

		// Act - Try to update with invalid negative limit during partial update
		// Name is unchanged, but we still need to validate limits using change detection
		existingPlan.StorageLimit = -100 // Invalid: less than -1
		err = quotaService.UpdateQuotaPlan(ctx, createPlan.ID, existingPlan)

		// Assert - Update should fail with invalid limit error
		assert.Error(t, err, "Update should fail with invalid negative limit")
		assert.Contains(t, err.Error(), "storage_limit",
			"Error should reference the invalid field")
		// Fetch again and verify the old limit is still in place (no corruption)
		var resultingPlan pluginModels.QuotaPlan
		_ = ctx.DB().Where("id = ?", createPlan.ID).First(&resultingPlan)
		assert.Equal(t, int64(10737418240), resultingPlan.StorageLimit,
			"Storage limit should remain unchanged after failed validation")
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_WithNonExistentName tests that creating a plan
// with a name that doesn't exist in the database (the common case) should succeed.
//
// This test replicates the bug scenario reported:
// - A new quota plan is being created with name "Enterprise Plan"
// - The duplicate name check calls GetQuotaPlanByName("Enterprise Plan")
// - GetQuotaPlanByName returns ErrQuotaPlanNotFound (plan doesn't exist - expected state)
// - The CreateQuotaPlan should NOT fail with "failed to check for existing plan"
// - It should proceed to create the plan successfully
//
// Bug: The CreateQuotaPlan implementation in fix/duplicate-quota-plan-name-error-handling
// incorrectly treats ErrQuotaPlanNotFound as an actual error instead of the expected
// "plan doesn't exist" state, causing create to fail.
func TestQuotaServiceDefault_CreateQuotaPlan_WithNonExistentName(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Mock the plan manager to simulate what happens when checking for a plan name
		// that doesn't exist
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		quotaService.(*QuotaServiceDefault).planManager = mockPlanManager

		// Setup: GetQuotaPlanByName returns ErrQuotaPlanNotFound because the plan
		// doesn't exist yet (this is the expected state when creating a new plan)
		mockPlanManager.EXPECT().
			GetQuotaPlanByName(mock.Anything, "Enterprise Plan").
			Return(nil, pluginModels.ErrQuotaPlanNotFound)

		// Act: Create the plan
		plan := &pluginModels.QuotaPlan{
			Name:        "Enterprise Plan",
			Description: "Enterprise tier quota plan",
		}

		err := quotaService.CreateQuotaPlan(ctx, plan)

		// Assert: Creation should succeed, not fail with "failed to check for existing plan"
		// The error message "failed to create quota plan: failed to check for existing plan: quota plan not found: Enterprise Plan"
		// indicates the implementation incorrectly treats the "not found" error as an actual error

		// This assert will fail with the buggy behavior:
		// Error: "failed to create quota plan: failed to check for existing plan: quota plan not found: Enterprise Plan"
		require.NoError(t, err, "Creating plan with non-existent name should succeed")

		// Verify plan was actually created in the database
		require.NotZero(t, plan.ID, "Plan should have been assigned an ID")

		// Verify we can retrieve it back
		retrieved, err := quotaService.GetQuotaPlan(ctx, plan.ID)
		require.NoError(t, err)
		assert.Equal(t, "Enterprise Plan", retrieved.Name)
		assert.Equal(t, "Enterprise tier quota plan", retrieved.Description)
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_WithExistingName tests that creating a plan
// with a name that already exists returns a conflict error.
func TestQuotaServiceDefault_CreateQuotaPlan_WithExistingName(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Mock the plan manager to simulate what happens when checking for a plan name
		// that already exists
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		quotaService.(*QuotaServiceDefault).planManager = mockPlanManager

		// Setup: GetQuotaPlanByName returns an existing plan (name is taken)
		existingPlan := &pluginModels.QuotaPlan{
			Model: gorm.Model{ID: 1},
			Name:  "Enterprise Plan",
		}
		mockPlanManager.EXPECT().
			GetQuotaPlanByName(mock.Anything, "Enterprise Plan").
			Return(existingPlan, nil)

		// Act: Try to create a plan with the same name
		plan := &pluginModels.QuotaPlan{
			Name:        "Enterprise Plan",
			Description: "Another Enterprise tier quota plan",
		}

		err := quotaService.CreateQuotaPlan(ctx, plan)

		// Assert: Should get name conflict error
		require.Error(t, err)
		require.ErrorIs(t, err, pluginModels.ErrQuotaPlanNameExists)
	}, testOptions())
}

// TestQuotaServiceDefault_CreateQuotaPlan_WithDatabaseError tests that actual database errors
// during the duplicate check are properly propagated.
func TestQuotaServiceDefault_CreateQuotaPlan_WithDatabaseError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)

		// Mock the plan manager to simulate a database error
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		quotaService.(*QuotaServiceDefault).planManager = mockPlanManager

		// Setup: GetQuotaPlanByName returns a database error (e.g., connection issue)
		dbError := errors.New("database connection failed")
		mockPlanManager.EXPECT().
			GetQuotaPlanByName(mock.Anything, "Enterprise Plan").
			Return(nil, dbError)

		// Act: Try to create a plan
		plan := &pluginModels.QuotaPlan{
			Name:        "Enterprise Plan",
			Description: "Enterprise tier quota plan",
		}

		err := quotaService.CreateQuotaPlan(ctx, plan)

		// Assert: The database error should be wrapped and propagated
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to check for existing plan")
		assert.Contains(t, err.Error(), "database connection failed")
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
			Name:               "Enterprise Plan",
			Description:        "First enterprise plan",
			StorageLimit:       1000,
			UploadDailyLimit:   500,
			DownloadDailyLimit: 750,
		}
		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)
		require.NotZero(t, plan1.ID)

		// Try to create second plan with same name
		plan2 := &pluginModels.QuotaPlan{
			Name:               "Enterprise Plan", // Duplicate name
			Description:        "Second enterprise plan",
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
		expectedUpload := uint64(6000)   // 1000 + 2000 + 3000
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
			UserID:          1,
			Date:            today,
			BytesUploaded:   500,
			BytesDownloaded: 250,
			BytesStored:     1000,
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

// TestSetDefaultQuotaPlan_SetsDefaultAndRetrieves tests that SetDefaultQuotaPlan
// correctly marks a plan as default and GetDefaultQuotaPlan returns it.
func TestSetDefaultQuotaPlan_SetsDefaultAndRetrieves(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan
		plan := &pluginModels.QuotaPlan{
			Name:               "Default Test Plan",
			Description:        "Test plan for default bug",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}
		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)
		planID := plan.ID

		// Act - Set plan as default
		err = quotaService.SetDefaultQuotaPlan(ctx, planID)
		require.NoError(t, err)

		// Assert - Verify is_default flag is set
		updatedPlan, err := quotaService.GetQuotaPlan(ctx, planID)
		require.NoError(t, err)
		assert.True(t, updatedPlan.IsDefault, "Plan should be marked as default")

		// BUG CHECK: GetDefaultQuotaPlan should return the plan we just set
		defaultPlan, err := quotaService.GetDefaultQuotaPlan(ctx)
		require.NoError(t, err)
		assert.NotNil(t, defaultPlan, "GetDefaultQuotaPlan should return a plan")
		assert.Equal(t, planID, defaultPlan.ID, "Default plan ID should match the plan we set")

	}, testOptions())
}

// TestSetDefaultQuotaPlan_SwitchDefaultPlan tests switching from one default plan to another.
// This is a regression test for the duplicate key error that occurred when setting a new default,
// which happens when two plans temporarily have is_default=true and is_active=1,
// violating the uniq_default_active_one constraint.
func TestSetDefaultQuotaPlan_SwitchDefaultPlan(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create two quota plans
		plan1 := &pluginModels.QuotaPlan{
			Name:               "First Plan",
			Description:        "First default plan",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}
		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)
		plan1ID := plan1.ID

		plan2 := &pluginModels.QuotaPlan{
			Name:               "Second Plan",
			Description:        "Second default plan",
			StorageLimit:       5368709120,
			UploadDailyLimit:   52428800,
			DownloadDailyLimit: 262144000,
			UploadTotalLimit:   5368709120,
			DownloadTotalLimit: 2684354560,
			IsDefault:          false,
			IsActive:           new(true),
		}
		err = quotaService.CreateQuotaPlan(ctx, plan2)
		require.NoError(t, err)
		plan2ID := plan2.ID

		// Act 1 - Set first plan as default
		err = quotaService.SetDefaultQuotaPlan(ctx, plan1ID)
		require.NoError(t, err)

		// Assert 1 - Verify plan1 is default
		defaultPlan1, err := quotaService.GetDefaultQuotaPlan(ctx)
		require.NoError(t, err)
		assert.NotNil(t, defaultPlan1)
		assert.Equal(t, plan1ID, defaultPlan1.ID)
		assert.True(t, defaultPlan1.IsDefault)

		// Act 2 - Switch to second plan (this was causing duplicate key error)
		err = quotaService.SetDefaultQuotaPlan(ctx, plan2ID)
		require.NoError(t, err, "Switching default plan should not cause duplicate key error")

		// Assert 2 - Verify plan2 is now default and plan1 is not
		defaultPlan2, err := quotaService.GetDefaultQuotaPlan(ctx)
		require.NoError(t, err)
		assert.NotNil(t, defaultPlan2)
		assert.Equal(t, plan2ID, defaultPlan2.ID, "Second plan should now be default")

		// Verify the old default plan is no longer marked as default
		oldPlan, err := quotaService.GetQuotaPlan(ctx, plan1ID)
		require.NoError(t, err)
		assert.False(t, oldPlan.IsDefault, "Old default plan should no longer be marked as default")

		// Verify the new default plan is marked as default
		newPlan, err := quotaService.GetQuotaPlan(ctx, plan2ID)
		require.NoError(t, err)
		assert.True(t, newPlan.IsDefault, "New default plan should be marked as default")

	}, testOptions())
}

// TestUpdateQuotaPlan_CannotChangeDefault tests that UpdateQuotaPlan rejects attempts
// to change the IsDefault field. This is a regression test to ensure that only
// SetDefaultQuotaPlan can change default plans, preventing duplicate key violations.
func TestUpdateQuotaPlan_CannotChangeDefault(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a non-default plan
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Plan",
			Description:        "Original plan",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}
		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)
		planID := plan.ID

		// Act & Assert - Try to set IsDefault=true via UpdateQuotaPlan
		updatePlan := &pluginModels.QuotaPlan{
			Name:               "Test Plan",
			Description:        "Updated Plan",
			StorageLimit:       5368709120,
			UploadDailyLimit:   52428800,
			DownloadDailyLimit: 262144000,
			UploadTotalLimit:   5368709120,
			DownloadTotalLimit: 2684354560,
			IsDefault:          true, // Try to set to default
			IsActive:           new(true),
		}
		err = quotaService.UpdateQuotaPlan(ctx, planID, updatePlan)
		assert.Error(t, err, "UpdateQuotaPlan should return error when trying to change IsDefault to true")
		assert.Contains(t, err.Error(), "cannot change default", "Error message should indicate the restriction")

		// Verify plan was NOT updated
		updatedPlan, err := quotaService.GetQuotaPlan(ctx, planID)
		require.NoError(t, err)
		assert.Equal(t, "Test Plan", updatedPlan.Name, "Plan name should remain unchanged")
		assert.False(t, updatedPlan.IsDefault, "Plan should still not be default")

		// Arrange - Set plan as default using proper method
		err = quotaService.SetDefaultQuotaPlan(ctx, planID)
		require.NoError(t, err)

		// Act & Assert - Try to unset IsDefault via UpdateQuotaPlan
		unsetPlan := &pluginModels.QuotaPlan{
			Name:               "Test Plan",
			Description:        "Unset Default",
			StorageLimit:       5368709120,
			UploadDailyLimit:   52428800,
			DownloadDailyLimit: 262144000,
			UploadTotalLimit:   5368709120,
			DownloadTotalLimit: 2684354560,
			IsDefault:          false, // Try to unset default
			IsActive:           new(true),
		}
		err = quotaService.UpdateQuotaPlan(ctx, planID, unsetPlan)
		assert.Error(t, err, "UpdateQuotaPlan should return error when trying to unset IsDefault")
		assert.Contains(t, err.Error(), "cannot change default", "Error message should indicate the restriction")

		// Verify plan is still default
		defaultPlan, err := quotaService.GetDefaultQuotaPlan(ctx)
		require.NoError(t, err)
		assert.Equal(t, planID, defaultPlan.ID, "Plan should still be default")
		assert.Equal(t, "Test Plan", defaultPlan.Name, "Plan name should remain unchanged")

	}, testOptions())
}

// TestCreateQuotaPlan_CannotCreateAsDefault tests that CreateQuotaPlan rejects attempts
// to create a plan with IsDefault=true. This prevents duplicate key violations
// by ensuring default status must be set via SetDefaultQuotaPlan after creation.
func TestCreateQuotaPlan_CannotCreateAsDefault(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Act & Assert - Try to create a plan with IsDefault=true
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Default Plan",
			Description:        "Attempt to create as default",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          true, // This should be rejected
			IsActive:           new(true),
		}
		err := quotaService.CreateQuotaPlan(ctx, plan)
		assert.Error(t, err, "CreateQuotaPlan should return error when trying to create with IsDefault=true")
		assert.Contains(t, err.Error(), "cannot create plan with IsDefault=true", "Error message should indicate the restriction")
	}, testOptions())
}

// TestSetDefaultQuotaPlan_AfterSoftDelete tests that setting a default plan still works
// after the previous default plan has been soft-deleted. This edge case triggers
// the 'uniq_default_active_one' constraint violation if soft-deleted records with
// is_default=1 are not properly handled.
func TestSetDefaultQuotaPlan_AfterSoftDelete(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		db := ctx.DB()
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create first plan and set as default
		plan1 := &pluginModels.QuotaPlan{
			Name:               "First Plan",
			Description:        "First plan",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(true),
		}
		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)
		plan1ID := plan1.ID

		err = quotaService.SetDefaultQuotaPlan(ctx, plan1ID)
		require.NoError(t, err)

		// Verify plan1 is default
		defaultPlan, err := quotaService.GetDefaultQuotaPlan(ctx)
		require.NoError(t, err)
		assert.Equal(t, plan1ID, defaultPlan.ID)

		// Act - Soft-delete the default plan
		// Simulates a scenario where a plan is marked as deleted but still exists in database
		err = db.Model(&pluginModels.QuotaPlan{}).Where("id = ?", plan1ID).Delete(&pluginModels.QuotaPlan{}).Error
		require.NoError(t, err, "Failed to soft-delete plan")

		// Verify the plan is soft-deleted (exists in DB but has deleted_at set)
		var count int64
		err = db.Model(&pluginModels.QuotaPlan{}).Unscoped().Where("id = ?", plan1ID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(1), count, "Plan should still exist in database")

		// Arrange - Create second plan and try to set as default
		plan2 := &pluginModels.QuotaPlan{
			Name:               "Second Plan",
			Description:        "Second plan",
			StorageLimit:       5368709120,
			UploadDailyLimit:   52428800,
			DownloadDailyLimit: 262144000,
			UploadTotalLimit:   5368709120,
			DownloadTotalLimit: 2684354560,
			IsDefault:          false,
			IsActive:           new(true),
		}
		err = quotaService.CreateQuotaPlan(ctx, plan2)
		require.NoError(t, err)
		plan2ID := plan2.ID

		// Act - This should NOT fail with duplicate entry error
		err = quotaService.SetDefaultQuotaPlan(ctx, plan2ID)
		assert.NoError(t, err, "Setting new default after soft-delete should succeed")

		// Verify plan2 is now default
		newDefaultPlan, err := quotaService.GetDefaultQuotaPlan(ctx)
		require.NoError(t, err)
		assert.Equal(t, plan2ID, newDefaultPlan.ID, "Second plan should now be default")
		assert.True(t, newDefaultPlan.IsDefault, "New plan should be marked as default")
	}, testOptions())
}

// TestUpdateAllowanceGrant_PreservesUserID tests that updating an allowance grant
// doesn't overwrite the UserID with zero when only some fields are specified.
func TestUpdateAllowanceGrant_PreservesUserID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)
		grantManager := quotaService.GetGrantManager()

		// Arrange - Create an initial grant with userID 12345
		userID := uint(12345)
		initialGrant := &pluginModels.AllowanceGrant{
			Type:   pluginModels.GrantTypeStorage,
			Source: pluginModels.GrantSourceSubscription,
			Bytes:  10000,
		}
		err := grantManager.CreateAllowanceGrant(ctx, userID, initialGrant)
		require.NoError(t, err)
		grantID := initialGrant.ID
		require.NotZero(t, grantID)

		// Verify initial state
		savedGrant, err := grantManager.GetGrantByID(ctx, grantID)
		require.NoError(t, err)
		assert.Equal(t, userID, savedGrant.UserID, "Initial grant should have correct UserID")

		// Act - Simulate admin update behavior: get grant, modify only bytes, update
		targetGrant, err := grantManager.GetGrantByID(ctx, grantID)
		require.NoError(t, err)

		// Update only bytes (and expiry as needed) - don't touch UserID
		targetGrant.Bytes = 20000
		targetGrant.ExpiryDate = nil

		// This should succeed without validation error
		err = grantManager.UpdateAllowanceGrant(ctx, targetGrant)

		// Assert - Update should succeed
		require.NoError(t, err, "UpdateAllowanceGrant should succeed")

		// Verify the grant still has correct UserID
		updatedGrant, err := grantManager.GetGrantByID(ctx, grantID)
		require.NoError(t, err)
		assert.Equal(t, userID, updatedGrant.UserID, "UserID should not be overwritten")
		assert.Equal(t, uint64(20000), updatedGrant.Bytes, "Bytes should be updated")

	}, testOptions())
}

// TestDeleteQuotaPlan_WithAssignedUsers_PreventsDeletion tests that deleting a plan
// with assigned users fails with an appropriate error.
func TestDeleteQuotaPlan_WithAssignedUsers_PreventsDeletion(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Plan For Deletion",
			Description:        "Plan to test deletion prevention",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan.IsActive = true

		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)
		require.NotZero(t, plan.ID)

		// Assign a user to this plan
		userID := uint(99999)
		err = quotaService.AssignUserToPlan(ctx, userID, plan.ID)
		require.NoError(t, err)

		// Act - Try to delete the plan
		err = quotaService.DeleteQuotaPlan(ctx, plan.ID)

		// Assert - Deletion should fail
		require.Error(t, err)
		assert.Contains(t, err.Error(), "users assigned")

		// Verify the plan still exists
		existingPlan, err := quotaService.GetQuotaPlan(ctx, plan.ID)
		require.NoError(t, err)
		assert.NotNil(t, existingPlan)
	}, testOptions())
}

// TestDeleteQuotaPlan_WithoutAssignedUsers_Succeeds tests that deleting a plan
// with no assigned users succeeds.
func TestDeleteQuotaPlan_WithoutAssignedUsers_Succeeds(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Plan For Deletion Success",
			Description:        "Plan to test successful deletion",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan.IsActive = true

		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)
		require.NotZero(t, plan.ID)
		planID := plan.ID

		// Act - Delete the plan (no users assigned)
		err = quotaService.DeleteQuotaPlan(ctx, planID)

		// Assert - Deletion should succeed
		require.NoError(t, err)

		// Verify the plan no longer exists
		_, err = quotaService.GetQuotaPlan(ctx, planID)
		require.Error(t, err)
	}, testOptions())
}

// TestDeleteQuotaPlan_AfterRemovingUsers_Succeeds tests that a plan can be deleted
// after all users have been removed from it.
func TestDeleteQuotaPlan_AfterRemovingUsers_Succeeds(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a quota plan
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Plan Remove Then Delete",
			Description:        "Plan to test removal then deletion",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan.IsActive = true

		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)
		require.NotZero(t, plan.ID)

		// Assign a user to this plan
		userID := uint(88888)
		err = quotaService.AssignUserToPlan(ctx, userID, plan.ID)
		require.NoError(t, err)

		// Try to delete - should fail
		err = quotaService.DeleteQuotaPlan(ctx, plan.ID)
		require.Error(t, err)

		// Remove user from plan
		err = quotaService.RemoveUserFromPlan(ctx, userID)
		require.NoError(t, err)

		// Act - Now deletion should succeed
		err = quotaService.DeleteQuotaPlan(ctx, plan.ID)

		// Assert - Deletion should succeed
		require.NoError(t, err)
	}, testOptions())
}

// TestListUserQuotaConfigs_ReturnsConfigs tests that ListUserQuotaConfigs returns
// user quota configurations with pagination.
func TestListUserQuotaConfigs_ReturnsConfigs(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a plan and assign users
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Plan For List",
			Description:        "Plan for testing list configs",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan.IsActive = true

		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)

		// Assign multiple users to the plan
		for i := uint(1); i <= 3; i++ {
			err = quotaService.AssignUserToPlan(ctx, i+1000, plan.ID)
			require.NoError(t, err)
		}

		// Act - List user configs
		pagination, err := queryutil.NewPagination(0, 10)
		require.NoError(t, err)
		configs, total, err := quotaService.ListUserQuotaConfigs(ctx, nil, nil, pagination)

		// Assert
		require.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(3))
		assert.GreaterOrEqual(t, len(configs), 3)
	}, testOptions())
}

// TestListUserQuotaConfigs_WithPlanFilter_FiltersByPlan tests that ListUserQuotaConfigs
// correctly filters by plan ID.
func TestListUserQuotaConfigs_WithPlanFilter_FiltersByPlan(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create two plans
		plan1 := &pluginModels.QuotaPlan{
			Name:               "Test Plan 1",
			Description:        "First plan",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan1.IsActive = true

		plan2 := &pluginModels.QuotaPlan{
			Name:               "Test Plan 2",
			Description:        "Second plan",
			StorageLimit:       5368709120,
			UploadDailyLimit:   52428800,
			DownloadDailyLimit: 262144000,
			UploadTotalLimit:   5368709120,
			DownloadTotalLimit: 2684354560,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan2.IsActive = true

		err := quotaService.CreateQuotaPlan(ctx, plan1)
		require.NoError(t, err)
		err = quotaService.CreateQuotaPlan(ctx, plan2)
		require.NoError(t, err)

		// Assign users to different plans
		err = quotaService.AssignUserToPlan(ctx, 2001, plan1.ID)
		require.NoError(t, err)
		err = quotaService.AssignUserToPlan(ctx, 2002, plan1.ID)
		require.NoError(t, err)
		err = quotaService.AssignUserToPlan(ctx, 2003, plan2.ID)
		require.NoError(t, err)

		// Create filter for plan1 using queryutil.Equal
		planIDUint := uint64(plan1.ID)
		filters := []queryutil.CrudFilter{queryutil.Equal("quota_plan_id", planIDUint)}

		// Act - List configs filtered by plan1
		pagination, err := queryutil.NewPagination(0, 10)
		require.NoError(t, err)
		configs, total, err := quotaService.ListUserQuotaConfigs(ctx, filters, nil, pagination)

		// Assert
		require.NoError(t, err)
		assert.Equal(t, int64(2), total)
		assert.Len(t, configs, 2)

		// Verify all returned configs belong to plan1
		for _, config := range configs {
			require.NotNil(t, config.QuotaPlanID)
			assert.Equal(t, uint64(plan1.ID), *config.QuotaPlanID)
		}
	}, testOptions())
}

// TestUpdateUserQuotaConfig_UpdatesFields tests that UpdateUserQuotaConfig correctly
// updates the specified fields.
func TestUpdateUserQuotaConfig_UpdatesFields(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create initial config for user
		userID := uint(3001)
		initialConfig := &pluginCore.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		err := quotaService.SetQuotaConfig(ctx, userID, initialConfig)
		require.NoError(t, err)

		// Prepare update
		newStorageLimit := int64(999999999)
		newPolicy := pluginModels.EnforcementPolicyThreshold
		update := &pluginCore.UserQuotaConfigUpdate{
			StorageLimit:      &newStorageLimit,
			EnforcementPolicy: &newPolicy,
		}

		// Act - Update the config
		updatedConfig, err := quotaService.UpdateUserQuotaConfig(ctx, userID, update)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, updatedConfig)
		assert.Equal(t, newStorageLimit, *updatedConfig.StorageLimit)
		assert.Equal(t, newPolicy, updatedConfig.EnforcementPolicy)
	}, testOptions())
}

// TestUpdateUserQuotaConfig_NoFields_ReturnsError tests that UpdateUserQuotaConfig
// returns an error when no fields are provided to update.
func TestUpdateUserQuotaConfig_NoFields_ReturnsError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create initial config
		userID := uint(3002)
		initialConfig := &pluginCore.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		err := quotaService.SetQuotaConfig(ctx, userID, initialConfig)
		require.NoError(t, err)

		// Act - Update with empty update (no fields)
		update := &pluginCore.UserQuotaConfigUpdate{}
		updatedConfig, err := quotaService.UpdateUserQuotaConfig(ctx, userID, update)

		// Assert
		require.Error(t, err)
		require.Nil(t, updatedConfig)
		assert.Contains(t, err.Error(), "no fields to update")
	}, testOptions())
}

// TestResetUserQuotaPlan_SetsPlanToNull tests that ResetUserQuotaPlan correctly
// sets the quota_plan_id to NULL.
func TestResetUserQuotaPlan_SetsPlanToNull(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Arrange - Create a plan and assign user
		plan := &pluginModels.QuotaPlan{
			Name:               "Test Plan For Reset",
			Description:        "Plan for testing reset",
			StorageLimit:       10737418240,
			UploadDailyLimit:   104857600,
			DownloadDailyLimit: 524288000,
			UploadTotalLimit:   10737418240,
			DownloadTotalLimit: 5368709120,
			IsDefault:          false,
			IsActive:           new(bool),
		}
		*plan.IsActive = true

		err := quotaService.CreateQuotaPlan(ctx, plan)
		require.NoError(t, err)

		userID := uint(4001)
		err = quotaService.AssignUserToPlan(ctx, userID, plan.ID)
		require.NoError(t, err)

		// Verify user is assigned
		config, err := quotaService.GetQuotaConfig(ctx, userID)
		require.NoError(t, err)
		require.NotNil(t, config.QuotaPlanID)
		assert.Equal(t, uint64(plan.ID), *config.QuotaPlanID)

		// Act - Reset the plan
		err = quotaService.ResetUserQuotaPlan(ctx, userID)

		// Assert
		require.NoError(t, err)

		// Verify plan is now NULL
		config, err = quotaService.GetQuotaConfig(ctx, userID)
		require.NoError(t, err)
		assert.Nil(t, config.QuotaPlanID)
	}, testOptions())
}

// TestResetUserQuotaPlan_NonExistentUser_NoError tests that ResetUserQuotaPlan
// doesn't error when the user doesn't exist (no-op).
func TestResetUserQuotaPlan_NonExistentUser_NoError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		// Act - Reset plan for non-existent user
		nonExistentUserID := uint(999999)
		err := quotaService.ResetUserQuotaPlan(ctx, nonExistentUserID)

		// Assert - Should not error (no-op)
		require.NoError(t, err)
	}, testOptions())
}

