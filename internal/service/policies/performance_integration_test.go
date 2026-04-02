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
			uploadLimit: &largeValue,
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
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			quotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
			mockReservationManager.EXPECT().SumPendingBytesForUser(mock.Anything, userID, models.UsageTypeUpload).Return(int64(0))
			quotaWindowDuration := int64(86400) // 1 day in seconds
			quotaWindowStartHour := 0
			quotaWindowTimezone := "UTC"
			mockQuotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, uint64(1)).Return(&models.QuotaPlan{
				StorageLimitBytes:  1000,
				UploadLimitBytes:    1000,
				DownloadLimitBytes: 1000,
				WindowType:         models.WindowTypeCalendarDay,
				WindowDuration:     &quotaWindowDuration,
				WindowStartHour:    &quotaWindowStartHour,
				WindowTimezone:     &quotaWindowTimezone,
			}, nil)
			configWindowDuration := int64(86400)
			configWindowStartHour := 0
			configWindowTimezone := "UTC"
			mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, userID).Return(&models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				QuotaPlanID:       lo.ToPtr(uint64(1)),
				UploadLimitBytes:  uint64(largeValue),
				WindowType:        models.WindowTypeCalendarDay,
				WindowDuration:    &configWindowDuration,
				WindowStartHour:   &configWindowStartHour,
				WindowTimezone:    &configWindowTimezone,
			}, nil)

			// GetUsageManager is called once by CheckUploadQuota and potentially by getEffectiveLimits
			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager).Maybe()
			// CheckUploadQuota calls GetAggregatedUsageByType once for upload total limit check
			mockUsageManager.EXPECT().GetAggregatedUsageByType(mock.Anything, userID, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
			
			// Mock GetUsageForWindow with exact window parameters matching production code
			windowDuration := int64(86400)
			windowStartHour := 0
			windowTimezone := "UTC"
			window := pluginCore.LimitWindow{
				Type:      pluginCore.WindowTypeCalendarDay,
				Duration:  &windowDuration,
				StartHour: &windowStartHour,
				Timezone:  &windowTimezone,
			}
			mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, pluginCore.UsageTypeUpload, window).Return(uint64(largeValue/2)-100, time.Now(), time.Now(), nil)

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
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			quotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)
			thresholdWindowDuration := int64(86400)
			thresholdWindowStartHour := 0
			thresholdWindowTimezone := "UTC"
			mockUsageManager.EXPECT().GetUserQuotaConfig(ctx, thresholdUserID2).Return(&models.UserQuotaConfig{
				UserID:              thresholdUserID2,
				EnforcementPolicy:   models.EnforcementPolicyThreshold,
				UploadLimitBytes:    uint64(largeValue),
				UploadThreshold:     &thresholdValue,
				WindowType:          models.WindowTypeCalendarDay,
				WindowDuration:      &thresholdWindowDuration,
				WindowStartHour:     &thresholdWindowStartHour,
				WindowTimezone:      &thresholdWindowTimezone,
			}, nil)

			enforcer := NewThresholdPolicyEnforcer(ctx, quotaService)

			// Get existing config for the user
			config, err := quotaService.GetUsageManager().GetUserQuotaConfig(ctx, thresholdUserID2)
			require.NoError(t, err)

			// Setup GetUsageForWindow expectation
			mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, thresholdUserID2, pluginCore.UsageTypeUpload, mock.Anything).Return(uint64(thresholdValue/2)-100, time.Now(), time.Now(), nil)

			// Update the config with required limits by first deleting any existing record
			// and then creating a new one to avoid UNIQUE constraint issues
			var existingConfig models.UserQuotaConfig
			err = ctx.DB().Where("user_id = ?", thresholdUserID2).First(&existingConfig).Error
			if err == nil {
				// Config exists, update it
				existingConfig.UploadLimitBytes = uint64(largeValue)
				existingConfig.UploadThreshold = &thresholdValue
				err = ctx.DB().Save(&existingConfig).Error
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				// Config doesn't exist, create it
				newConfig := &models.UserQuotaConfig{
					UserID:              thresholdUserID2,
					EnforcementPolicy:   models.EnforcementPolicyThreshold,
					UploadLimitBytes:    uint64(largeValue),
					UploadThreshold:     &thresholdValue,
					WindowType:          models.WindowTypeCalendarDay,
					WindowDuration:      &thresholdWindowDuration,
					WindowStartHour:     &thresholdWindowStartHour,
					WindowTimezone:      &thresholdWindowTimezone,
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
			uploadLimit: &uploadDailyLimit,
		})

		t.Cleanup(func() {
			dataManager.Cleanup()
		})

		t.Run("Multiple rapid upload recordings", func(t *testing.T) {
			quotaService := pluginCore.NewMockQuotaService(t)
			mockUsageManager := pluginCore.NewMockUsageManager(t)
			mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
			quotaService.EXPECT().GetReservationManager().Return(mockReservationManager)
			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager).Maybe()
			mockReservationManager.EXPECT().SumPendingBytesForUser(mock.Anything, userID, models.UsageTypeUpload).Return(int64(0)).Maybe()

			// Setup mocks that will be called multiple times
			mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound).Maybe()
			mockUsageManager.EXPECT().GetAggregatedUsageByType(mock.Anything, userID, models.UsageTypeUpload).Return(uint64(0), nil).Maybe()
			
			// Mock GetUsageForWindow with exact window parameters matching production code
			rapidWindowDuration := int64(86400)
			rapidWindowStartHour := 0
			rapidWindowTimezone := "UTC"
			window := pluginCore.LimitWindow{
				Type:      pluginCore.WindowTypeCalendarDay,
				Duration:  &rapidWindowDuration,
				StartHour: &rapidWindowStartHour,
				Timezone:  &rapidWindowTimezone,
			}
			mockUsageManager.EXPECT().GetUsageForWindow(mock.Anything, userID, pluginCore.UsageTypeUpload, window).Return(uint64(0), time.Now(), time.Now(), nil).Maybe()

			// Setup config mock to handle multiple calls
			mockUsageManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(&models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadLimitBytes:  uint64(uploadDailyLimit),
				WindowType:        models.WindowTypeCalendarDay,
				WindowDuration:    &rapidWindowDuration,
				WindowStartHour:   &rapidWindowStartHour,
				WindowTimezone:    &rapidWindowTimezone,
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
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			// GetUsageManager is called twice - once by constructor and once by GetUsageHistory
			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager).Twice()
			quotaService.EXPECT().GetReservationManager().Return(mockReservationManager)

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
			mockReservationManager := pluginCore.NewMockReservationManager(t)

			quotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
			quotaService.EXPECT().GetReservationManager().Return(mockReservationManager)

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
