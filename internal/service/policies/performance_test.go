package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestPerformance_LargeByteValues(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		thresholdUserID := uint(2)
		thresholdUserID2 := uint(3)         // New user ID for threshold test - use 3 to avoid conflict
		largeValue := int64(1000000000000) // 1TB - large but reasonable
		createTestUser(t, ctx, userID, models.EnforcementPolicyUnlimited, &testUserLimits{})
		createTestUser(t, ctx, thresholdUserID, models.EnforcementPolicyThreshold, &testUserLimits{})
		createTestUser(t, ctx, thresholdUserID2, models.EnforcementPolicyThreshold, &testUserLimits{})

		t.Run("Hard limits with large values", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			config := &models.UserQuotaConfig{
				UserID:            userID,
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
				UploadDailyLimit:  &largeValue,
			}

			result, err := enforcer.CheckUploadQuota(config, uint64(largeValue/2))
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		})

		t.Run("Threshold policy with large values", func(t *testing.T) {
			enforcer := NewThresholdPolicyEnforcer(ctx)
			thresholdValue := int64(900000000000) // 900GB - below upload limit

			// Get existing config for the user
			config, err := enforcer.getUserQuotaConfig(thresholdUserID2)
			require.NoError(t, err)

			// Update the config with required limits
			config.UploadDailyLimit = &largeValue
			config.UploadThreshold = &thresholdValue
			err = ctx.DB().Save(config).Error
			require.NoError(t, err)

			result, err := enforcer.CheckUploadQuota(thresholdUserID2, uint64(thresholdValue/2))
			require.NoError(t, err)
			assert.True(t, result.Allowed)
		})
	}, testOptions())
}

func TestPerformance_RapidSuccessiveOperations(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		uploadDailyLimit := int64(10000)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{
			UploadDailyLimit: &uploadDailyLimit,
		})

		t.Run("Multiple rapid upload recordings", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)

			// Record multiple uploads rapidly
			for i := 1; i <= 100; i++ {
				err := enforcer.RecordUpload(userID, uint(i), 10, "127.0.0.1")
				assert.NoError(t, err)
			}

			// Verify total usage
			usage, err := enforcer.GetCurrentUsage(userID)
			require.NoError(t, err)
			assert.Equal(t, uint64(1000), usage.BytesUploaded) // 100 uploads * 10 bytes each
		})
	}, testOptions())
}

func TestPerformance_HistoricalData(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("Long time period usage history", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
			now := time.Now()

			// Create usage records across a long time period
			for i := 1; i <= 365; i++ {
				detail := &models.UserUsageDetail{
					UserID:    userID,
					UploadID:  uint(i),
					Type:      models.UsageTypeUpload,
					Bytes:     100,
					IP:        "192.168.1.1",
					Timestamp: now.Add(-time.Duration(i-1) * 24 * time.Hour),
				}
				err := ctx.DB().Create(detail).Error
				require.NoError(t, err)
			}

			// Get usage history for a long period
			history, err := enforcer.GetUsageHistory(userID, 365, models.UsageTypeUpload)
			assert.NoError(t, err)
			assert.Len(t, history, 365)
		})
	}, testOptions())
}

func TestPerformance_TimezoneBoundaries(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		t.Run("Date boundary transitions", func(t *testing.T) {
			enforcer := NewHardLimitsPolicyEnforcer(ctx)
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
				UploadID:  2,
				Type:      models.UsageTypeUpload,
				Bytes:     200,
				IP:        "192.168.1.1",
				Timestamp: startOfNextDay,
			}
			err = ctx.DB().Create(detail2).Error
			require.NoError(t, err)

			// Get detailed usage across both days
			start := endOfDay.Add(-time.Hour)
			end := startOfNextDay.Add(time.Hour)

			details, err := enforcer.GetDetailedUsage(userID, start, end)
			assert.NoError(t, err)
			assert.Len(t, details, 2)
		})
	}, testOptions())
}
