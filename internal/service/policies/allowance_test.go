package policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestAllowancePolicyEnforcer_CheckUploadQuota_SufficientAllowance_Integration_Allowed tests the CheckUploadQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckUploadQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	// Set up mock expectations
	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		},
	}

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeUpload).Return(grants, nil)
	mockGrantManager.On("CalculateAvailableBytes", grants).Return(uint64(1000))

	result, err := enforcer.CheckUploadQuota(config, 500)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_CheckUploadQuota_InsufficientAllowance_Integration_Blocked tests the CheckUploadQuota method with insufficient allowance
func TestAllowancePolicyEnforcer_CheckUploadQuota_InsufficientAllowance_Integration_Blocked(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	// Set up mock expectations
	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          100,
			BytesUsed:      0,
			BytesRemaining: 100,
			IsActive:       true,
		},
	}

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeUpload).Return(grants, nil)
	mockGrantManager.On("CalculateAvailableBytes", grants).Return(uint64(100))

	result, err := enforcer.CheckUploadQuota(config, 500)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(100), *result.Details.Allowance)
	assert.Equal(t, uint64(0), *result.Details.AllowanceUsed)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_CheckDownloadQuota_SufficientAllowance_Integration_Allowed tests the CheckDownloadQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckDownloadQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	// Set up mock expectations
	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeDownload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		},
	}

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeDownload).Return(grants, nil)
	mockGrantManager.On("CalculateAvailableBytes", grants).Return(uint64(1000))

	result, err := enforcer.CheckDownloadQuota(config, 500)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_CheckDownloadQuota_InsufficientAllowance_Integration_Blocked tests the CheckDownloadQuota method with insufficient allowance
func TestAllowancePolicyEnforcer_CheckDownloadQuota_InsufficientAllowance_Integration_Blocked(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	// Set up mock expectations
	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeDownload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          100,
			BytesUsed:      0,
			BytesRemaining: 100,
			IsActive:       true,
		},
	}

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeDownload).Return(grants, nil)
	mockGrantManager.On("CalculateAvailableBytes", grants).Return(uint64(100))

	result, err := enforcer.CheckDownloadQuota(config, 500)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(100), *result.Details.Allowance)
	assert.Equal(t, uint64(0), *result.Details.AllowanceUsed)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_CheckStorageQuota_SufficientAllowance_Integration_Allowed tests the CheckStorageQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckStorageQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	// Set up mock expectations
	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeStorage,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		},
	}

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeStorage).Return(grants, nil)
	mockGrantManager.On("CalculateAvailableBytes", grants).Return(uint64(1000))

	result, err := enforcer.CheckStorageQuota(config, 500)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_CheckStorageQuota_InsufficientAllowance_Integration_Blocked tests the CheckStorageQuota method with insufficient allowance
func TestAllowancePolicyEnforcer_CheckStorageQuota_InsufficientAllowance_Integration_Blocked(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	// Set up mock expectations
	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeStorage,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          100,
			BytesUsed:      0,
			BytesRemaining: 100,
			IsActive:       true,
		},
	}

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockGrantManager.On("GetActiveGrantsByType", userID, models.GrantTypeStorage).Return(grants, nil)
	mockGrantManager.On("CalculateAvailableBytes", grants).Return(uint64(100))

	result, err := enforcer.CheckStorageQuota(config, 500)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(100), *result.Details.Allowance)
	assert.Equal(t, uint64(0), *result.Details.AllowanceUsed)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordUpload_SuccessfulRecording_Integration_Success tests the RecordUpload method with successful recording
func TestAllowancePolicyEnforcer_RecordUpload_SuccessfulRecording_Integration_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(100)
	ip := "192.168.1.1"

	// Set up mock expectations
	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil)
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeUpload, bytes, uint(1)).Return([]*models.AllowanceConsumption{}, nil)
	mockUsageManager.On("RecordUpload", userID, uploadID, bytes, ip).Return(nil)

	err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
	assert.NoError(t, err)

	// Verify usage manager was called with correct parameters
	mockUsageManager.AssertCalled(t, "RecordUserUsageDetail", mock.Anything)
	mockUsageManager.AssertCalled(t, "RecordUpload", userID, uploadID, bytes, ip)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordUpload_GrantConsumptionFailure_Integration_Error tests the RecordUpload method with grant consumption failure
func TestAllowancePolicyEnforcer_RecordUpload_GrantConsumptionFailure_Integration_Error(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(100)
	ip := "192.168.1.2"

	// Set up mock expectations for failure
	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil)
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeUpload, bytes, uint(1)).Return(nil, assert.AnError)

	err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to consume upload allowance")
	// Ensure no recording happened for the upload itself
	mockUsageManager.AssertNumberOfCalls(t, "RecordUserUsageDetail", 1) // The usage detail should still be recorded
	mockUsageManager.AssertNotCalled(t, "RecordUpload", userID, uploadID, bytes, ip)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordDownload_SuccessfulRecording_Integration_Success tests the RecordDownload method with successful recording
func TestAllowancePolicyEnforcer_RecordDownload_SuccessfulRecording_Integration_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(100)
	ip := "192.168.1.1"

	// Set up mock expectations
	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil)
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeDownload, bytes, uint(1)).Return([]*models.AllowanceConsumption{}, nil)
	mockUsageManager.On("RecordDownload", userID, uploadID, bytes, ip).Return(nil)

	err := enforcer.RecordDownload(userID, uploadID, bytes, ip)
	assert.NoError(t, err)

	// Verify usage manager was called with correct parameters
	mockUsageManager.AssertCalled(t, "RecordUserUsageDetail", mock.Anything)
	mockUsageManager.AssertCalled(t, "RecordDownload", userID, uploadID, bytes, ip)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordDownload_GrantConsumptionFailure_Integration_Error tests the RecordDownload method with grant consumption failure
func TestAllowancePolicyEnforcer_RecordDownload_GrantConsumptionFailure_Integration_Error(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(100)
	ip := "192.168.1.2"

	// Set up mock expectations for failure
	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil)
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeDownload, bytes, uint(1)).Return(nil, assert.AnError)

	err := enforcer.RecordDownload(userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to consume download allowance")
	// Ensure no recording happened for the download itself
	mockUsageManager.AssertNumberOfCalls(t, "RecordUserUsageDetail", 1) // The usage detail should still be recorded
	mockUsageManager.AssertNotCalled(t, "RecordDownload", userID, uploadID, bytes, ip)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordStorageChange_SuccessfulRecording_Integration_Success tests the RecordStorageChange method with successful addition recording
func TestAllowancePolicyEnforcer_RecordStorageChange_SuccessfulRecording_Integration_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := int64(100)
	ip := "192.168.1.1"

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil)
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeStorage, uint64(bytes), uint(1)).Return([]*models.AllowanceConsumption{}, nil)
	mockUsageManager.On("RecordStorageChange", userID, uploadID, bytes, ip).Return(nil)

	err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
	assert.NoError(t, err)

	// Verify usage manager was called with correct parameters
	mockUsageManager.AssertCalled(t, "RecordUserUsageDetail", mock.Anything)
	mockUsageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, bytes, ip)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordStorageChange_Removal_Integration_Success tests the RecordStorageChange method with storage removal
func TestAllowancePolicyEnforcer_RecordStorageChange_Removal_Integration_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := int64(-50)
	ip := "192.168.1.2"

	// For storage removal, we don't consume grants, so only usage manager expectation needed
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Return(nil)
	mockUsageManager.On("RecordStorageChange", userID, uploadID, bytes, ip).Return(nil)

	err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
	assert.NoError(t, err)

	// Verify usage manager was called with correct parameters
	mockUsageManager.AssertCalled(t, "RecordUserUsageDetail", mock.Anything)
	mockUsageManager.AssertCalled(t, "RecordStorageChange", userID, uploadID, bytes, ip)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordStorageChange_GrantConsumptionFailure_Integration_Error tests the RecordStorageChange method with grant consumption failure
func TestAllowancePolicyEnforcer_RecordStorageChange_GrantConsumptionFailure_Integration_Error(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := int64(100)
	ip := "192.168.1.3"

	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil)
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeStorage, uint64(bytes), uint(1)).Return(nil, assert.AnError)

	err := enforcer.RecordStorageChange(userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to consume storage allowance")
	// Ensure no recording happened for the storage change itself
	mockUsageManager.AssertNumberOfCalls(t, "RecordUserUsageDetail", 1) // The usage detail should still be recorded

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordUpload_SufficientAllowance_Unit_Success tests upload recording with sufficient allowance
func TestAllowancePolicyEnforcer_RecordUpload_SufficientAllowance_Unit_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager).Once()

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(100)
	ip := "192.168.1.1"

	// Set up mock expectations
	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil).Once()
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeUpload, bytes, uint(1)).Return([]*models.AllowanceConsumption{}, nil).Once()
	mockUsageManager.On("RecordUpload", userID, uploadID, bytes, ip).Return(nil).Once()

	err := enforcer.RecordUpload(userID, uploadID, bytes, ip)
	assert.NoError(t, err)

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}

// TestAllowancePolicyEnforcer_RecordDownload_SufficientAllowance_Unit_Success tests download recording with sufficient allowance
func TestAllowancePolicyEnforcer_RecordDownload_SufficientAllowance_Unit_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager).Once()

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(100)
	ip := "192.168.1.1"

	// Set up mock expectations
	mockQuotaService.On("GetGrantManager").Return(mockGrantManager)
	mockQuotaService.On("GetUsageManager").Return(mockUsageManager)
	mockUsageManager.On("RecordUserUsageDetail", mock.Anything).Run(func(args mock.Arguments) {
		detail := args.Get(0).(*models.UserUsageDetail)
		detail.ID = 1 // Simulate ID being set by database
	}).Return(nil).Once()
	mockGrantManager.On("ConsumeFromGrants", userID, models.GrantTypeDownload, bytes, uint(1)).Return([]*models.AllowanceConsumption{}, nil).Once()
	mockUsageManager.On("RecordDownload", userID, uploadID, bytes, ip).Return(nil).Once()

	err := enforcer.RecordDownload(userID, uploadID, bytes, ip)
	assert.NoError(t, err)

	t.Cleanup(func() {
		dataManager.Cleanup()
	})
}
