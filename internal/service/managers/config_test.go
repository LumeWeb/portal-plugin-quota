package managers

import (
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

// TestConfigManager_ResolveEffectiveLimits_UserWithPlan_Success tests resolving effective limits for a user with a plan
func TestConfigManager_ResolveEffectiveLimits_UserWithPlan_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)
		planID := uint64(100)

		// Create user quota config
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
			QuotaPlanID:       lo.ToPtr(planID),
		}

		err := ctx.DB().Create(userConfig).Error
		require.NoError(t, err)

		// Setup mock expectations
		expectedLimits := &pluginCore.EffectiveLimits{
			StorageLimit:       lo.ToPtr(uint64(1000000)),
			UploadDailyLimit:   lo.ToPtr(uint64(500000)),
			DownloadDailyLimit: lo.ToPtr(uint64(500000)),
			UploadTotalLimit:   lo.ToPtr(uint64(10000000)),
			DownloadTotalLimit: lo.ToPtr(uint64(10000000)),
		}

		mockLimitResolver.On("ResolveEffectiveLimits", userConfig, pluginModels.EnforcementPolicyHardLimits).Return(expectedLimits, nil)

		// Test
		limits, err := configManager.ResolveEffectiveLimits(userID)
		require.NoError(t, err)
		assert.Equal(t, expectedLimits, limits)

		mockLimitResolver.AssertExpectations(t)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimits_UserWithoutPlan_Success tests resolving effective limits for a user without a plan
func TestConfigManager_ResolveEffectiveLimits_UserWithoutPlan_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Create user quota config without plan
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
			QuotaPlanID:       nil,
		}

		err := ctx.DB().Create(userConfig).Error
		require.NoError(t, err)

		// Setup mock expectations
		expectedLimits := &pluginCore.EffectiveLimits{
			StorageLimit:       lo.ToPtr(uint64(100000)),
			UploadDailyLimit:   lo.ToPtr(uint64(50000)),
			DownloadDailyLimit: lo.ToPtr(uint64(50000)),
			UploadTotalLimit:   lo.ToPtr(uint64(1000000)),
			DownloadTotalLimit: lo.ToPtr(uint64(1000000)),
		}

		mockLimitResolver.On("ResolveEffectiveLimits", userConfig, pluginModels.EnforcementPolicyHardLimits).Return(expectedLimits, nil)

		// Test
		limits, err := configManager.ResolveEffectiveLimits(userID)
		require.NoError(t, err)
		assert.Equal(t, expectedLimits, limits)

		mockLimitResolver.AssertExpectations(t)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserQuotaConfig_ExistingConfig_Success tests getting existing user quota config
func TestConfigManager_GetUserQuotaConfig_ExistingConfig_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)
		planID := uint64(100)

		// Create user quota config
		expectedConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
			QuotaPlanID:       lo.ToPtr(planID),
		}

		err := ctx.DB().Create(expectedConfig).Error
		require.NoError(t, err)

		// Test
		config, err := configManager.GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.Equal(t, expectedConfig.UserID, config.UserID)
		assert.Equal(t, expectedConfig.EnforcementPolicy, config.EnforcementPolicy)
		assert.Equal(t, expectedConfig.QuotaPlanID, config.QuotaPlanID)

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserQuotaConfig_NonExistentConfig_CreatesDefault tests creating default config when none exists
func TestConfigManager_GetUserQuotaConfig_NonExistentConfig_CreatesDefault(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Setup mock expectation for GetDefaultQuotaPlan - return error to simulate no default plan
		mockPlanManager.On("GetDefaultQuotaPlan").Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Test - should create default config
		cfg, err := configManager.GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.Equal(t, userID, cfg.UserID)
		assert.Equal(t, pluginModels.EnforcementPolicy("HARD_LIMITS"), cfg.EnforcementPolicy)
		assert.Nil(t, cfg.QuotaPlanID)

		// Verify it was saved to DB
		var savedConfig pluginModels.UserQuotaConfig
		err = ctx.DB().Where("user_id = ?", userID).First(&savedConfig).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedConfig.UserID)

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserQuotaConfig_WithDefaultPlan_AssignsPlan tests assigning default plan to new user config
func TestConfigManager_GetUserQuotaConfig_WithDefaultPlan_AssignsPlan(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Setup mock plan manager to return a default plan
		defaultPlan := &pluginModels.QuotaPlan{
			Model: gorm.Model{ID: 1},
			Name:  "default",
		}
		mockPlanManager.On("GetDefaultQuotaPlan").Return(defaultPlan, nil)

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Test - should create default config with plan
		config, err := configManager.GetUserQuotaConfig(userID)
		require.NoError(t, err)
		assert.Equal(t, userID, config.UserID)
		assert.Equal(t, pluginModels.EnforcementPolicyHardLimits, config.EnforcementPolicy)
		assert.NotNil(t, config.QuotaPlanID)
		assert.Equal(t, uint64(defaultPlan.ID), *config.QuotaPlanID)

		// Verify it was saved to DB
		var savedConfig pluginModels.UserQuotaConfig
		err = ctx.DB().Where("user_id = ?", userID).First(&savedConfig).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedConfig.UserID)
		assert.NotNil(t, savedConfig.QuotaPlanID)

		mockPlanManager.AssertExpectations(t)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetPolicyEnforcer_ValidPolicy_Success tests getting policy enforcer for valid policy
func TestConfigManager_GetPolicyEnforcer_ValidPolicy_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Create user quota config
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		err := ctx.DB().Create(userConfig).Error
		require.NoError(t, err)

		// Test
		enforcer, err := configManager.GetPolicyEnforcer(userID)
		require.NoError(t, err)
		assert.Equal(t, mockPolicyEnforcer, enforcer)

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetPolicyEnforcer_InvalidPolicy_Error tests getting policy enforcer for invalid policy
func TestConfigManager_GetPolicyEnforcer_InvalidPolicy_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Insert user quota config with invalid policy directly using raw SQL to bypass validation
		result := ctx.DB().Exec(`INSERT INTO user_quota_configs 
			(user_id, enforcement_policy, created_at, updated_at) 
			VALUES (?, ?, ?, ?)`,
			userID, "INVALID_POLICY", time.Now().UTC(), time.Now().UTC())
		require.NoError(t, result.Error)

		// Test
		enforcer, err := configManager.GetPolicyEnforcer(userID)
		assert.Error(t, err)
		assert.Nil(t, enforcer)
		assert.Contains(t, err.Error(), "no policy enforcer found for policy")

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserAllowanceGrants_Success tests getting user allowance grants
func TestConfigManager_GetUserAllowanceGrants_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)
		now := time.Now().UTC()
		futureDate := now.Add(30 * 24 * time.Hour).UTC()
		pastDate := now.Add(-24 * time.Hour).UTC()
		
		// Ensure we're working with UTC times for consistency
		futureDate = futureDate.UTC()
		pastDate = pastDate.UTC()

		// Create active grants
		activeGrant1 := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeStorage,
			Source:         pluginModels.GrantSourceSubscription,
			Bytes:          1000000,
			BytesUsed:      100000,
			BytesRemaining: 900000,
			ExpiryDate:     &futureDate,
			IsActive:       true,
		}

		activeGrant2 := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeUpload,
			Source:         pluginModels.GrantSourceBonus,
			Bytes:          500000,
			BytesUsed:      0,
			BytesRemaining: 500000,
			ExpiryDate:     nil, // No expiry
			IsActive:       true,
		}

		// Create inactive grant
		inactiveGrant := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeDownload,
			Source:         pluginModels.GrantSourcePromo,
			Bytes:          200000,
			BytesUsed:      50000,
			BytesRemaining: 150000,
			ExpiryDate:     &futureDate,
			IsActive:       false,
		}

		// Create expired grant using raw SQL to bypass validation
		expiredGrant := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeStorage,
			Source:         pluginModels.GrantSourcePAYGAddon,
			Bytes:          300000,
			BytesUsed:      0,
			BytesRemaining: 300000,
			IsActive:       true,
		}

		err := ctx.DB().Create(activeGrant1).Error
		require.NoError(t, err)

		err = ctx.DB().Create(activeGrant2).Error
		require.NoError(t, err)

		err = ctx.DB().Create(inactiveGrant).Error
		require.NoError(t, err)

		// Insert expired grant directly using raw SQL to bypass validation
		result := ctx.DB().Exec(`INSERT INTO allowance_grants 
			(user_id, type, source, bytes, bytes_used, bytes_remaining, expiry_date, is_active, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			userID, string(pluginModels.GrantTypeStorage), string(pluginModels.GrantSourcePAYGAddon), 
			300000, 0, 300000, pastDate, true, time.Now().UTC(), time.Now().UTC())
		require.NoError(t, result.Error)
		
		// Retrieve the inserted grant to get its ID for later verification
		var savedExpiredGrant pluginModels.AllowanceGrant
		err = ctx.DB().Where("user_id = ? AND type = ? AND source = ?", userID, pluginModels.GrantTypeStorage, pluginModels.GrantSourcePAYGAddon).First(&savedExpiredGrant).Error
		require.NoError(t, err)
		expiredGrant.ID = savedExpiredGrant.ID

		// Test
		grants, err := configManager.GetUserAllowanceGrants(userID)
		require.NoError(t, err)
		assert.Len(t, grants, 2)

		// Should only return active, non-expired grants
		grantIDs := make(map[uint]bool)
		for _, grant := range grants {
			grantIDs[grant.ID] = true
		}

		assert.True(t, grantIDs[activeGrant1.ID])
		assert.True(t, grantIDs[activeGrant2.ID])
		assert.False(t, grantIDs[inactiveGrant.ID])
		assert.False(t, grantIDs[expiredGrant.ID])

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserAllowanceGrantsByType_Success tests getting user allowance grants by type
func TestConfigManager_GetUserAllowanceGrantsByType_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)
		now := time.Now().UTC()
		futureDate := now.Add(30 * 24 * time.Hour).UTC()

		// Create storage grants
		storageGrant1 := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeStorage,
			Source:         pluginModels.GrantSourceSubscription,
			Bytes:          1000000,
			BytesUsed:      100000,
			BytesRemaining: 900000,
			ExpiryDate:     &futureDate,
			IsActive:       true,
		}

		storageGrant2 := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeStorage,
			Source:         pluginModels.GrantSourceBonus,
			Bytes:          500000,
			BytesUsed:      0,
			BytesRemaining: 500000,
			ExpiryDate:     nil,
			IsActive:       true,
		}

		// Create upload grant (different type)
		uploadGrant := &pluginModels.AllowanceGrant{
			UserID:         userID,
			Type:           pluginModels.GrantTypeUpload,
			Source:         pluginModels.GrantSourcePromo,
			Bytes:          200000,
			BytesUsed:      50000,
			BytesRemaining: 150000,
			ExpiryDate:     &futureDate,
			IsActive:       true,
		}

		err := ctx.DB().Create(storageGrant1).Error
		require.NoError(t, err)

		err = ctx.DB().Create(storageGrant2).Error
		require.NoError(t, err)

		err = ctx.DB().Create(uploadGrant).Error
		require.NoError(t, err)

		// Test
		grants, err := configManager.GetUserAllowanceGrantsByType(userID, pluginModels.GrantTypeStorage)
		require.NoError(t, err)
		assert.Len(t, grants, 2)

		// Should only return storage grants
		grantTypes := make(map[pluginModels.GrantType]bool)
		for _, grant := range grants {
			grantTypes[grant.Type] = true
		}

		assert.True(t, grantTypes[pluginModels.GrantTypeStorage])
		assert.False(t, grantTypes[pluginModels.GrantTypeUpload])

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserAllowanceGrants_InvalidUserID_Error tests error with invalid user ID
func TestConfigManager_GetUserAllowanceGrants_InvalidUserID_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Test with invalid user ID
		grants, err := configManager.GetUserAllowanceGrants(0)
		assert.Error(t, err)
		assert.Nil(t, grants)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)

	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserAllowanceGrantsByType_InvalidUserID_Error tests error with invalid user ID for type-specific grants
func TestConfigManager_GetUserAllowanceGrantsByType_InvalidUserID_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Setup mocks using factories
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Test with invalid user ID
		grants, err := configManager.GetUserAllowanceGrantsByType(0, pluginModels.GrantTypeStorage)
		assert.Error(t, err)
		assert.Nil(t, grants)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)

	}, pluginTesting.TestOptions())
}
