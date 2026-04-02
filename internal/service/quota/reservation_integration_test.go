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

func TestQuotaService_CheckUploadQuota_WithReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)

		dataManager := testdata.NewTestDataManager(ctx)
		defer dataManager.Cleanup()
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

		// Mock config manager and policy enforcer to bypass full quota check flow
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		service.configManager = mockConfigManager

		// Create minimal user config for the mock to return
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil)
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil)
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), bytes).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil)

		// Check quota with reservation
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result.Allowed)
		require.NotNil(t, result.Reservation)

		// Verify reservation was created and has expected properties
		assert.NotEmpty(t, result.Reservation.UUID())
		assert.Equal(t, userID, result.Reservation.UserID())
		assert.Equal(t, pluginCore.UsageTypeUpload, result.Reservation.UsageType())
		assert.Equal(t, int64(bytes), result.Reservation.Bytes())

		// Verify reservation is pending and can be retrieved
		retrievedReservation := service.reservationManager.GetReservation(result.Reservation.UUID())
		assert.NotNil(t, retrievedReservation)
		assert.Equal(t, result.Reservation.UUID(), retrievedReservation.UUID())

		// Release the reservation
		result.Reservation.Release()

		// After release, reservation should still be retrievable (released in memory)
		// but the pending bytes sum should be reduced to 0
		pendingBytes := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(0), pendingBytes)

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_CheckUploadQuota_IncludesPendingReservations(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)
		dataManager := testdata.NewTestDataManager(ctx)
		defer dataManager.Cleanup()
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

		// Mock config manager and policy enforcer
		mockConfigManager := pluginCore.NewMockConfigManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		service.configManager = mockConfigManager

		// Create minimal user config
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		// First check quota with reservation (500 bytes)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, userID).Return(userConfig, nil).Once()
		mockConfigManager.EXPECT().GetPolicyEnforcer(mock.Anything, userID).Return(mockPolicyEnforcer, nil).Once()
		mockPolicyEnforcer.EXPECT().CheckUploadQuota(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), uint64(500)).Return(pluginCore.QuotaCheckResult{
			Allowed: true,
			Reason:  pluginCore.QuotaCheckReason(pluginModels.QuotaCheckReasonOK),
		}, nil).Once()

		result1, err := service.CheckUploadQuota(ctx, userID, 500, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.True(t, result1.Allowed)
		require.NotNil(t, result1.Reservation)
		uuid1 := result1.Reservation.UUID()

		// Verify pending bytes after first reservation
		pendingBytes1 := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(500), pendingBytes1)

		// Second check quota with reservation (500 bytes)
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
		uuid2 := result2.Reservation.UUID()

		// Verify total pending bytes
		totalPending := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(1000), totalPending)

		// Verify both reservations exist and can be retrieved
		reservation1 := service.reservationManager.GetReservation(uuid1)
		assert.NotNil(t, reservation1)
		assert.Equal(t, uuid1, reservation1.UUID())

		reservation2 := service.reservationManager.GetReservation(uuid2)
		assert.NotNil(t, reservation2)
		assert.Equal(t, uuid2, reservation2.UUID())

		// Release one reservation and verify pending bytes decreased
		reservation1.Release()
		totalPendingAfterRelease := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(500), totalPendingAfterRelease)

		// Release the other reservation
		reservation2.Release()
	}, testOptions())
}

func TestQuotaService_RecordUsage_WithReservation(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)
		dataManager := testdata.NewTestDataManager(ctx)
		defer dataManager.Cleanup()
		userID := dataManager.GenerateUserID()
		uploadID := dataManager.GenerateUploadID()
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

		// Check quota with reservation
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.NotNil(t, result.Reservation)

		// Verify reservation is pending
		uuid := result.Reservation.UUID()
		pendingBefore := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(bytes), pendingBefore)

		result.Reservation.Release()
		pendingAfterRelease := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(0), pendingAfterRelease)

		// Record usage separately from reservation release
		err = service.RecordUpload(ctx, userID, uploadID, bytes, "")
		require.NoError(t, err)

		// Verify usage detail was created
		var usageDetail pluginModels.UserUsageDetail
		err = ctx.DB().Where("user_id = ? AND upload_id = ?", userID, uploadID).First(&usageDetail).Error
		require.NoError(t, err)
		assert.Equal(t, bytes, usageDetail.Bytes)
		assert.Equal(t, pluginModels.UsageTypeUpload, usageDetail.Type)

		// Reservation is cleaned up after release
		reservation := service.reservationManager.GetReservation(uuid)
		assert.Nil(t, reservation)

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_ReleaseReservation_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)
		dataManager := testdata.NewTestDataManager(ctx)
		defer dataManager.Cleanup()
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

		// Check quota with reservation
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.NotNil(t, result.Reservation)


		// Verify reservation is pending
		pendingBefore := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(bytes), pendingBefore)

		// Release the reservation
		result.Reservation.Release()

		// Verify pending bytes decreased after release
		pendingAfter := service.reservationManager.SumPendingBytesForUser(ctx, userID, pluginCore.UsageTypeUpload)
		assert.Equal(t, int64(0), pendingAfter)

		// Calling release multiple times should be safe (no panic, no error)
		result.Reservation.Release()

		dataManager.Cleanup()
	}, testOptions())
}

func TestQuotaService_Reservation_GetByID(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Get quota service from context
		service := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE).(*QuotaServiceDefault)
		dataManager := testdata.NewTestDataManager(ctx)
		defer dataManager.Cleanup()
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

		// Check quota with reservation
		result, err := service.CheckUploadQuota(ctx, userID, bytes, pluginCore.WithCreateReservation())
		require.NoError(t, err)
		require.NotNil(t, result.Reservation)

		uuid := result.Reservation.UUID()

		// Get reservation by UUID
		reservation := service.reservationManager.GetReservation(uuid)
		require.NotNil(t, reservation)
		assert.Equal(t, uuid, reservation.UUID())
		assert.Equal(t, userID, reservation.UserID())
		assert.Equal(t, pluginCore.UsageTypeUpload, reservation.UsageType())
		assert.Equal(t, int64(bytes), reservation.Bytes())

		// Try to get non-existent reservation
		nonExistent := service.reservationManager.GetReservation("non-existent-uuid")
		assert.Nil(t, nonExistent)

		dataManager.Cleanup()
	}, testOptions())
}
