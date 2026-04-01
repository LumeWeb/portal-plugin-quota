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

// allowanceTestSetup holds common test setup components for allowance policy tests
type allowanceTestSetup struct {
	ctx                    coreTesting.TestContext
	mockQuotaService       *pluginCore.MockQuotaService
	mockUsageManager       *pluginCore.MockUsageManager
	mockReservationManager *pluginCore.MockReservationManager
	mockGrantManager       *pluginCore.MockGrantManager
	enforcer               *AllowancePolicyEnforcer
	dataManager            *testdata.TestDataManager
}

// setupAllowanceTest creates a new test setup with mocked dependencies for allowance policy tests
func setupAllowanceTest(t *testing.T) *allowanceTestSetup {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)

	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockReservationManager := pluginCore.NewMockReservationManager(t)
	mockGrantManager := pluginCore.NewMockGrantManager(t)

	// Setup base mock expectations
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)
	mockQuotaService.EXPECT().GetReservationManager().Return(mockReservationManager)

	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	t.Cleanup(func() {
		dataManager.Cleanup()
	})

	return &allowanceTestSetup{
		ctx:                    ctx,
		mockQuotaService:       mockQuotaService,
		mockUsageManager:       mockUsageManager,
		mockReservationManager: mockReservationManager,
		mockGrantManager:       mockGrantManager,
		enforcer:               enforcer,
		dataManager:            dataManager,
	}
}

// TestAllowancePolicyEnforcer_CheckUploadQuota_SufficientAllowance_Integration_Allowed tests the CheckUploadQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckUploadQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

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

	setup.mockQuotaService.EXPECT().GetGrantManager().Return(setup.mockGrantManager)
	setup.mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeUpload).Return(grants, nil)
	setup.mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1000))

	result, err := setup.enforcer.CheckUploadQuota(setup.ctx.GetContext(), config, 500)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
}

// TestAllowancePolicyEnforcer_CheckUploadQuota_InsufficientAllowance_Integration_Blocked tests the CheckUploadQuota method with insufficient allowance
func TestAllowancePolicyEnforcer_CheckUploadQuota_InsufficientAllowance_Integration_Blocked(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

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

	setup.mockQuotaService.EXPECT().GetGrantManager().Return(setup.mockGrantManager)
	setup.mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeUpload).Return(grants, nil)
	setup.mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

	result, err := setup.enforcer.CheckUploadQuota(setup.ctx.GetContext(), config, 500)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(100), *result.Details.Allowance)
	assert.Equal(t, uint64(0), *result.Details.AllowanceUsed)
}

// TestAllowancePolicyEnforcer_CheckDownloadQuota_SufficientAllowance_Integration_Allowed tests the CheckDownloadQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckDownloadQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeDownload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          2000,
			BytesUsed:      500,
			BytesRemaining: 1500,
			IsActive:       true,
		},
	}

	setup.mockQuotaService.EXPECT().GetGrantManager().Return(setup.mockGrantManager)
	setup.mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeDownload).Return(grants, nil)
	setup.mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1500))

	result, err := setup.enforcer.CheckDownloadQuota(setup.ctx.GetContext(), config, 1000)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
}

// TestAllowancePolicyEnforcer_CheckDownloadQuota_InsufficientAllowance_Integration_Blocked tests the CheckDownloadQuota method with insufficient allowance
func TestAllowancePolicyEnforcer_CheckDownloadQuota_InsufficientAllowance_Integration_Blocked(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeDownload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          100,
			BytesUsed:      50,
			BytesRemaining: 50,
			IsActive:       true,
		},
	}

	setup.mockQuotaService.EXPECT().GetGrantManager().Return(setup.mockGrantManager)
	setup.mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeDownload).Return(grants, nil)
	setup.mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(50))

	result, err := setup.enforcer.CheckDownloadQuota(setup.ctx.GetContext(), config, 100)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(50), *result.Details.Allowance)
	assert.Equal(t, uint64(50), *result.Details.AllowanceUsed)
}

// TestAllowancePolicyEnforcer_CheckStorageQuota_SufficientAllowance_Integration_Allowed tests the CheckStorageQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckStorageQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeStorage,
			Source:         models.GrantSourceSubscription,
			Bytes:          10000,
			BytesUsed:      2000,
			BytesRemaining: 8000,
			IsActive:       true,
		},
	}

	setup.mockQuotaService.EXPECT().GetGrantManager().Return(setup.mockGrantManager)
	setup.mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeStorage).Return(grants, nil)
	setup.mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(8000))

	result, err := setup.enforcer.CheckStorageQuota(setup.ctx.GetContext(), config, 5000)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonOK, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
}

// TestAllowancePolicyEnforcer_CheckStorageQuota_InsufficientAllowance_Integration_Blocked tests the CheckStorageQuota method with insufficient allowance
func TestAllowancePolicyEnforcer_CheckStorageQuota_InsufficientAllowance_Integration_Blocked(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	config := &models.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: models.EnforcementPolicyAllowance,
	}

	grants := []*models.AllowanceGrant{
		{
			UserID:         userID,
			Type:           models.GrantTypeStorage,
			Source:         models.GrantSourceSubscription,
			Bytes:          1000,
			BytesUsed:      900,
			BytesRemaining: 100,
			IsActive:       true,
		},
	}

	setup.mockQuotaService.EXPECT().GetGrantManager().Return(setup.mockGrantManager)
	setup.mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeStorage).Return(grants, nil)
	setup.mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

	result, err := setup.enforcer.CheckStorageQuota(setup.ctx.GetContext(), config, 200)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(100), *result.Details.Allowance)
	assert.Equal(t, uint64(900), *result.Details.AllowanceUsed)
}

// TestAllowancePolicyEnforcer_RecordUpload_Success tests the RecordUpload method
func TestAllowancePolicyEnforcer_RecordUpload_Success(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := uint64(1000)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail"), models.GrantTypeUpload, bytes).Return(nil)
	setup.mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := setup.enforcer.RecordUpload(setup.ctx.GetContext(), userID, uploadID, bytes, ip)
	require.NoError(t, err)
}

// TestAllowancePolicyEnforcer_RecordUpload_InsufficientAllowance tests the RecordUpload method with insufficient allowance
func TestAllowancePolicyEnforcer_RecordUpload_InsufficientAllowance(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := uint64(1000)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail"), models.GrantTypeUpload, bytes).Return(models.ErrInsufficientAllowance)

	err := setup.enforcer.RecordUpload(setup.ctx.GetContext(), userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient allowance")
}

// TestAllowancePolicyEnforcer_RecordDownload_Success tests the RecordDownload method
func TestAllowancePolicyEnforcer_RecordDownload_Success(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := uint64(500)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail"), models.GrantTypeDownload, bytes).Return(nil)
	setup.mockUsageManager.EXPECT().RecordDownload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := setup.enforcer.RecordDownload(setup.ctx.GetContext(), userID, uploadID, bytes, ip)
	require.NoError(t, err)
}

// TestAllowancePolicyEnforcer_RecordDownload_InsufficientAllowance tests the RecordDownload method with insufficient allowance
func TestAllowancePolicyEnforcer_RecordDownload_InsufficientAllowance(t *testing.T) {
	setup := setupAllowanceTest(t)

	userID := setup.dataManager.NextUserID()
	uploadID := setup.dataManager.NextUploadID()
	bytes := uint64(500)
	ip := "192.168.1.1"

	setup.mockUsageManager.EXPECT().RecordUsageAndConsume(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail"), models.GrantTypeDownload, bytes).Return(models.ErrInsufficientAllowance)

	err := setup.enforcer.RecordDownload(setup.ctx.GetContext(), userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient allowance")
}
