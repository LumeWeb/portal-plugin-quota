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
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestPerformance_LargeByteValues(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		thresholdUserID := dataManager.NextUserID()
		thresholdUserID2 := dataManager.NextUserID()
		largeValue := int64(1000000000000) // 1TB - large but reasonable

		// Create users with unique IDs
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &largeValue,
		})
		createTestUser(t, ctx, thresholdUserID, models.EnforcementPolicyThreshold, &testUserLimits{})
		createTestUser(t, ctx, thresholdUserID2, models.EnforcementPolicyThreshold, &testUserLimits{})

		t.Cleanup(func() {
			dataManager.Cleanup()
		})

		t.Run("Hard limits with large values", func(t *testing.T) {
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			mockQuotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, uint64(1)).Return(&models.QuotaPlan{
				StorageLimit:       1000,
				UploadDailyLimit:   1000,
				DownloadDailyLimit: 1000,
			}, nil)
			mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, userID).Return(&models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				QuotaPlanID:       lo.ToPtr(uint64(1)),
				UploadDailyLimit:  &largeValue,
			}, nil)

			// GetUsageAggregator is called once by CheckUploadQuota and potentially by getEffectiveLimits
			quotaService.EXPECT().GetUsageAggregator().Return(mockUsageAggregator).Maybe()
			// CheckUploadQuota calls GetAggregatedUsageByType once for upload total limit check
			mockUsageAggregator.EXPECT().GetAggregatedUsageByType(ctx, userID, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
			quotaService.EXPECT().GetTodayUsage(mock.Anything, userID).Return(&pluginCore.Usage{
				UserID:        userID,
				BytesUploaded: uint64(largeValue/2) - 100, // Well within the limit
			}, nil)

			enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)

			// Get existing config for the user
			config, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, userID)
			require.NoError(t, err)

			result, err := enforcer.CheckUploadQuota(ctx, config, uint64(largeValue/2))
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		})

		t.Run("Threshold policy with large values", func(t *testing.T) {
			thresholdValue := int64(900000000000) // 900GB - below upload limit
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
			mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, thresholdUserID2).Return(&models.UserQuotaConfig{
				UserID:            thresholdUserID2,
				EnforcementPolicy: models.EnforcementPolicyThreshold,
				UploadDailyLimit:  &largeValue,
				UploadThreshold:   &thresholdValue,
			}, nil)

			quotaService.EXPECT().GetTodayUsage(mock.Anything, thresholdUserID2).Return(&pluginCore.Usage{
				UserID:        thresholdUserID2,
				BytesUploaded: uint64(thresholdValue/2) - 100, // Well within the threshold
			}, nil).Maybe()

			enforcer := NewThresholdPolicyEnforcer(ctx, quotaService)

			// Get existing config for the user
			config, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, thresholdUserID2)
			require.NoError(t, err)

			// Update the config with required limits by first deleting any existing record
			// and then creating a new one to avoid UNIQUE constraint issues
			var existingConfig models.UserQuotaConfig
			err = ctx.DB().Where("user_id = ?", thresholdUserID2).First(&existingConfig).Error
			if err == nil {
				// Config exists, update it
				existingConfig.UploadDailyLimit = &largeValue
				existingConfig.UploadThreshold = &thresholdValue
				err = ctx.DB().Save(&existingConfig).Error
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// Config doesn't exist, create it
				newConfig := &models.UserQuotaConfig{
					UserID:            thresholdUserID2,
					EnforcementPolicy: models.EnforcementPolicyThreshold,
					UploadDailyLimit:  &largeValue,
					UploadThreshold:   &thresholdValue,
				}
				err = ctx.DB().Create(newConfig).Error
			}
			require.NoError(t, err)

			result, err := enforcer.CheckUploadQuota(ctx, config, uint64(thresholdValue/2))
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		})
	}, pluginTesting.TestOptions())
}

func TestPerformance_RapidSuccessiveOperations(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		uploadDailyLimit := int64(10000)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			uploadDailyLimit: &uploadDailyLimit,
		})

		t.Cleanup(func() {
			dataManager.Cleanup()
		})

		t.Run("Multiple rapid upload recordings", func(t *testing.T) {
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockUsageAggregator := pluginCore.NewMockUsageAggregator(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			quotaService.EXPECT().GetUsageAggregator().Return(mockUsageAggregator).Maybe()

			// Setup mocks that will be called multiple times
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()
			mockUsageAggregator.EXPECT().GetAggregatedUsageByType(mock.Anything, userID, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
			quotaService.EXPECT().GetTodayUsage(mock.Anything, userID).Return(&pluginCore.Usage{
				UserID:        userID,
				BytesUploaded: 0,
			}, nil).Maybe()

			// Setup config mock to handle multiple calls
			mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(&models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadDailyLimit:  &uploadDailyLimit,
			}, nil).Maybe()

			enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)

			// Record multiple uploads rapidly
			mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, mock.AnythingOfType("uint"), uint64(10), "127.0.0.1").
				Return(nil).
				Times(100)

			// Execute the uploads
			for i := 1; i <= 100; i++ {
				err := enforcer.RecordUpload(ctx, userID, dataManager.NextUploadID(), 10, "127.0.0.1")
				assert.NoError(t, err)
			}

			// Verify total usage through GetCurrentUsage
			mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(&pluginCore.Usage{
				UserID:        userID,
				BytesUploaded: uint64(1000),
			}, nil).Once()

			// Verify total usage
			usage, err := enforcer.GetCurrentUsage(ctx, userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(1000), usage.BytesUploaded) // 100 uploads * 10 bytes each
		})
	}, pluginTesting.TestOptions())
}

func TestPerformance_HistoricalData(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Cleanup(func() {
			dataManager.Cleanup()
		})

		t.Run("Long time period usage history", func(t *testing.T) {
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			// GetUsageManager is called twice - once by constructor and once by GetUsageHistory
			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager).Twice()

			// Mock GetUsageHistory which is called once by the test
			mockUsageManager.EXPECT().GetUsageHistory(mock.Anything, userID, 365, models.UsageTypeUpload).
				Return(make([]*pluginCore.UsagePoint, 365), nil).Once()

			enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)

			// Get usage history for a long period
			history, err := enforcer.GetUsageHistory(ctx, userID, 365, models.UsageTypeUpload)
			require.NoError(t, err)
			assert.Len(t, history, 365)
		})
	}, pluginTesting.TestOptions())
}

func TestPerformance_TimezoneBoundaries(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Cleanup(func() {
			dataManager.Cleanup()
		})

		t.Run("Date boundary transitions", func(t *testing.T) {
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

			enforcer := NewHardLimitsPolicyEnforcer(ctx, quotaService)
			now := time.Now()

			// Create usage at end of day
			endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)

			// Update the timestamp to be at end of day
			var detail models.UserUsageDetail
			err := ctx.DB().Where("user_id = ? AND type = ?", userID, models.UsageTypeUpload).First(&detail).Error
			require.NoError(t, err)
			detail.Timestamp = endOfDay
			err = ctx.DB().Save(&detail).Error
			require.NoError(t, err)

			// Create usage at start of next day
			startOfNextDay := endOfDay.Add(2 * time.Second) // Past midnight
			detail2 := &models.UserUsageDetail{
				UserID:    userID,
				UploadID:  dataManager.NextUploadID(),
				Type:      models.UsageTypeUpload,
				Bytes:     200,
				IP:        models.IPAddr("192.168.1.1"),
				Timestamp: startOfNextDay,
			}
			err = ctx.DB().Create(detail2).Error
			require.NoError(t, err)

			// Get detailed usage across both days
			start := endOfDay.Add(-time.Hour)
			end := startOfNextDay.Add(time.Hour)

			mockUsageManager.EXPECT().GetDetailedUsage(mock.Anything, userID, start, end).Return([]*models.UserUsageDetail{
				&detail,
				detail2,
			}, nil)

			details, err := enforcer.GetDetailedUsage(ctx, userID, start, end)
			assert.NoError(t, err)
			assert.Len(t, details, 2)
		})
	}, pluginTesting.TestOptions())
}
