package managers

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	portalMocks "go.lumeweb.com/portal/core/testing/mocks"
	"go.lumeweb.com/portal/db/models"
)

// Test options for different configurations
func testOptionsWithSharedUsageDisabled() coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    false,
		SharedUsagePrecision: 0,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

func testOptionsWithSharedUsageEnabled() coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    true,
		SharedUsagePrecision: 0,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

func testOptionsWithSharedUsagePrecision(precision int) coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    true,
		SharedUsagePrecision: precision,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

// Test options with precision 0 for backward compatibility with existing tests
func testOptionsWithPrecision0() coreTesting.TestContextBuilderOption {
	quotaCfg := &config.QuotaConfig{
		EnableSharedUsage:    true,
		SharedUsagePrecision: 0,
	}
	return pluginTesting.TestOptionsWithConfig(quotaCfg)
}

// TestUsageManager_RecordUpload tests the RecordUpload method
func TestUsageManager_RecordUpload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1000)
		uploadID := uint(100)
		bytes := uint64(500)
		ip := "192.168.1.1"

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordUpload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeUpload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Uploads are not shared

		// Verify the daily quota was updated
		today := time.Now().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, bytes, dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)
	}, testOptionsWithPrecision0())
}

func TestUsageManager_RecordUpload_InvalidUserID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordUpload(0, 1, 100, "192.168.1.1")
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)
	}, testOptionsWithPrecision0())
}

func TestUsageManager_RecordUpload_InvalidBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1001)
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordUpload(userID, 1, 0, "192.168.1.1")
		assert.ErrorIs(t, err, pluginModels.ErrInvalidBytes)
	}, testOptionsWithPrecision0())
}

// TestUsageManager_RecordDownload tests the RecordDownload method
func TestUsageManager_RecordDownload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(2000)
		uploadID := uint(100)
		bytes := uint64(500)
		ip := "192.168.1.1"

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordDownload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Not shared when disabled

		// Verify the daily quota was updated
		today := time.Now().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, bytes, dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordDownload_InvalidUserID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordDownload(0, 1, 100, "192.168.1.1")
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordDownload_InvalidBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(2001)
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordDownload(userID, 1, 0, "192.168.1.1")
		assert.ErrorIs(t, err, pluginModels.ErrInvalidBytes)
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordDownload_WithSharedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(2002)
		uploadID := uint(100)
		bytes := uint64(600)
		ip := "192.168.1.1"

		// Register mock pin service
		mockPinService := portalMocks.NewMockPinService(ctx.T())
		ctx.RegisterService(core.PIN_SERVICE, mockPinService)
		pins := []*models.Pin{
			{UserID: userID},
			{UserID: userID + 1},
			{UserID: userID + 2},
		}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordDownload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded with shared calculation
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, uint64(200), usageDetails[0].Bytes) // 600/3 = 200
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(3), usageDetails[0].SharedWith)

		// Verify the daily quota was updated
		today := time.Now().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(200), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesStored)
	}, testOptionsWithSharedUsageEnabled())
}

func TestUsageManager_RecordDownload_PinServiceError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(2003)
		uploadID := uint(100)
		bytes := uint64(500)
		ip := "192.168.1.1"

		mockPinService := core.GetService[*portalMocks.MockPinService](ctx, core.PIN_SERVICE)
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return([]*models.Pin{}, fmt.Errorf("pin service error"))

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordDownload(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded with fallback individual usage
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes) // Full bytes since fallback
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Fallback to 1
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_RecordStorageChange tests the RecordStorageChange method
func TestUsageManager_RecordStorageChange(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(3000)
		uploadID := uint(100)
		bytes := int64(500)
		ip := "192.168.1.1"

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeStorageAdd, usageDetails[0].Type)
		assert.Equal(t, uint64(bytes), usageDetails[0].Bytes)
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Not shared when disabled

		// Verify the daily quota was updated
		today := time.Now().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(bytes), dailyQuota.BytesStored)
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordStorageChange_InvalidBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(3001)
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(userID, 1, 0, "192.168.1.1")
		assert.ErrorIs(t, err, pluginModels.ErrInvalidBytes)
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordStorageChange_InvalidUserID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(0, 1, 100, "192.168.1.1")
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordStorageChange_WithSharedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(3002)
		uploadID := uint(100)
		bytes := int64(600)
		ip := "192.168.1.1"

		// Storage usage is never shared, so we don't expect the pin service to be called
		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded without shared calculation
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeStorageAdd, usageDetails[0].Type)
		assert.Equal(t, uint64(600), usageDetails[0].Bytes) // Full bytes since storage is never shared
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Always 1 for storage (never shared)

		// Verify the daily quota was updated
		today := time.Now().Truncate(24 * time.Hour)
		var dailyQuota pluginModels.UserQuota
		err = ctx.DB().Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), dailyQuota.BytesUploaded)
		assert.Equal(t, uint64(0), dailyQuota.BytesDownloaded)
		assert.Equal(t, uint64(600), dailyQuota.BytesStored)
	}, testOptionsWithSharedUsageEnabled())
}

func TestUsageManager_RecordStorageChange_Remove(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(3003)
		uploadID := uint(100)
		bytes := int64(-300)
		ip := "192.168.1.1"

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeStorageRemove, usageDetails[0].Type)
		assert.Equal(t, uint64(-bytes), usageDetails[0].Bytes) // Positive bytes in record
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Removals are not shared
	}, testOptionsWithSharedUsageDisabled())
}

func TestUsageManager_RecordStorageChange_RemoveWithSharedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(3004)
		uploadID := uint(100)
		bytes := int64(-500)
		ip := "192.168.1.1"

		usageManager := NewUsageManager(ctx)

		err := usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
		require.NoError(t, err)

		// Verify the usage detail was recorded without shared calculation
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeStorageRemove, usageDetails[0].Type)
		assert.Equal(t, uint64(-bytes), usageDetails[0].Bytes) // Positive bytes in record
		assert.Equal(t, ip, usageDetails[0].IP)
		assert.Equal(t, uint(1), usageDetails[0].SharedWith) // Always 1 for removals
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_calculateSharedUsage tests the calculateSharedUsage method
func TestUsageManager_calculateSharedUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(4001)
		uploadID := uint(100)
		totalBytes := uint64(600)

		var mockPinService = core.GetService[*portalMocks.MockPinService](ctx, core.PIN_SERVICE)
		pins := []*models.Pin{
			{UserID: userID},
			{UserID: userID + 1},
			{UserID: userID + 2},
		}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		require.NoError(t, err)
		assert.Equal(t, uint(3), sharedWith)
		assert.Equal(t, uint64(200), sharedBytes) // 600/3 = 200
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		uploadID := uint(100)
		totalBytes := uint64(500)

		// Simulate pin service unavailability by getting the service and setting it to nil
		// This test now checks that the usage manager properly handles nil pin service
		usageManager := NewUsageManager(ctx)
		usageManager.pinService = nil

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pin service not available")
		assert.Equal(t, uint(1), sharedWith)
		assert.Equal(t, totalBytes, sharedBytes)
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		uploadID := uint(100)
		totalBytes := uint64(500)

		mockPinService := core.GetService[*portalMocks.MockPinService](ctx, core.PIN_SERVICE)
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return([]*models.Pin{}, fmt.Errorf("pin service error"))

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get pins for upload")
		assert.Equal(t, uint(1), sharedWith)
		assert.Equal(t, totalBytes, sharedBytes)
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		uploadID := uint(100)
		totalBytes := uint64(500)

		mockPinService := core.GetService[*portalMocks.MockPinService](ctx, core.PIN_SERVICE)
		pins := []*models.Pin{}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		require.NoError(t, err)
		assert.Equal(t, uint(1), sharedWith) // Default to 1
		assert.Equal(t, totalBytes, sharedBytes)
	}, testOptionsWithSharedUsageEnabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(4003)
		uploadID := uint(100)
		totalBytes := uint64(600)

		mockPinService := core.GetService[*portalMocks.MockPinService](ctx, core.PIN_SERVICE)
		pins := []*models.Pin{
			{UserID: userID},
			{UserID: userID}, // Duplicate
			{UserID: userID + 1},
			{UserID: userID + 1}, // Duplicate
			{UserID: userID + 2},
		}
		mockPinService.On("GetPinsByUploadID", mock.Anything, uploadID).Return(pins, nil)

		usageManager := NewUsageManager(ctx)

		sharedWith, sharedBytes, err := usageManager.calculateSharedUsage(uploadID, totalBytes)
		require.NoError(t, err)
		assert.Equal(t, uint(3), sharedWith)      // Unique users only
		assert.Equal(t, uint64(200), sharedBytes) // 600/3 = 200
	}, testOptionsWithSharedUsageEnabled())
}

// TestUsageManager_calculateSharedBytes tests the calculateSharedBytes method
func TestUsageManager_calculateSharedBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1000)
		userCount := uint(4)

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 0)
		// 1000/4 = 250, exact division
		expected := uint64(250)
		assert.Equal(t, expected, sharedBytes)
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1000)
		userCount := uint(0)

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 0)
		assert.Equal(t, uint64(0), sharedBytes)
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1)
		userCount := uint(10) // More users than bytes

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 0)
		// Exact division: 1/10 = 0
		assert.Equal(t, uint64(0), sharedBytes)
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1000)
		userCount := uint(3)

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 2)
		// 1000/3 = 333.333..., ceil(333.333... * 100) = ceil(33333.333...) = 33334
		assert.Equal(t, uint64(33334), sharedBytes)
	}, testOptionsWithSharedUsagePrecision(2))

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1000)
		userCount := uint(3)

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, -1)
		// With precision clamped to 0: 1000/3 = 333.333..., exact division truncated
		assert.Equal(t, uint64(333), sharedBytes)
	}, testOptionsWithSharedUsagePrecision(-1))

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1000)
		userCount := uint(3)

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 10)
		// 1000/3 = 333.333..., ceil(333.333... * 10^10) = ceil(3333333333333.333...) = 3333333333334
		// But precision is clamped to 10, so we get ceil(333.333... * 10^10) = 3333333333334
		assert.Equal(t, uint64(3333333333334), sharedBytes)
	}, testOptionsWithSharedUsagePrecision(15))

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		totalBytes := uint64(1000)
		userCount := uint(3)

		usageManager := NewUsageManager(ctx)

		sharedBytes := usageManager.calculateSharedBytes(totalBytes, userCount, 1)
		// 1000/3 = 333.333..., ceil(333.333... * 10) = ceil(3333.333...) = 3334
		assert.Equal(t, uint64(3334), sharedBytes)
	}, testOptionsWithSharedUsagePrecision(1))
}

// TestUsageManager_ConcurrentAccess tests concurrent usage recording
func TestUsageManager_ConcurrentAccess(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(5001)
		usageManager := NewUsageManager(ctx)

		// Run concurrent upload recordings
		var errors []error
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := 5
		bytesPerGoroutine := uint64(100)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordUpload(userID, uint(goroutineID+1), bytesPerGoroutine, "192.168.1.1")
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

		// Verify total bytes recorded
		usage, err := usageManager.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, bytesPerGoroutine*uint64(numGoroutines), usage.BytesUploaded)
	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(5002)
		usageManager := NewUsageManager(ctx)

		// Run concurrent download recordings
		var errors []error
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := 5
		bytesPerGoroutine := uint64(200)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordDownload(userID, uint(goroutineID+1), bytesPerGoroutine, "192.168.1.2")
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

		// Verify total bytes recorded
		usage, err := usageManager.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, bytesPerGoroutine*uint64(numGoroutines), usage.BytesDownloaded)
	}, testOptionsWithSharedUsageDisabled())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(5003)
		usageManager := NewUsageManager(ctx)

		// Run concurrent storage addition recordings
		var errors []error
		var mu sync.Mutex
		var wg sync.WaitGroup

		numGoroutines := 5
		bytesPerGoroutine := int64(300)
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				err := usageManager.RecordStorageChange(userID, uint(goroutineID+1), bytesPerGoroutine, "192.168.1.3")
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

		// Verify total bytes recorded
		usage, err := usageManager.GetCurrentUsage(userID)
		require.NoError(t, err)
		assert.Equal(t, uint64(bytesPerGoroutine)*uint64(numGoroutines), usage.BytesStored)
	}, testOptionsWithSharedUsageDisabled())
}

// TestUsageManager_Validation tests validation methods
func TestUsageManager_Validation(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateUserID(1)
		assert.NoError(t, err)
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateUserID(0)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateBytes(100)
		assert.NoError(t, err)
	}, testOptionsWithPrecision0())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		usageManager := NewUsageManager(ctx)

		err := usageManager.validateBytes(0)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidBytes)
	}, testOptionsWithPrecision0())
}
