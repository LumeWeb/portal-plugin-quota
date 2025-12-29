package policies

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// TestAllowancePolicyEnforcer_CheckUploadQuota_SufficientAllowance_Integration_Allowed tests the CheckUploadQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckUploadQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

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

	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
	mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeUpload).Return(grants, nil)
	mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1000))

	result, err := enforcer.CheckUploadQuota(ctx.GetContext(), config, 500)
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
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

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

	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
	mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeUpload).Return(grants, nil)
	mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

	result, err := enforcer.CheckUploadQuota(ctx.GetContext(), config, 500)
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
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

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
			Bytes:          2000,
			BytesUsed:      500,
			BytesRemaining: 1500,
			IsActive:       true,
		},
	}

	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
	mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeDownload).Return(grants, nil)
	mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(1500))

	result, err := enforcer.CheckDownloadQuota(ctx.GetContext(), config, 1000)
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
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

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
			BytesUsed:      50,
			BytesRemaining: 50,
			IsActive:       true,
		},
	}

	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
	mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeDownload).Return(grants, nil)
	mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(50))

	result, err := enforcer.CheckDownloadQuota(ctx.GetContext(), config, 100)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(50), *result.Details.Allowance)
	assert.Equal(t, uint64(50), *result.Details.AllowanceUsed)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_CheckStorageQuota_SufficientAllowance_Integration_Allowed tests the CheckStorageQuota method with sufficient allowance
func TestAllowancePolicyEnforcer_CheckStorageQuota_SufficientAllowance_Integration_Allowed(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

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
			Source:         models.GrantSourceSubscription,
			Bytes:          10000,
			BytesUsed:      2000,
			BytesRemaining: 8000,
			IsActive:       true,
		},
	}

	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
	mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeStorage).Return(grants, nil)
	mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(8000))

	result, err := enforcer.CheckStorageQuota(ctx.GetContext(), config, 5000)
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
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

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
			Source:         models.GrantSourceSubscription,
			Bytes:          1000,
			BytesUsed:      900,
			BytesRemaining: 100,
			IsActive:       true,
		},
	}

	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)
	mockGrantManager.EXPECT().GetActiveGrantsByType(mock.Anything, userID, models.GrantTypeStorage).Return(grants, nil)
	mockGrantManager.EXPECT().CalculateAvailableBytes(grants).Return(uint64(100))

	result, err := enforcer.CheckStorageQuota(ctx.GetContext(), config, 200)
	require.NoError(t, err)
	assert.False(t, result.Allowed)
	assert.Equal(t, models.QuotaCheckReasonAllowanceDepleted, result.Reason)
	assert.Equal(t, models.EnforcementPolicyAllowance, result.Details.Policy)
	assert.Equal(t, uint64(100), *result.Details.Allowance)
	assert.Equal(t, uint64(900), *result.Details.AllowanceUsed)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordUpload_Success tests the RecordUpload method
func TestAllowancePolicyEnforcer_RecordUpload_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)

	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(1000)
	ip := "192.168.1.1"

	mockUsageManager.EXPECT().RecordUserUsageDetail(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail")).Return(nil)
	consumptions := []*models.AllowanceConsumption{}
	mockGrantManager.EXPECT().ConsumeFromGrants(mock.Anything, userID, models.GrantTypeUpload, bytes, mock.AnythingOfType("uint"), (*gorm.DB)(nil)).RunAndReturn(
		func(ctx context.Context, userID uint, grantType models.GrantType, bytes uint64, usageDetailID uint, tx *gorm.DB) ([]*models.AllowanceConsumption, error) {
			return consumptions, nil
		}).Return(consumptions, nil)
	mockUsageManager.EXPECT().RecordUpload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := enforcer.RecordUpload(ctx.GetContext(), userID, uploadID, bytes, ip)
	require.NoError(t, err)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordUpload_InsufficientAllowance tests the RecordUpload method with insufficient allowance
func TestAllowancePolicyEnforcer_RecordUpload_InsufficientAllowance(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)

	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(1000)
	ip := "192.168.1.1"

	mockUsageManager.EXPECT().RecordUserUsageDetail(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail")).Return(nil)
	mockGrantManager.EXPECT().ConsumeFromGrants(mock.Anything, userID, models.GrantTypeUpload, bytes, mock.AnythingOfType("uint"), (*gorm.DB)(nil)).Return(nil, models.ErrInsufficientAllowance)

	err := enforcer.RecordUpload(ctx.GetContext(), userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient upload allowance")

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordDownload_Success tests the RecordDownload method
func TestAllowancePolicyEnforcer_RecordDownload_Success(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)

	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(500)
	ip := "192.168.1.1"

	mockUsageManager.EXPECT().RecordUserUsageDetail(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail")).Return(nil)
	consumptions := []*models.AllowanceConsumption{}
	mockGrantManager.EXPECT().ConsumeFromGrants(mock.Anything, userID, models.GrantTypeDownload, bytes, mock.AnythingOfType("uint"), (*gorm.DB)(nil)).RunAndReturn(
		func(ctx context.Context, userID uint, grantType models.GrantType, bytes uint64, usageDetailID uint, tx *gorm.DB) ([]*models.AllowanceConsumption, error) {
			return consumptions, nil
		}).Return(consumptions, nil)
	mockUsageManager.EXPECT().RecordDownload(mock.Anything, userID, uploadID, bytes, ip).Return(nil)

	err := enforcer.RecordDownload(ctx.GetContext(), userID, uploadID, bytes, ip)
	require.NoError(t, err)

	dataManager.Cleanup()
}

// TestAllowancePolicyEnforcer_RecordDownload_InsufficientAllowance tests the RecordDownload method with insufficient allowance
func TestAllowancePolicyEnforcer_RecordDownload_InsufficientAllowance(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	dataManager := testdata.NewTestDataManager(ctx)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockUsageManager := pluginCore.NewMockUsageManager(t)
	mockQuotaService.EXPECT().GetUsageManager().Return(mockUsageManager)

	mockGrantManager := pluginCore.NewMockGrantManager(t)
	mockQuotaService.EXPECT().GetGrantManager().Return(mockGrantManager)

	enforcer := NewAllowancePolicyEnforcer(ctx, mockQuotaService)

	userID := dataManager.NextUserID()
	uploadID := dataManager.NextUploadID()
	bytes := uint64(500)
	ip := "192.168.1.1"

	mockUsageManager.EXPECT().RecordUserUsageDetail(mock.Anything, mock.AnythingOfType("*models.UserUsageDetail")).Return(nil)
	mockGrantManager.EXPECT().ConsumeFromGrants(mock.Anything, userID, models.GrantTypeDownload, bytes, mock.AnythingOfType("uint"), (*gorm.DB)(nil)).Return(nil, models.ErrInsufficientAllowance)

	err := enforcer.RecordDownload(ctx.GetContext(), userID, uploadID, bytes, ip)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient download allowance")

	dataManager.Cleanup()
}
