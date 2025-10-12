package policies

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestQuotaPlanManagerDefault_GetQuotaPlanByID_ValidID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create a test quota plan with unique name
		planName := "Test Plan " + t.Name()
		plan := createTestQuotaPlan(t, ctx, planName, true, &testPlanLimits{
			storageLimit:       1000,
			uploadDailyLimit:   500,
			downloadDailyLimit: 750,
			uploadTotalLimit:   5000,
			downloadTotalLimit: 10000,
		})

		manager := NewQuotaPlanManager(ctx.DB(), ctx.Logger())

		result, err := manager.GetQuotaPlanByID(uint64(plan.ID))
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, plan.Name, result.Name)
		assert.Equal(t, plan.StorageLimit, result.StorageLimit)
		assert.Equal(t, plan.UploadDailyLimit, result.UploadDailyLimit)
		assert.Equal(t, plan.DownloadDailyLimit, result.DownloadDailyLimit)
		assert.Equal(t, plan.UploadTotalLimit, result.UploadTotalLimit)
		assert.Equal(t, plan.DownloadTotalLimit, result.DownloadTotalLimit)
	}, pluginTesting.TestOptions())
}

func TestQuotaPlanManagerDefault_GetQuotaPlanByID_NonExistentID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		manager := NewQuotaPlanManager(ctx.DB(), ctx.Logger())

		result, err := manager.GetQuotaPlanByID(999999)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
	}, pluginTesting.TestOptions())
}

func TestQuotaPlanManagerDefault_GetDefaultQuotaPlan_Exists(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create a default quota plan
		plan := createTestQuotaPlan(t, ctx, "Default Plan", true, &testPlanLimits{
			storageLimit:       2000,
			uploadDailyLimit:   1000,
			downloadDailyLimit: 1500,
			uploadTotalLimit:   10000,
			downloadTotalLimit: 20000,
		})

		// Explicitly set this plan as default
		plan.IsDefault = true
		err := ctx.DB().Save(plan).Error
		require.NoError(t, err)

		manager := NewQuotaPlanManager(ctx.DB(), ctx.Logger())

		result, err := manager.GetDefaultQuotaPlan()
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Default Plan", result.Name)
		assert.Equal(t, int64(2000), result.StorageLimit)
		assert.Equal(t, int64(1000), result.UploadDailyLimit)
		assert.Equal(t, int64(1500), result.DownloadDailyLimit)
		assert.Equal(t, int64(10000), result.UploadTotalLimit)
		assert.Equal(t, int64(20000), result.DownloadTotalLimit)
		assert.True(t, result.IsDefault)
		assert.True(t, *result.IsActive)
	}, pluginTesting.TestOptions())
}

func TestQuotaPlanManagerDefault_GetDefaultQuotaPlan_NotExists(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		manager := NewQuotaPlanManager(ctx.DB(), ctx.Logger())

		result, err := manager.GetDefaultQuotaPlan()
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
		assert.Nil(t, result)
	}, pluginTesting.TestOptions())
}

func TestQuotaPlanManagerDefault_GetDefaultQuotaPlan_Inactive(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create a default quota plan but set it as inactive
		plan := createTestQuotaPlan(t, ctx, "Inactive Default Plan", false, &testPlanLimits{
			storageLimit:       3000,
			uploadDailyLimit:   1500,
			downloadDailyLimit: 2000,
			uploadTotalLimit:   15000,
			downloadTotalLimit: 30000,
		})

		// Explicitly set this plan as default and inactive
		plan.IsDefault = true
		plan.IsActive = lo.ToPtr(false) // Ensure inactive
		err := ctx.DB().Save(plan).Error
		require.NoError(t, err)

		manager := NewQuotaPlanManager(ctx.DB(), ctx.Logger())

		result, err := manager.GetDefaultQuotaPlan()
		assert.Error(t, err)
		assert.Equal(t, gorm.ErrRecordNotFound, err)
		assert.Nil(t, result)
	}, pluginTesting.TestOptions())
}
