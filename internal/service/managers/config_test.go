package managers

import (
	"fmt"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
			StorageLimitConfig: &pluginCore.Limit{
				Bytes: uint64(1000000),
				Window: pluginCore.LimitWindow{
					Type: pluginCore.WindowTypeLifetime,
				},
				Priority: 0,
			},
			UploadLimitConfig: &pluginCore.Limit{
				Bytes: uint64(500000),
				Window: pluginCore.LimitWindow{
					Type: pluginCore.WindowTypeLifetime,
				},
				Priority: 0,
			},
			DownloadLimitConfig: &pluginCore.Limit{
				Bytes: uint64(500000),
				Window: pluginCore.LimitWindow{
					Type: pluginCore.WindowTypeLifetime,
				},
				Priority: 0,
			},
			HasStorageLimitConfig:   true,
			HasUploadLimitConfig:    true,
			HasDownloadLimitConfig:  true,
		}

		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, userConfig, pluginModels.EnforcementPolicyHardLimits).Return(expectedLimits, nil)

		// Setup mock expectation for GetDefaultQuotaPlan - return error since user config already exists
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		// Test
		limits, err := configManager.ResolveEffectiveLimits(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedLimits, limits)
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
			StorageLimitConfig: &pluginCore.Limit{
				Bytes: uint64(100000),
				Window: pluginCore.LimitWindow{
					Type: pluginCore.WindowTypeLifetime,
				},
				Priority: 0,
			},
			UploadLimitConfig: &pluginCore.Limit{
				Bytes: uint64(50000),
				Window: pluginCore.LimitWindow{
					Type: pluginCore.WindowTypeLifetime,
				},
				Priority: 0,
			},
			DownloadLimitConfig: &pluginCore.Limit{
				Bytes: uint64(50000),
				Window: pluginCore.LimitWindow{
					Type: pluginCore.WindowTypeLifetime,
				},
				Priority: 0,
			},
			HasStorageLimitConfig:  true,
			HasUploadLimitConfig:   true,
			HasDownloadLimitConfig: true,
		}

		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, userConfig, pluginModels.EnforcementPolicyHardLimits).Return(expectedLimits, nil)

		// Setup mock expectation for GetDefaultQuotaPlan - return error since user config already exists
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		// Test
		limits, err := configManager.ResolveEffectiveLimits(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, expectedLimits, limits)
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

		// Setup mock expectation for GetDefaultQuotaPlan - return error since user config already exists
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		// Test
		config, err := configManager.GetUserQuotaConfig(ctx, userID)
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
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Test - should create default config
		cfg, err := configManager.GetUserQuotaConfig(ctx, userID)
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
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(defaultPlan, nil)

		// Create ConfigManager
		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Setup test data
		userID := uint(1)

		// Test - should create default config with plan
		config, err := configManager.GetUserQuotaConfig(ctx, userID)
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

		// Setup mock expectation for GetDefaultQuotaPlan - return error since user config already exists
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		// Test
		enforcer, err := configManager.GetPolicyEnforcer(ctx, userID)
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

		// Setup mock expectation for GetDefaultQuotaPlan - return error since user config already exists
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		// Test
		enforcer, err := configManager.GetPolicyEnforcer(ctx, userID)
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

		// Create user quota config
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		err := ctx.DB().Create(userConfig).Error
		require.NoError(t, err)

		now := time.Now().UTC()
		futureDate := now.Add(30 * 24 * time.Hour)
		pastDate := now.Add(-24 * time.Hour)

		// Create active grants
		activeGrant1 := &pluginModels.AllowanceGrant{
			UserID:     userID,
			Type:       pluginModels.GrantTypeStorage,
			Source:     pluginModels.GrantSourceBonus,
			Bytes:      1000000,
			ExpiryDate: &futureDate,
			IsActive:   true,
		}
		activeGrant2 := &pluginModels.AllowanceGrant{
			UserID:     userID,
			Type:       pluginModels.GrantTypeUpload,
			Source:     pluginModels.GrantSourceBonus,
			Bytes:      500000,
			ExpiryDate: nil,
			IsActive:   true,
		}

		// Create inactive grant
		inactiveGrant := &pluginModels.AllowanceGrant{
			UserID:     userID,
			Type:       pluginModels.GrantTypeDownload,
			Source:     pluginModels.GrantSourceBonus,
			Bytes:      200000,
			ExpiryDate: &futureDate,
			IsActive:   false,
		}

		// Create expired grant with future date first (to pass validation), then update to past date
		expiredGrant := &pluginModels.AllowanceGrant{
			UserID:     userID,
			Type:       pluginModels.GrantTypeStorage,
			Source:     pluginModels.GrantSourceBonus,
			Bytes:      300000,
			ExpiryDate: &futureDate,
			IsActive:   true,
		}

		require.NoError(t, ctx.DB().Create(activeGrant1).Error)
		require.NoError(t, ctx.DB().Create(activeGrant2).Error)
		require.NoError(t, ctx.DB().Create(inactiveGrant).Error)
		// Create expired grant with future date first (to pass validation), then update to past date
		// Use raw SQL to bypass validation hooks
		require.NoError(t, ctx.DB().Create(expiredGrant).Error)
		require.NoError(t, ctx.DB().Exec("UPDATE allowance_grants SET expiry_date = ? WHERE id = ?", pastDate, expiredGrant.ID).Error)

		// Test
		grants, err := configManager.GetUserAllowanceGrants(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, grants, 2)

		// Should only return active, non-expired grants
		grantIDs := lo.SliceToMap(grants, func(g *pluginModels.AllowanceGrant) (uint, bool) {
			return g.ID, true
		})

		assert.Contains(t, grantIDs, activeGrant1.ID)
		assert.Contains(t, grantIDs, activeGrant2.ID)
		assert.NotContains(t, grantIDs, inactiveGrant.ID)
		assert.NotContains(t, grantIDs, expiredGrant.ID)
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

		// Create user quota config
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		err := ctx.DB().Create(userConfig).Error
		require.NoError(t, err)

		// Create test grants
		storageGrant := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeStorage,
			Source:   pluginModels.GrantSourceBonus,
			Bytes:    1000000,
			IsActive: true,
		}

		uploadGrant := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceBonus,
			Bytes:    500000,
			IsActive: true,
		}

		err = ctx.DB().Create(storageGrant).Error
		require.NoError(t, err)

		err = ctx.DB().Create(uploadGrant).Error
		require.NoError(t, err)

		// Test - get storage grants only
		storageGrants, err := configManager.GetUserAllowanceGrantsByType(ctx, userID, pluginModels.GrantTypeStorage)
		require.NoError(t, err)
		assert.Len(t, storageGrants, 1)
		assert.Equal(t, pluginModels.GrantTypeStorage, storageGrants[0].Type)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserAllowanceGrants_NoGrants_Success tests getting user allowance grants when none exist
func TestConfigManager_GetUserAllowanceGrants_NoGrants_Success(t *testing.T) {
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

		// Test - no grants exist
		grants, err := configManager.GetUserAllowanceGrants(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, grants, 0)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_GetUserQuotaConfig_ExistingConfigDifferentPolicy_FindsExisting tests
// that GetUserQuotaConfig finds an existing config even when its enforcement_policy
// differs from the default. This is a regression test for a bug where FirstOrCreate
// included enforcement_policy in the WHERE clause, causing it to miss existing rows
// and attempt a duplicate INSERT.
func TestConfigManager_GetUserQuotaConfig_ExistingConfigDifferentPolicy_FindsExisting(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyUnlimited: mockPolicyEnforcer,
		}

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		userID := uint(2)

		// Create config with UNLIMITED policy (different from default HARD_LIMITS)
		userConfig := &pluginModels.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: pluginModels.EnforcementPolicyUnlimited,
		}
		err := ctx.DB().Create(userConfig).Error
		require.NoError(t, err)

		// GetUserQuotaConfig should find the existing row, not try to create a new one
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		config, err := configManager.GetUserQuotaConfig(ctx, userID)
		require.NoError(t, err)
		assert.Equal(t, userID, config.UserID)
		assert.Equal(t, pluginModels.EnforcementPolicyUnlimited, config.EnforcementPolicy)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimitsBatch_MultipleUsers tests batch resolution
// of effective limits for multiple users with existing configs.
func TestConfigManager_ResolveEffectiveLimitsBatch_MultipleUsers(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		userID1 := uint(1)
		userID2 := uint(2)
		userID3 := uint(3)

		config1 := &pluginModels.UserQuotaConfig{
			UserID:            userID1,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		config2 := &pluginModels.UserQuotaConfig{
			UserID:            userID2,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		config3 := &pluginModels.UserQuotaConfig{
			UserID:            userID3,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}

		for _, cfg := range []*pluginModels.UserQuotaConfig{config1, config2, config3} {
			err := ctx.DB().Create(cfg).Error
			require.NoError(t, err)
		}

		limits1 := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}
		limits2 := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}
		limits3 := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}

		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, config1, pluginModels.EnforcementPolicyHardLimits).Return(limits1, nil)
		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, config2, pluginModels.EnforcementPolicyHardLimits).Return(limits2, nil)
		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, config3, pluginModels.EnforcementPolicyHardLimits).Return(limits3, nil)

		result, err := configManager.ResolveEffectiveLimitsBatch(ctx, []uint{userID1, userID2, userID3})
		require.NoError(t, err)
		assert.Len(t, result, 3)
		assert.Equal(t, limits1, result[userID1])
		assert.Equal(t, limits2, result[userID2])
		assert.Equal(t, limits3, result[userID3])
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimitsBatch_EmptyInput tests that an empty
// user ID slice returns an empty map without error or DB queries.
func TestConfigManager_ResolveEffectiveLimitsBatch_EmptyInput(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, nil)

		result, err := configManager.ResolveEffectiveLimitsBatch(ctx, []uint{})
		require.NoError(t, err)
		assert.Empty(t, result)
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimitsBatch_UserWithoutConfig tests that
// users without an existing config fall back to single-user resolution which
// handles default config creation.
func TestConfigManager_ResolveEffectiveLimitsBatch_UserWithoutConfig(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		userID := uint(42)

		// User has no config in DB — batch should fall back to single-user path
		// which calls GetUserQuotaConfig (creates default), then ResolveEffectiveLimits
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))

		expectedLimits := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}
		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), pluginModels.EnforcementPolicyHardLimits).Return(expectedLimits, nil)

		result, err := configManager.ResolveEffectiveLimitsBatch(ctx, []uint{userID})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, expectedLimits, result[userID])
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimitsBatch_MixedExistingAndMissing tests
// batch resolution where some users have configs and some don't.
func TestConfigManager_ResolveEffectiveLimitsBatch_MixedExistingAndMissing(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// User 1: has config in DB
		userID1 := uint(1)
		config1 := &pluginModels.UserQuotaConfig{
			UserID:            userID1,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		err := ctx.DB().Create(config1).Error
		require.NoError(t, err)

		// User 2: no config in DB (will fall back to single-user path)
		userID2 := uint(2)

		limits1 := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}
		limits2 := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}

		// User 1: resolved directly from existing config
		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, config1, pluginModels.EnforcementPolicyHardLimits).Return(limits1, nil)
		// User 2: falls back to single-user path — needs plan lookup + limit resolution
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return((*pluginModels.QuotaPlan)(nil), fmt.Errorf("no default plan"))
		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, mock.AnythingOfType("*models.UserQuotaConfig"), pluginModels.EnforcementPolicyHardLimits).Return(limits2, nil)

		result, err := configManager.ResolveEffectiveLimitsBatch(ctx, []uint{userID1, userID2})
		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, limits1, result[userID1])
		assert.Equal(t, limits2, result[userID2])
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimitsBatch_ResolutionError tests that
// when the limit resolver returns an error for any user, the batch omits that
// user from the result map (log+continue) rather than aborting the entire batch.
func TestConfigManager_ResolveEffectiveLimitsBatch_ResolutionError(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// User 1: resolves successfully
		userID1 := uint(1)
		config1 := &pluginModels.UserQuotaConfig{
			UserID:            userID1,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		err := ctx.DB().Create(config1).Error
		require.NoError(t, err)

		// User 2: will fail resolution
		userID2 := uint(2)
		config2 := &pluginModels.UserQuotaConfig{
			UserID:            userID2,
			EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
		}
		err = ctx.DB().Create(config2).Error
		require.NoError(t, err)

		limits1 := &pluginCore.EffectiveLimits{HasStorageLimitConfig: false}

		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, config1, pluginModels.EnforcementPolicyHardLimits).Return(limits1, nil)
		mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, config2, pluginModels.EnforcementPolicyHardLimits).Return((*pluginCore.EffectiveLimits)(nil), fmt.Errorf("resolver failure"))

		result, err := configManager.ResolveEffectiveLimitsBatch(ctx, []uint{userID1, userID2})
		require.NoError(t, err, "Batch should not return error for individual resolution failure")
		assert.Len(t, result, 1, "Only successfully resolved users should be in the map")
		assert.Equal(t, limits1, result[userID1])
		_, exists := result[userID2]
		assert.False(t, exists, "Failed user should be omitted from result")
	}, pluginTesting.TestOptions())
}

// TestConfigManager_ResolveEffectiveLimitsBatch_BatchCreatesMissingConfigs is a regression test
// verifying that missing user configs are batch-created in a single query
// rather than falling back to per-user ResolveEffectiveLimits (N+1).
func TestConfigManager_ResolveEffectiveLimitsBatch_BatchCreatesMissingConfigs(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		mockLimitResolver := pluginCore.NewMockLimitResolver(tb)
		mockPlanManager := pluginCore.NewMockQuotaPlanManager(tb)
		mockPolicyEnforcer := pluginCore.NewMockPolicyEnforcer(tb)

		policyEnforcers := map[pluginModels.EnforcementPolicy]pluginCore.PolicyEnforcer{
			pluginModels.EnforcementPolicyHardLimits: mockPolicyEnforcer,
		}

		configManager := NewConfigManager(ctx, mockLimitResolver, mockPlanManager, policyEnforcers)

		// Three users with NO pre-existing configs
		userIDs := []uint{101, 102, 103}

		// Mock plan manager: no default plan → system defaults used
		mockPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, fmt.Errorf("no default plan"))

		// Mock resolver: called once per user with the batch-created default config
		for range userIDs {
			mockLimitResolver.EXPECT().ResolveEffectiveLimits(mock.Anything, mock.Anything, pluginModels.EnforcementPolicyHardLimits).Return(&pluginCore.EffectiveLimits{HasStorageLimitConfig: false}, nil)
		}

		result, err := configManager.ResolveEffectiveLimitsBatch(ctx, userIDs)

		require.NoError(t, err)
		assert.Len(t, result, len(userIDs), "All users should be resolved after batch default config creation")

		// Verify configs were actually created in the DB
		for _, uid := range userIDs {
			var cfg pluginModels.UserQuotaConfig
			err := ctx.DB().Where("user_id = ?", uid).First(&cfg).Error
			require.NoError(t, err, "Config should exist in DB for user %d", uid)
		}
	}, pluginTesting.TestOptions())
}
