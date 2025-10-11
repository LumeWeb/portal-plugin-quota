package policies

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestAllowancePolicyEnforcer_CheckUploadQuota tests the CheckUploadQuota method
func TestAllowancePolicyEnforcer_CheckUploadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		t.Run("Sufficient allowance", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			grants := []*models.AllowanceGrant{
				createTestAllowanceGrant(t, ctx, userID, models.GrantTypeUpload, 1000),
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(userID, models.GrantTypeUpload).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1000))

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with sufficient allowance
			_, err = enforcer.CheckUploadQuota(config, 500)
			require.NoError(t, err)
		})

		t.Run("Insufficient allowance", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			grants := []*models.AllowanceGrant{
				createTestAllowanceGrant(t, ctx, userID, models.GrantTypeUpload, 100),
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(userID, models.GrantTypeUpload).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with insufficient allowance
			result, err := enforcer.CheckUploadQuota(config, 500)
			require.NoError(t, err)

			assert.False(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
			assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance), result.Details.Policy)
			assert.Equal(t, uint64(100), *result.Details.Allowance)
			assert.Equal(t, uint64(0), *result.Details.AllowanceUsed) // No bytes used yet
		})

		t.Run("Invalid bytes requested", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with zero bytes
			_, err = enforcer.CheckUploadQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

// TestAllowancePolicyEnforcer_CheckDownloadQuota tests the CheckDownloadQuota method
func TestAllowancePolicyEnforcer_CheckDownloadQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		t.Run("Sufficient allowance", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			grants := []*models.AllowanceGrant{
				createTestAllowanceGrant(t, ctx, userID, models.GrantTypeDownload, 1000),
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(userID, models.GrantTypeDownload).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1000))

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with sufficient allowance
			_, err = enforcer.CheckDownloadQuota(config, 500)
			require.NoError(t, err)
		})

		t.Run("Insufficient allowance", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			grants := []*models.AllowanceGrant{
				createTestAllowanceGrant(t, ctx, userID, models.GrantTypeDownload, 100),
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(userID, models.GrantTypeDownload).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with insufficient allowance
			result, err := enforcer.CheckDownloadQuota(config, 500)
			require.NoError(t, err)

			assert.False(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
			assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance), result.Details.Policy)
			assert.Equal(t, uint64(100), *result.Details.Allowance)
			assert.Equal(t, uint64(0), *result.Details.AllowanceUsed) // No bytes used yet
		})

		t.Run("Invalid bytes requested", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with zero bytes
			_, err = enforcer.CheckDownloadQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

// TestAllowancePolicyEnforcer_CheckStorageQuota tests the CheckStorageQuota method
func TestAllowancePolicyEnforcer_CheckStorageQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		t.Run("Sufficient allowance", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			grants := []*models.AllowanceGrant{
				createTestAllowanceGrant(t, ctx, userID, models.GrantTypeStorage, 1000),
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(userID, models.GrantTypeStorage).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1000))

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with sufficient allowance
			_, err = enforcer.CheckStorageQuota(config, 500)
			require.NoError(t, err)
		})

		t.Run("Insufficient allowance", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			grants := []*models.AllowanceGrant{
				createTestAllowanceGrant(t, ctx, userID, models.GrantTypeStorage, 100),
			}

			mockGrantManager.EXPECT().GetActiveGrantsByType(userID, models.GrantTypeStorage).Return(grants, nil)
			mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with insufficient allowance
			result, err := enforcer.CheckStorageQuota(config, 500)
			require.NoError(t, err)

			assert.False(t, result.Allowed)
			assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
			assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyAllowance), result.Details.Policy)
			assert.Equal(t, uint64(100), *result.Details.Allowance)
			assert.Equal(t, uint64(0), *result.Details.AllowanceUsed) // No bytes used yet
		})

		t.Run("Invalid bytes requested", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Get user config
			config, err := enforcer.getUserQuotaConfig(userID)
			require.NoError(t, err)

			// Check quota with zero bytes
			_, err = enforcer.CheckStorageQuota(config, 0)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

// TestAllowancePolicyEnforcer_RecordUpload tests the RecordUpload method
func TestAllowancePolicyEnforcer_RecordUpload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		uploadID := uint(1)
		ip := "192.168.1.1"
		bytes := uint64(100)

		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		t.Run("Successful upload recording", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeUpload, bytes).Return([]*models.AllowanceConsumption{}, nil)

			// Record upload
			err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
			assert.NoError(t, err)

			// Verify usage was recorded in database
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeUpload, usageDetails[0].Type)
			assert.Equal(t, bytes, usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)
		})

		t.Run("Grant consumption failure", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations for failure
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeUpload, bytes).Return(nil, assert.AnError)

			// Record upload should fail
			err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to consume upload allowance")
		})

		t.Run("Invalid user ID", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			err := enforcer.RecordUpload(0, uploadID, bytes, ip)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			err := enforcer.RecordUpload(userID, uploadID, 0, ip)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

// TestAllowancePolicyEnforcer_RecordDownload tests the RecordDownload method
func TestAllowancePolicyEnforcer_RecordDownload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		uploadID := uint(1)
		ip := "192.168.1.1"
		bytes := uint64(100)

		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		t.Run("Successful download recording", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeDownload, bytes).Return([]*models.AllowanceConsumption{}, nil)

			// Record download
			err := enforcer.RecordDownload(userID, uploadID, bytes, ip)
			assert.NoError(t, err)

			// Verify usage was recorded in database
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeDownload, usageDetails[0].Type)
			assert.Equal(t, bytes, usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)
		})

		t.Run("Grant consumption failure", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations for failure
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeDownload, bytes).Return(nil, assert.AnError)

			// Record download should fail
			err := enforcer.RecordDownload(userID, uploadID, bytes, ip)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to consume download allowance")
		})

		t.Run("Invalid user ID", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			err := enforcer.RecordDownload(0, uploadID, bytes, ip)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Invalid bytes", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			err := enforcer.RecordDownload(userID, uploadID, 0, ip)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

// TestAllowancePolicyEnforcer_RecordStorageChange tests the RecordStorageChange method
func TestAllowancePolicyEnforcer_RecordStorageChange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		uploadID := uint(1)
		ip := "192.168.1.1"
		bytes := int64(100)

		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		t.Run("Successful storage addition recording", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeStorage, uint64(bytes)).Return([]*models.AllowanceConsumption{}, nil)

			// Record storage change
			err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
			assert.NoError(t, err)

			// Verify usage was recorded in database
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeStorageAdd, usageDetails[0].Type)
			assert.Equal(t, uint64(bytes), usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)
		})

		t.Run("Storage removal (no grant consumption)", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)
			removalBytes := int64(-50)
			removalUploadID := uint(2) // Use different upload ID to avoid conflict

			// Record storage change - should not consume grants for removal
			err := enforcer.RecordStorageChange(userID, removalUploadID, removalBytes, ip)
			assert.NoError(t, err)

			// Verify usage was recorded in database
			var usageDetails []models.UserUsageDetail
			err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, removalUploadID).Find(&usageDetails).Error
			require.NoError(t, err)
			assert.Len(t, usageDetails, 1)
			assert.Equal(t, models.UsageTypeStorageRemove, usageDetails[0].Type) // Storage removal is stored as STORAGE_REMOVE with positive bytes
			assert.Equal(t, uint64(-removalBytes), usageDetails[0].Bytes)
			assert.Equal(t, ip, usageDetails[0].IP)
		})

		t.Run("Grant consumption failure", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations for failure
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeStorage, uint64(bytes)).Return(nil, assert.AnError)

			// Record storage change should fail
			err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to consume storage allowance")
		})

		t.Run("Invalid user ID", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			err := enforcer.RecordStorageChange(0, uploadID, bytes, ip)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidUserID, err)
		})

		t.Run("Zero bytes", func(t *testing.T) {
			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			err := enforcer.RecordStorageChange(userID, uploadID, 0, ip)
			assert.Error(t, err)
			assert.Equal(t, models.ErrInvalidBytes, err)
		})
	}, testOptions())
}

// TestAllowancePolicyEnforcer_GetDetailedUsage tests the GetDetailedUsage method
func TestAllowancePolicyEnforcer_GetDetailedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		// Create some usage records
		createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)
		createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 200)

		mockGrantManager := createMockGrantManager(t)
		enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

		start := time.Now().Add(-time.Hour)
		end := time.Now().Add(time.Hour)

		usageDetails, err := enforcer.GetDetailedUsage(userID, start, end)
		assert.NoError(t, err)
		assert.Len(t, usageDetails, 2)

		// Verify the records are returned in descending order by timestamp
		assert.True(t, usageDetails[0].Timestamp.After(usageDetails[1].Timestamp) || usageDetails[0].Timestamp.Equal(usageDetails[1].Timestamp))
	}, testOptions())
}

// TestAllowancePolicyEnforcer_GetCurrentUsage tests the GetCurrentUsage method
func TestAllowancePolicyEnforcer_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent upload recordings", func(t *testing.T) {
		coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
			userID := uint(10000)
			createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

			mockGrantManager := createMockGrantManager(t)
			enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

			// Set up mock expectations for multiple calls
			mockGrantManager.EXPECT().ConsumeFromGrants(userID, models.GrantTypeUpload, uint64(100)).Return([]*models.AllowanceConsumption{}, nil).Times(5)

			// Run concurrent upload recordings
			var errors []error
			var mu sync.Mutex
			var wg sync.WaitGroup

			numGoroutines := 5
			for i := 0; i < numGoroutines; i++ {
				wg.Add(1)
				go func(goroutineID int) {
					defer wg.Done()
					err := enforcer.RecordUpload(userID, uint(goroutineID+1), 100, "192.168.1.1")
					mu.Lock()
					errors = append(errors, err)
					mu.Unlock()
				}(i)
			}

			wg.Wait()

			// All should succeed
			for _, err := range errors {
				assert.NoError(t, err)
			}
		}, testOptions())
	})
}

func TestAllowancePolicyEnforcer_GetCurrentUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		// Create some usage records
		createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)
		createTestUsageRecord(t, ctx, userID, models.UsageTypeDownload, 200)
		createTestUsageRecord(t, ctx, userID, models.UsageTypeStorageAdd, 300)

		mockGrantManager := createMockGrantManager(t)
		enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

		usage, err := enforcer.GetCurrentUsage(userID)
		assert.NoError(t, err)
		assert.Equal(t, userID, usage.UserID)
		assert.Equal(t, uint64(100), usage.BytesUploaded)
		assert.Equal(t, uint64(200), usage.BytesDownloaded)
		assert.Equal(t, uint64(300), usage.BytesStored)
	}, testOptions())
}
