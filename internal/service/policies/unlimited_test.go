package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// unlimitedTestSetup holds common test setup components
type unlimitedTestSetup struct {
	ctx              core.Context
	mockQuotaService *pluginCore.MockQuotaService
	mockUsageManager *pluginCore.MockUsageManager
	enforcer         *UnlimitedPolicyEnforcer
	dataManager      *testdata.TestDataManager
}

// setupUnlimitedTest creates a new test setup with mocked dependencies
func setupUnlimitedTest(t *testing.T) *unlimitedTestSetup {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)

	// Setup mock expectations
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

	enforcer := NewUnlimitedPolicyEnforcer(ctx, mockQuotaService)

	t.Cleanup(func() {
		dataManager.Cleanup()
	})

	return &unlimitedTestSetup{
		ctx:              ctx,
		mockQuotaService: mockQuotaService,
		mockUsageManager: mockUsageManager,
		enforcer:         enforcer,
		dataManager:      dataManager,
	}
}

// TestUnlimitedPolicyEnforcer_CheckQuotaMethods tests all quota checking methods
func TestUnlimitedPolicyEnforcer_CheckQuotaMethods_AllAllowed(t *testing.T) {
	setup := setupUnlimitedTest(t)

	config := &models.UserQuotaConfig{
		UserID:            setup.dataManager.NextUserID(),
		EnforcementPolicy: models.EnforcementPolicyUnlimited,
	}

	t.Run("CheckUploadQuota", func(t *testing.T) {
		result, err := setup.enforcer.CheckUploadQuota(setup.ctx, config, uint64(1000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, models.EnforcementPolicyUnlimited, result.Details.Policy)
	})

	t.Run("CheckDownloadQuota", func(t *testing.T) {
		result, err := setup.enforcer.CheckDownloadQuota(setup.ctx, config, uint64(1000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, models.EnforcementPolicyUnlimited, result.Details.Policy)
	})

	t.Run("CheckStorageQuota", func(t *testing.T) {
		result, err := setup.enforcer.CheckStorageQuota(setup.ctx, config, uint64(1000))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
		assert.Equal(t, models.EnforcementPolicyUnlimited, result.Details.Policy)
	})

}

// TestUnlimitedPolicyEnforcer_RecordUpload tests the RecordUpload method
func TestUnlimitedPolicyEnforcer_RecordUpload_Success(t *testing.T) {
	setup := setupUnlimitedTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := uint64(500)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := setup.enforcer.RecordUpload(setup.ctx, userID, uploadID, bytes, ip)
	assert.NoError(t, err)
	setup.mockUsageManager.AssertCalled(t, "RecordUpload", mock.Anything, userID, uploadID, bytes, ip)
}

// TestUnlimitedPolicyEnforcer_RecordDownload tests the RecordDownload method
func TestUnlimitedPolicyEnforcer_RecordDownload_Success(t *testing.T) {
	setup := setupUnlimitedTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := uint64(500)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordDownload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := setup.enforcer.RecordDownload(setup.ctx, userID, uploadID, bytes, ip)
	assert.NoError(t, err)
	setup.mockUsageManager.AssertCalled(t, "RecordDownload", mock.Anything, userID, uploadID, bytes, ip)
}

// TestUnlimitedPolicyEnforcer_RecordStorageChange tests the RecordStorageChange method
func TestUnlimitedPolicyEnforcer_RecordStorageChange_Success(t *testing.T) {
	setup := setupUnlimitedTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := int64(500)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordStorageChange(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := setup.enforcer.RecordStorageChange(setup.ctx, userID, uploadID, bytes, ip)
	assert.NoError(t, err)
	setup.mockUsageManager.AssertCalled(t, "RecordStorageChange", mock.Anything, userID, uploadID, bytes, ip)
}

// TestUnlimitedPolicyEnforcer_UsageMethods tests usage-related methods
func TestUnlimitedPolicyEnforcer_UsageMethods_Success(t *testing.T) {
	setup := setupUnlimitedTest(t)

	t.Run("GetDetailedUsage", func(t *testing.T) {
		userID := setup.dataManager.NextUserID()
		start := time.Now().UTC().Add(-time.Hour)
		end := time.Now().UTC().Add(time.Hour)
		earlierTime := time.Now().UTC().Add(-30 * time.Minute)
		laterTime := time.Now().UTC()

		// Set up mock expectations with chronological order
		expectedDetails := []*models.UserUsageDetail{
			{
				UserID:    userID,
				UploadID:  setup.dataManager.NextUploadID(),
				Type:      models.UsageTypeDownload,
				Bytes:     200,
				IP:        "192.168.1.2",
				Timestamp: earlierTime,
			},
			{
				UserID:    userID,
				UploadID:  setup.dataManager.NextUploadID(),
				Type:      models.UsageTypeUpload,
				Bytes:     100,
				IP:        "192.168.1.1",
				Timestamp: laterTime,
			},
		}

		setup.mockUsageManager.EXPECT().GetDetailedUsage(mock.Anything, userID, start, end).Return(expectedDetails, nil)

		details, err := setup.enforcer.GetDetailedUsage(setup.ctx, userID, start, end)
		assert.NoError(t, err)
		assert.Len(t, details, 2)

		// Verify the records are returned in ascending order by timestamp
		assert.True(t, details[0].Timestamp.Before(details[1].Timestamp) || details[0].Timestamp.Equal(details[1].Timestamp))
	})

	t.Run("GetCurrentUsage", func(t *testing.T) {
		userID := setup.dataManager.NextUserID()

		// Set up mock expectations
		expectedUsage := &pluginCore.Usage{
			UserID:          userID,
			BytesUploaded:   100,
			BytesDownloaded: 200,
			BytesStored:     300,
			LastUpdated:     time.Now(),
		}

		setup.mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, userID).Return(expectedUsage, nil)

		usage, err := setup.enforcer.GetCurrentUsage(setup.ctx, userID)
		assert.NoError(t, err)
		assert.Equal(t, userID, usage.UserID)
		assert.Equal(t, uint64(100), usage.BytesUploaded)
		assert.Equal(t, uint64(200), usage.BytesDownloaded)
		assert.Equal(t, uint64(300), usage.BytesStored)
	})

	t.Run("GetUsageHistory", func(t *testing.T) {
		userID := setup.dataManager.NextUserID()
		period := 7
		usageType := models.UsageTypeUpload

		// Set up mock expectations
		expectedHistory := []*pluginCore.UsagePoint{
			{
				Date:  time.Now().Add(-24 * time.Hour),
				Bytes: 100,
				Type:  models.UsageTypeUpload,
			},
			{
				Date:  time.Now(),
				Bytes: 200,
				Type:  models.UsageTypeUpload,
			},
		}

		setup.mockUsageManager.EXPECT().GetUsageHistory(mock.Anything, userID, period, usageType).Return(expectedHistory, nil)

		history, err := setup.enforcer.GetUsageHistory(setup.ctx, userID, period, usageType)
		assert.NoError(t, err)
		assert.Len(t, history, 2)
	})

}
