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
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.ReservationID)

		reservationID := *result.ReservationID

		// Verify reservation is pending
		var reservation pluginModels.QuotaReservation
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())
		assert.Nil(t, reservation.UploadID)

		// Handle upload completed event with reservation
		err = service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationID, true)
		require.NoError(t, err)

		// Verify reservation was committed
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsCommitted())
		assert.NotNil(t, reservation.UploadID)
		assert.Equal(t, uploadID, *reservation.UploadID)

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
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.ReservationID)

		reservationID := *result.ReservationID

		// Verify reservation is pending
		var reservation pluginModels.QuotaReservation
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())

		// Handle upload completed event with failure (successful=false)
		// The caller is expected to release the reservation themselves
		err = service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationID, false)
		require.NoError(t, err)

		// Verify reservation was NOT committed (still pending)
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())
		assert.Nil(t, reservation.UploadID)

		// Verify usage detail was NOT created
		var usageDetails []pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).Find(&usageDetails).Error
		require.NoError(t, err)
		assert.Len(t, usageDetails, 0)

		// Release the reservation
		err = result.ReleaseReservation(ctx)
		require.NoError(t, err)

		// Verify reservation was rolled back
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsRolledBack())

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

		// Verify no reservation was created
		var count int64
		err = ctx.DB().Model(&pluginModels.QuotaReservation{}).Where("user_id = ?", userID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

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
		result, err := service.CheckDownloadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.ReservationID)

		reservationID := *result.ReservationID

		// Verify reservation is pending
		var reservation pluginModels.QuotaReservation
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())
		assert.Nil(t, reservation.UploadID)

		// Handle download completed event with reservation
		err = service.handleDownloadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationID, true)
		require.NoError(t, err)

		// Verify reservation was committed
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsCommitted())
		assert.NotNil(t, reservation.UploadID)
		assert.Equal(t, uploadID, *reservation.UploadID)

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

		// Verify no reservation was created
		var count int64
		err = ctx.DB().Model(&pluginModels.QuotaReservation{}).Where("user_id = ?", userID).Count(&count).Error
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)

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
		ip := "192.168.1.1"

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

		result1, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result1.Allowed)
		require.NotNil(t, result1.ReservationID)

		// Create second reservation (500 bytes) - should succeed
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(500)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result2, err := service.CheckUploadQuota(ctx, userID, 500, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result2.Allowed)
		require.NotNil(t, result2.ReservationID)

		// Try to create third reservation (1000 bytes) - should fail
		// Total: 1000 + 500 + 1000 = 2500 > limit of 2000
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: false,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded),
		}, nil).Once()

		result3, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.False(t, result3.Allowed)
		assert.Equal(t, pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded), result3.Reason)

		// Release first reservation
		err = result1.ReleaseReservation(ctx)
		require.NoError(t, err)

		// Now third reservation should succeed (only 500 pending)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1000)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result4, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result4.Allowed)
		require.NotNil(t, result4.ReservationID)

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
		ip := "192.168.1.1"

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

		result1, err := service.CheckUploadQuota(ctx, userID, 1000, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result1.Allowed)
		require.NotNil(t, result1.ReservationID)

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
		assert.Nil(t, result2.ReservationID)

		// But checking WITH reservation option should still count pending
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(1500)).Return(pluginCore.QuotaCheckResult{
			Allowed: false,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded),
		}, nil).Once()

		result3, err := service.CheckUploadQuota(ctx, userID, 1500, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.False(t, result3.Allowed) // 1000 + 1500 = 2500 > 2000
		assert.Equal(t, pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonLimitExceeded), result3.Reason)

		// Clean up
		err = result1.ReleaseReservation(ctx)
		require.NoError(t, err)

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

		// Create a reservation
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.NotNil(t, result.ReservationID)

		reservationID := *result.ReservationID

		// Release the reservation using service.ReleaseReservation
		err = service.ReleaseReservation(ctx, reservationID)
		require.NoError(t, err)

		// Verify reservation was rolled back
		var reservation pluginModels.QuotaReservation
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsRolledBack())

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

		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.ReservationID)

		reservationID := *result.ReservationID

		// Step 2: Verify reservation is pending
		var reservation pluginModels.QuotaReservation
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsPending())

		// Step 3: Handle upload completed event
		err = service.handleUploadCompleted(ctx, uploadID, bytes, ip, &userID, &reservationID, true)
		require.NoError(t, err)

		// Step 4: Verify reservation was committed
		err = ctx.DB().First(&reservation, reservationID).Error
		require.NoError(t, err)
		assert.True(t, reservation.IsCommitted())

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

		result2, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation(ip))
		require.NoError(t, err)
		require.True(t, result2.Allowed)

		dataManager.Cleanup()
	}, testOptions())
}
