package policies

import (
	"errors"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"gorm.io/gorm"
)

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_CustomLimitsOnly(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:             1,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		StorageLimitBytes:  1000,
		UploadLimitBytes:   500,
		DownloadLimitBytes: 2000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
	}

	// Mock default quota plan lookup to return not found
	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

	limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.Equal(t, uint(1), limits.UserID)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits), limits.EnforcementPolicy)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(1000), limits.StorageLimitConfig.Bytes)
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(500), limits.UploadLimitConfig.Bytes)
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(2000), limits.DownloadLimitConfig.Bytes)
	assert.True(t, limits.HasStorageLimitConfig)
	assert.True(t, limits.HasUploadLimitConfig)
	assert.True(t, limits.HasDownloadLimitConfig)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_PlanLimitsWithCustomOverrides(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	planID := uint64(42)
	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		QuotaPlanID:       &planID,
		StorageLimitBytes: uint64(3000), // Custom override
	}

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 42},
		StorageLimitBytes:  1000,
		UploadLimitBytes:   500,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 2000,
		IsActive:           lo.ToPtr(true),
	}

	mockQuotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)

	limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(3000), limits.StorageLimitConfig.Bytes) // Custom value
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(500), limits.UploadLimitConfig.Bytes) // Plan value
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(2000), limits.DownloadLimitConfig.Bytes) // Plan value
	assert.Equal(t, &planID, limits.QuotaPlanID)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_DefaultPlanWhenNoCustomPlan(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimitBytes:  5000,
		UploadLimitBytes:   1000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 3000,
		IsActive:           lo.ToPtr(true),
	}

	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(plan, nil)

	limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(5000), limits.StorageLimitConfig.Bytes)
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(1000), limits.UploadLimitConfig.Bytes)
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(3000), limits.DownloadLimitConfig.Bytes)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_ErrorWhenNoLimitsConfigured(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            4,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, gorm.ErrRecordNotFound)

	limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyHardLimits)
	assert.Error(t, err)
	assert.Nil(t, limits)
	assert.Contains(t, err.Error(), "no limits configured")
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_DefaultPlanProvidesLimitsWhenNoCustomPlan(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            5,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	windowDuration := int64(86400)
	windowStartHour := 0
	timezone := "UTC"

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimitBytes:  5000,
		UploadLimitBytes:   1000,
		WindowType:         models.WindowTypeCalendarDay,
		WindowDuration:     &windowDuration,
		WindowStartHour:    &windowStartHour,
		WindowTimezone:     &timezone,
		DownloadLimitBytes: 3000,
		IsActive:           lo.ToPtr(true),
	}

	mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(plan, nil)

	limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.NotNil(t, limits.StorageLimitConfig)
	assert.Equal(t, uint64(5000), limits.StorageLimitConfig.Bytes)
	assert.NotNil(t, limits.UploadLimitConfig)
	assert.Equal(t, uint64(1000), limits.UploadLimitConfig.Bytes)
	assert.NotNil(t, limits.DownloadLimitConfig)
	assert.Equal(t, uint64(3000), limits.DownloadLimitConfig.Bytes)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_Threshold(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)

	resolver := NewLimitResolver(ctx, mockQuotaService)

	t.Run("With thresholds", func(t *testing.T) {
		planID := uint64(42)
		config := &models.UserQuotaConfig{
			UserID:            1,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			QuotaPlanID:       &planID,
		}

		storageThreshold := int64(800)
		uploadThreshold := int64(400)
		windowDuration := int64(86400)
		windowStartHour := 0
		timezone := "UTC"

		plan := &models.QuotaPlan{
			Model:             gorm.Model{ID: 42},
			StorageLimitBytes: 1000,
			UploadLimitBytes:  500,
			WindowType:        models.WindowTypeCalendarDay,
			WindowDuration:    &windowDuration,
			WindowStartHour:   &windowStartHour,
			WindowTimezone:    &timezone,
			StorageThreshold:  &storageThreshold,
			UploadThreshold:   &uploadThreshold,
			IsActive:          lo.ToPtr(true),
		}

		mockQuotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)

		limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyThreshold)
		require.NoError(t, err)
		assert.NotNil(t, limits.StorageLimitConfig)
		assert.Equal(t, uint64(1000), limits.StorageLimitConfig.Bytes)
		assert.NotNil(t, limits.UploadLimitConfig)
		assert.Equal(t, uint64(500), limits.UploadLimitConfig.Bytes)
		assert.Equal(t, uint64(800), *limits.StorageThreshold)
		assert.Equal(t, uint64(400), *limits.UploadThreshold)
		assert.True(t, limits.HasStorageThresholdConfig)
		assert.True(t, limits.HasUploadThresholdConfig)
	})

	t.Run("Custom threshold overrides", func(t *testing.T) {
		planID := uint64(42)
		customThreshold := int64(900)
		config := &models.UserQuotaConfig{
			UserID:            2,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
			QuotaPlanID:       &planID,
			StorageThreshold:  &customThreshold, // Custom override
		}

		planThreshold := int64(800)
		plan := &models.QuotaPlan{
			Model:             gorm.Model{ID: 42},
			StorageLimitBytes: 1000,
			StorageThreshold:  &planThreshold,
			IsActive:          lo.ToPtr(true),
		}

		mockQuotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(plan, nil)

		limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyThreshold)
		require.NoError(t, err)
		assert.Equal(t, uint64(900), *limits.StorageThreshold) // Custom value
	})
}

// ValidateThresholdVsLimit and ApplyLimit methods removed from LimitResolver interface
// as part of window-based limits simplification. These tests have been removed.
// Threshold validation is now handled by EvaluateThreshold in core/types.go

func TestEvaluateThreshold(t *testing.T) {
	t.Run("Always warn threshold", func(t *testing.T) {
		result := pluginCore.EvaluateThreshold(100, 50, 0, 1000)
		assert.True(t, result.ShouldWarn)
		assert.True(t, result.WithinLimit)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.DecisionReason)
	})

	t.Run("Would exceed threshold within limit", func(t *testing.T) {
		result := pluginCore.EvaluateThreshold(800, 300, 1000, 2000)
		assert.True(t, result.ShouldWarn)
		assert.True(t, result.WithinLimit)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.DecisionReason)
	})

	t.Run("Would cross threshold within limit", func(t *testing.T) {
		result := pluginCore.EvaluateThreshold(900, 200, 1000, 2000)
		assert.True(t, result.ShouldWarn)
		assert.True(t, result.WithinLimit)
		assert.Equal(t, models.QuotaCheckReasonWarningThreshold, result.DecisionReason)
	})

	t.Run("No warning needed", func(t *testing.T) {
		result := pluginCore.EvaluateThreshold(500, 100, 1000, 2000)
		assert.False(t, result.ShouldWarn)
		assert.True(t, result.WithinLimit)
		assert.Equal(t, models.QuotaCheckReasonOK, result.DecisionReason)
	})

	t.Run("Exceeds limit", func(t *testing.T) {
		result := pluginCore.EvaluateThreshold(800, 300, 1000, 1000)
		assert.False(t, result.ShouldWarn) // No warning if exceeding limit
		assert.False(t, result.WithinLimit)
		assert.Equal(t, models.QuotaCheckReasonOK, result.DecisionReason)
	})
}

func TestDefaultLimitResolver_ErrorCases(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.EXPECT().GetQuotaPlanManager().Return(mockQuotaPlanManager)

	resolver := NewLimitResolver(ctx, mockQuotaService)

	t.Run("Nil config", func(t *testing.T) {
		limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), nil, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "quota config is nil")
	})

	t.Run("Plan lookup error", func(t *testing.T) {
		planID := uint64(42)
		config := &models.UserQuotaConfig{
			UserID:            1,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
			QuotaPlanID:       &planID,
		}

		mockQuotaPlanManager.EXPECT().GetQuotaPlanByID(mock.Anything, planID).Return(nil, errors.New("plan not found"))

		limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "failed to retrieve quota plan")
	})

	t.Run("Default plan lookup error", func(t *testing.T) {
		config := &models.UserQuotaConfig{
			UserID:            1,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		mockQuotaPlanManager.EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(nil, errors.New("default plan error"))

		limits, err := resolver.ResolveEffectiveLimits(ctx.GetContext(), config, models.EnforcementPolicyThreshold)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "failed to retrieve default quota plan")
	})
}

// Helper function
func intPtr(i int64) *int64 {
	return &i
}

func TestDefaultLimitResolver_applyPlanLimits_HasConfigFlags(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	resolver := NewLimitResolver(ctx, mockQuotaService)

	t.Run("AllPlanLimitsExplicitlySet", func(t *testing.T) {
		// Test with all plan limit values explicitly set using window-based schema
		windowType := models.WindowTypeRolling
		windowDuration := int64(86400) // 1 day
		plan := &models.QuotaPlan{
			StorageLimitBytes:  100, // normal value
			UploadLimitBytes:   0,   // disabled
			DownloadLimitBytes: 100, // normal value
			WindowType:         windowType,
			WindowDuration:     &windowDuration,
		}

		limits := &pluginCore.EffectiveLimits{}
		resolver.applyPlanLimits(limits, plan, models.EnforcementPolicyHardLimits)

		// Flags reflect what's actually configured (> 0 values)
		assert.True(t, limits.HasStorageLimitConfig)    // 100 > 0, so it's configured
		assert.False(t, limits.HasUploadLimitConfig)    // 0 means disabled, not configured
		assert.True(t, limits.HasDownloadLimitConfig)   // 100 > 0, so it's configured

		// UploadLimitConfig should be nil (disabled/zero)
		assert.Nil(t, limits.UploadLimitConfig)

		// StorageLimitConfig and DownloadLimitConfig should be set to their values
		assert.NotNil(t, limits.StorageLimitConfig)
		assert.NotNil(t, limits.DownloadLimitConfig)
		assert.Equal(t, uint64(100), limits.StorageLimitConfig.Bytes)
		assert.Equal(t, uint64(100), limits.DownloadLimitConfig.Bytes)
	})

	t.Run("PlanWithMixedValues", func(t *testing.T) {
		// Test with mixed plan limit values using window-based schema
		windowDuration := int64(86400)
		plan := &models.QuotaPlan{
			StorageLimitBytes: 1000,
			UploadLimitBytes:  0, // disabled
			WindowDuration:    &windowDuration,
		}

		limits := &pluginCore.EffectiveLimits{}
		resolver.applyPlanLimits(limits, plan, models.EnforcementPolicyHardLimits)

		// Limit config flags should reflect what's configured
		assert.True(t, limits.HasStorageLimitConfig)
		assert.False(t, limits.HasUploadLimitConfig)
		assert.False(t, limits.HasDownloadLimitConfig)

		// Check the actual values
		assert.NotNil(t, limits.StorageLimitConfig)
		assert.Equal(t, uint64(1000), limits.StorageLimitConfig.Bytes)

		assert.Nil(t, limits.UploadLimitConfig) // 0 becomes nil (disabled)

		// DownloadLimitConfig wasn't explicitly set in plan
		assert.Nil(t, limits.DownloadLimitConfig)

		// Thresholds are only set for Threshold policy, not HardLimits
		assert.Nil(t, limits.StorageThreshold)
	})

}
