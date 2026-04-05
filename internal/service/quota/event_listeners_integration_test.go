package quota

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
)

func TestEventListeners_handleUploadCompleted_WithReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
		bytes := uint64(1000)
		ip := "192.168.1.1"

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		// Create a reservation first
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.Reservation)

		reservationUUID := result.Reservation.UUID()

		// Verify reservation is active
		reservation := service.GetReservationManager().GetReservation(reservationUUID)
		require.NotNil(t, reservation)
		assert.Equal(t, userID, reservation.UserID())
		assert.Equal(t, bytes, uint64(reservation.Bytes()))

		// Handle upload completed event with reservation
		err = service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationUUID, true)
		require.NoError(t, err)

		// Verify reservation was released
		reservation = service.GetReservationManager().GetReservation(reservationUUID)
		assert.Nil(t, reservation, "reservation should be released after upload completes")

		// The event handler now records usage atomically with reservation release
		// The calling code does NOT need to call RecordUpload again

		// Verify usage detail was created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeUpload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)

		dataManager.Cleanup()
	}, testOptions())
}

func TestEventListeners_handleUploadCompleted_WithReservation_Failure(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
		bytes := uint64(1000)
		ip := "192.168.1.1"

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		// Create a reservation first
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.Reservation)

		reservationUUID := result.Reservation.UUID()

		// Verify reservation is active
		reservation := service.GetReservationManager().GetReservation(reservationUUID)
		require.NotNil(t, reservation)
		assert.Equal(t, userID, reservation.UserID())

		// Handle upload completed event with failure (successful=false)
		// Reservation should be released
		err = service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationUUID, false)
		require.NoError(t, err)

		// Verify reservation was released
		reservation = service.GetReservationManager().GetReservation(reservationUUID)
		assert.Nil(t, reservation, "reservation should be released after upload fails")

		// Release the reservation again - should be safe (no-op)
		result.Reservation.Release()

		// Verify usage detail was NOT created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 0)

		dataManager.Cleanup()
	}, testOptions())
}

func TestEventListeners_handleUploadCompleted_WithoutReservation(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
		bytes := uint64(1000)
		ip := "192.168.1.1"

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// No mocking needed since RecordUpload is called directly

		// Handle upload completed event without reservation
		err := service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, nil, true)
		require.NoError(t, err)

		// Verify usage detail was created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeUpload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)

		dataManager.Cleanup()
	}, testOptions())
}

func TestEventListeners_handleDownloadCompleted_WithReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
		bytes := uint64(1000)
		ip := "192.168.1.1"

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckDownloadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		// Create a reservation first
		result, err := service.CheckDownloadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.Reservation)
		reservationUUID := result.Reservation.UUID()

		// Verify reservation exists
		reservation := service.GetReservationManager().GetReservation(reservationUUID)
		require.NotNil(t, reservation)
		assert.Equal(t, userID, reservation.UserID())

		// Handle download completed event with reservation
		err = service.handleDownloadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationUUID, true)
		require.NoError(t, err)

		// Verify reservation was released
		reservation = service.GetReservationManager().GetReservation(reservationUUID)
		assert.Nil(t, reservation, "reservation should be released after download completes")

		// The event handler now records usage atomically with reservation release
		// The calling code does NOT need to call RecordDownload again

		// Verify usage detail was created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)

		dataManager.Cleanup()
	}, testOptions())
}

func TestEventListeners_handleDownloadCompleted_WithoutReservation(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
		bytes := uint64(1000)
		ip := "192.168.1.1"

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// No mocking needed since RecordDownload is called directly

		// Handle download completed event without reservation
		err := service.handleDownloadCompleted(ctx, uploadID, bytes, ip, &userID, nil, true)
		require.NoError(t, err)

		// Verify usage detail was created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)
		assert.Equal(t, pluginModels.UsageTypeDownload, usageDetails[0].Type)
		assert.Equal(t, bytes, usageDetails[0].Bytes)

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_CheckUploadQuota_WithReservation_IncludesPending(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadLimit := int64(2000)
		storageLimit := int64(0)
		downloadLimit := int64(0)

		// Create user quota config with upload limit
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		// Create first reservation (1000 bytes)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result1, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result1.Allowed)
		require.NotNil(t, result1.Reservation)

		// Create second reservation (500 bytes) - should succeed
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(500)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result2, err := service.CheckUploadQuota(ctx, userID, 500, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result2.Allowed)
		require.NotNil(t, result2.Reservation)

		// Try to create third reservation (1000 bytes) - should fail
		// Total: 1000 + 500 + 1000 = 2500 > limit of 2000
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: false,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded),
		}, nil).Once()

		result3, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.False(t, result3.Allowed)
		assert.Equal(t, pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded), result3.Reason)

		// Release first reservation
		result1.ReleaseReservation()

		// Now third reservation should succeed (only 500 pending)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result4, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result4.Allowed)
		require.NotNil(t, result4.Reservation)

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_CheckUploadQuota_WithoutReservation_DoesNotIncludePending(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadLimit := int64(2000)
		storageLimit := int64(0)
		downloadLimit := int64(0)

		// Create user quota config with upload limit
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		// Create first reservation with reservation (1000 bytes)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result1, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result1.Allowed)
		require.NotNil(t, result1.Reservation)

		// Check quota WITHOUT reservation option - should not count pending
		// This allows the check to succeed even though there's a pending reservation
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1500)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result2, err := service.CheckUploadQuota(ctx, userID, 1500)
		require.NoError(t, err)
		require.True(t, result2.Allowed)
		assert.Nil(t, result2.Reservation)

		// But checking WITH reservation option should still count pending
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1500)).Return(pluginCore.QuotaCheckResult{
			Allowed: false,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded),
		}, nil).Once()

		result3, err := service.CheckUploadQuota(ctx, userID, 1500, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.False(t, result3.Allowed) // 1000 + 1500 = 2500 > 2000
		assert.Equal(t, pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded), result3.Reason)

		// Clean up
		result1.ReleaseReservation()

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_ReleaseReservation_ViaManager(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		bytes := uint64(1000)

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		// Create a reservation
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.NotNil(t, result.Reservation)

		reservationUUID := result.Reservation.UUID()

		// Verify reservation exists
		reservation := service.GetReservationManager().GetReservation(reservationUUID)
		require.NotNil(t, reservation)

		// Release the reservation
		reservation.Release()

		// Verify reservation was released (no longer in memory)
		reservation = service.GetReservationManager().GetReservation(reservationUUID)
		assert.Nil(t, reservation, "reservation should be released")

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_ReservationWorkflow_EndToEnd(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
		bytes := uint64(1000)
		ip := "192.168.1.1"

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		// Step 1: Check quota with reservation
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.Reservation)

		reservationUUID := result.Reservation.UUID()

		// Step 2: Verify reservation exists in memory
		reservation := service.GetReservationManager().GetReservation(reservationUUID)
		require.NotNil(t, reservation)

		// Step 3: Handle upload completed event
		err = service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationUUID, true)
		require.NoError(t, err)

		// Step 4: Verify reservation was released
		reservation = service.GetReservationManager().GetReservation(reservationUUID)
		assert.Nil(t, reservation, "reservation should be released after upload completes")

		// The event handler now records usage atomically with reservation release
		// The calling code does NOT need to call RecordUpload again

		// Step 5: Verify usage was recorded
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 1)

		// Step 6: Check that a new quota check works correctly
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result2, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result2.Allowed)

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_ReleaseReservation_SafeNoOp(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Try to get a non-existent reservation - should return nil
		reservation := service.GetReservationManager().GetReservation("non-existent-uuid")
		assert.Nil(t, reservation, "non-existent reservation should return nil")

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_ReservationReleaseOnMultipleCalls(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()

		// Create user quota config
		uploadLimit := int64(0)
		storageLimit := int64(0)
		downloadLimit := int64(0)
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		// Create a reservation
		result, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.NotNil(t, result.Reservation)

		reservationUUID := result.Reservation.UUID()

		// Release multiple times - should be safe
		result.ReleaseReservation()
		result.ReleaseReservation()
		result.ReleaseReservation()

		// Verify reservation is gone
		reservation := service.GetReservationManager().GetReservation(reservationUUID)
		assert.Nil(t, reservation)

		dataManager.Cleanup()
	}, testOptions())
}

// Additional test to verify reservation tracking across operations
func TestQuotaService_ReservationTrackingWithPendingBytes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.GenerateUserID()
		uploadLimit := int64(5000)
		storageLimit := int64(0)
		downloadLimit := int64(0)

		// Create user quota config
		limits := &testdata.TestUserLimits{
			StorageLimitBytes:  &storageLimit,
			UploadLimitBytes:   &uploadLimit,
			DownloadLimitBytes: &downloadLimit,
		}
		dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, limits)

		// Mock config manager
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)
		service.configManager = mockConfigManager

		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		// Create multiple reservations
		for i := 0; i < 5; i++ {
			mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
			mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
			mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(500)).Return(pluginCore.QuotaCheckResult{
				Allowed: true,
				Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
			}, nil).Once()

			result, err := service.CheckUploadQuota(ctx, userID, 500, pluginCore.WithCreateReservation())
			require.NoError(t, err)
			require.True(t, result.Allowed)
			require.NotNil(t, result.Reservation)
		}

		// Verify pending bytes are tracked correctly
		pendingBytes := service.GetReservationManager().SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(2500), pendingBytes, "5 reservations of 500 bytes each = 2500 pending")

		// Try to exceed the limit (2500 + 3000 = 5500 > 5000)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(3000)).Return(pluginCore.QuotaCheckResult{
			Allowed: false,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded),
		}, nil).Once()

		result, err := service.CheckUploadQuota(ctx, userID, 3000, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.False(t, result.Allowed)

		dataManager.Cleanup()
	}, testOptions())
}
