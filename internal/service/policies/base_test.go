package policies

import (
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestBasePolicyEnforcer_ValidateUserID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		tests := []struct {
			name    string
			userID  uint
			wantErr error
		}{
			{
				name:    "Valid user ID",
				userID:  1,
				wantErr: nil,
			},
			{
				name:    "Zero user ID",
				userID:  0,
				wantErr: models.ErrInvalidUserID,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := enforcer.validateUserID(tt.userID)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}, testOptions())
}

func TestBasePolicyEnforcer_ValidateBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		tests := []struct {
			name    string
			bytes   uint64
			wantErr error
		}{
			{
				name:    "Valid bytes",
				bytes:   100,
				wantErr: nil,
			},
			{
				name:    "Zero bytes",
				bytes:   0,
				wantErr: models.ErrInvalidBytes,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := enforcer.validateBytes(tt.bytes)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}, testOptions())
}

func TestBasePolicyEnforcer_ValidateRequestedBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		tests := []struct {
			name           string
			requestedBytes uint64
			wantErr        error
		}{
			{
				name:           "Valid requested bytes",
				requestedBytes: 100,
				wantErr:        nil,
			},
			{
				name:           "Zero requested bytes",
				requestedBytes: 0,
				wantErr:        models.ErrInvalidBytes,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := enforcer.validateRequestedBytes(tt.requestedBytes)
				if tt.wantErr != nil {
					assert.ErrorIs(t, err, tt.wantErr)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	}, testOptions())
}

func TestBasePolicyEnforcer_GetUserQuotaConfig(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		t.Run("Existing config", func(t *testing.T) {
			userID := uint(1)
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)
			assert.NotNil(t, config)
			assert.Equal(t, userID, config.UserID)
			assert.Equal(t, models.EnforcementPolicyHardLimits, config.EnforcementPolicy)
		})

		t.Run("Non-existent config creates default", func(t *testing.T) {
			userID := uint(999) // This user doesn't exist

			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)
			assert.NotNil(t, config)
			assert.Equal(t, userID, config.UserID)
			assert.Equal(t, models.EnforcementPolicyHardLimits, config.EnforcementPolicy) // Default policy
		})
	}, testOptions())
}

func TestBasePolicyEnforcer_GetCurrentUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		t.Run("No usage records", func(t *testing.T) {
			userID := uint(1)
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			usage, err := enforcer.getCurrentUsage(userID)
			require.NoError(t, err)
			assert.NotNil(t, usage)
			assert.Equal(t, userID, usage.UserID)
			assert.Equal(t, uint64(0), usage.BytesUploaded)
			assert.Equal(t, uint64(0), usage.BytesDownloaded)
			assert.Equal(t, uint64(0), usage.BytesStored)
		})

		t.Run("With usage records", func(t *testing.T) {
			userID := uint(2)
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})
			createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)
			createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 200)
			createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 300)

			usage, err := enforcer.getCurrentUsage(userID)
			require.NoError(t, err)
			assert.NotNil(t, usage)
			assert.Equal(t, userID, usage.UserID)
			assert.Equal(t, uint64(100), usage.BytesUploaded)
			assert.Equal(t, uint64(200), usage.BytesDownloaded)
			assert.Equal(t, uint64(300), usage.BytesStored)
		})
	}, testOptions())
}

func TestBasePolicyEnforcer_GetUsageHistory(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		// Create usage records with different timestamps
		now := time.Now()
		createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)

		// Create a record in the past
		oldDetail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  2,
			Type:      models.UsageTypeUpload,
			Bytes:     200,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour), // 2 days ago
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		t.Run("Get recent usage history", func(t *testing.T) {
			history, err := enforcer.getUsageHistory(userID, 1, models.UsageTypeUpload)
			require.NoError(t, err)
			assert.Len(t, history, 1) // Only the recent record
			assert.Equal(t, uint64(100), history[0].Bytes)
		})

		t.Run("Get all usage history", func(t *testing.T) {
			history, err := enforcer.getUsageHistory(userID, 3, models.UsageTypeUpload)
			require.NoError(t, err)
			assert.Len(t, history, 2)                      // Both records
			assert.Equal(t, uint64(200), history[0].Bytes) // Older record first
			assert.Equal(t, uint64(100), history[1].Bytes) // Newer record second
		})
	}, testOptions())
}

func TestBasePolicyEnforcer_GetDetailedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		// Create usage records
		now := time.Now()
		createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)
		createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 200)

		// Create a record outside the time range
		oldDetail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  3,
			Type:      models.UsageTypeStorageAdd,
			Bytes:     300,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour),
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		t.Run("Get detailed usage within time range", func(t *testing.T) {
			start := now.Add(-24 * time.Hour)
			end := now.Add(24 * time.Hour)

			details, err := enforcer.getDetailedUsage(userID, start, end)
			require.NoError(t, err)
			assert.Len(t, details, 2) // Only records within time range

			// Verify records are in descending order by timestamp
			assert.True(t, details[0].Timestamp.After(details[1].Timestamp))
		})
	}, testOptions())
}

func TestBasePolicyEnforcer_RecordUserUsageDetail(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

		detail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  1,
			Type:      models.UsageTypeUpload,
			Bytes:     100,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		err := enforcer.recordUserUsageDetail(detail)
		require.NoError(t, err)

		// Verify the record was created
		var savedDetail models.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, 1).First(&savedDetail).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedDetail.UserID)
		assert.Equal(t, uint64(100), savedDetail.Bytes)
		assert.Equal(t, models.UsageTypeUpload, savedDetail.Type)
	}, testOptions())
}

func TestBasePolicyEnforcer_UpdateDailyUsage(t *testing.T) {
	t.Run("Create new daily quota record", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewBasePolicyEnforcer(ctx)

			userID := uint(1)
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			err := enforcer.updateDailyUsage(userID, models.UsageTypeUpload, 100)
			require.NoError(t, err)

			// Verify the record was created
			var dailyQuota models.UserQuota
			today := time.Now().Truncate(24 * time.Hour)
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, userID, dailyQuota.UserID)
			assert.Equal(t, uint64(100), dailyQuota.BytesUploaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
			assert.Equal(t, uint64(0), dailyQuota.BytesStored)
		}, testOptions())
	})

	t.Run("Update existing daily quota record", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewBasePolicyEnforcer(ctx)

			userID := uint(1)
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			// First create a record
			err := enforcer.updateDailyUsage(userID, models.UsageTypeUpload, 100)
			require.NoError(t, err)

			// Then update it
			err = enforcer.updateDailyUsage(userID, models.UsageTypeUpload, 50)
			require.NoError(t, err)

			// Verify the record was updated
			var dailyQuota models.UserQuota
			today := time.Now().Truncate(24 * time.Hour)
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, uint64(150), dailyQuota.BytesUploaded) // 100 + 50
		}, testOptions())
	})

	t.Run("Different usage types", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			enforcer := NewBasePolicyEnforcer(ctx)

			userID := uint(1)
			createTestUser(t, ctx, userID, models.EnforcementPolicyHardLimits, &testUserLimits{})

			// Add different types of usage
			err := enforcer.updateDailyUsage(userID, models.UsageTypeUpload, 100)
			require.NoError(t, err)
			err = enforcer.updateDailyUsage(userID, models.UsageTypeDownload, 200)
			require.NoError(t, err)
			err = enforcer.updateDailyUsage(userID, models.UsageTypeStorageAdd, 300)
			require.NoError(t, err)

			// Verify all types were recorded correctly
			var dailyQuota models.UserQuota
			today := time.Now().Truncate(24 * time.Hour)
			err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
			require.NoError(t, err)
			assert.Equal(t, uint64(100), dailyQuota.BytesUploaded)
			assert.Equal(t, uint64(200), dailyQuota.BytesDownloaded)
			assert.Equal(t, uint64(300), dailyQuota.BytesStored)
		}, testOptions())
	})
}

func TestBasePolicyEnforcer_CreateQuotaCheckResult(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		t.Run("Success result", func(t *testing.T) {
			details := core.QuotaCheckDetails{
				CurrentUsage: 100,
				Policy:       models.EnforcementPolicyHardLimits,
			}

			result := enforcer.createQuotaCheckResult(true, models.QuotaCheckReasonOK, models.EnforcementPolicyHardLimits, details)
			assert.True(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
			assert.Equal(t, details, result.Details)
		})

		t.Run("Limit exceeded result", func(t *testing.T) {
			details := core.QuotaCheckDetails{
				CurrentUsage: 100,
				Limit:        lo.ToPtr(uint64(200)),
				Policy:       models.EnforcementPolicyHardLimits,
			}

			result := enforcer.createQuotaCheckResult(false, models.QuotaCheckReasonLimitExceeded, models.EnforcementPolicyHardLimits, details)
			assert.False(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
			assert.Equal(t, details, result.Details)
		})
	}, testOptions())
}

func TestBasePolicyEnforcer_CreateSuccessResult(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		result := enforcer.createSuccessResult(models.EnforcementPolicyHardLimits)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
	})
}

func TestBasePolicyEnforcer_CreateLimitExceededResult(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		result := enforcer.createLimitExceededResult(models.EnforcementPolicyHardLimits, 150, 200)
		assert.False(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonLimitExceeded, result.Reason)
		assert.Equal(t, uint64(150), result.Details.CurrentUsage)
		assert.NotNil(t, result.Details.Limit)
		assert.Equal(t, uint64(200), *result.Details.Limit)
		assert.Equal(t, models.EnforcementPolicyHardLimits, result.Details.Policy)
	}, testOptions())
}

func TestBasePolicyEnforcer_CreateWarningResult(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		enforcer := NewBasePolicyEnforcer(ctx)

		result := enforcer.createWarningResult(models.EnforcementPolicyThreshold, 150, 120, 200)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.Reason)
		assert.Equal(t, uint64(150), result.Details.CurrentUsage)
		assert.NotNil(t, result.Details.Threshold)
		assert.Equal(t, uint64(120), *result.Details.Threshold)
		assert.NotNil(t, result.Details.Limit)
		assert.Equal(t, uint64(200), *result.Details.Limit)
		assert.Equal(t, models.EnforcementPolicyThreshold, result.Details.Policy)
	}, testOptions())
}
