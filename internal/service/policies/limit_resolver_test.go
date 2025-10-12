package policies

import (
	"errors"
	"testing"

	"github.com/docker/go-units"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
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
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:             1,
		EnforcementPolicy:  models.EnforcementPolicyHardLimits,
		StorageLimit:       intPtr(1000),
		UploadDailyLimit:   intPtr(500),
		DownloadDailyLimit: intPtr(2000),
	}

	// Mock default quota plan lookup to return not found
	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound)

	limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.Equal(t, uint(1), limits.UserID)
	assert.Equal(t, pluginCore.EnforcementPolicy(models.EnforcementPolicyHardLimits), limits.EnforcementPolicy)
	assert.Equal(t, uint64(1000), *limits.StorageLimit)
	assert.Equal(t, uint64(500), *limits.UploadDailyLimit)
	assert.Equal(t, uint64(2000), *limits.DownloadDailyLimit)
	assert.True(t, limits.HasStorageLimitConfig)
	assert.True(t, limits.HasUploadDailyLimitConfig)
	assert.True(t, limits.HasDownloadDailyLimitConfig)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_PlanLimitsWithCustomOverrides(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	planID := uint64(42)
	config := &models.UserQuotaConfig{
		UserID:            2,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
		QuotaPlanID:       &planID,
		StorageLimit:      intPtr(3000), // Custom override
	}

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 42},
		StorageLimit:       1000,
		UploadDailyLimit:   500,
		DownloadDailyLimit: 2000,
		IsActive:           lo.ToPtr(true),
	}

	mockQuotaPlanManager.On("GetQuotaPlanByID", planID).Return(plan, nil)

	limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.Equal(t, uint64(3000), *limits.StorageLimit)       // Custom value
	assert.Equal(t, uint64(500), *limits.UploadDailyLimit)    // Plan value
	assert.Equal(t, uint64(2000), *limits.DownloadDailyLimit) // Plan value
	assert.Equal(t, &planID, limits.QuotaPlanID)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_DefaultPlanWhenNoCustomPlan(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            3,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimit:       5000,
		UploadDailyLimit:   1000,
		DownloadDailyLimit: 3000,
		IsActive:           lo.ToPtr(true),
	}

	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(plan, nil)

	limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.Equal(t, uint64(5000), *limits.StorageLimit)
	assert.Equal(t, uint64(1000), *limits.UploadDailyLimit)
	assert.Equal(t, uint64(3000), *limits.DownloadDailyLimit)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_ErrorWhenNoLimitsConfigured(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            4,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, gorm.ErrRecordNotFound)

	limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	assert.Error(t, err)
	assert.Nil(t, limits)
	assert.Contains(t, err.Error(), "no limits configured")
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_HardLimits_DefaultPlanProvidesLimitsWhenNoCustomPlan(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)
	resolver := NewLimitResolver(ctx, mockQuotaService)
	config := &models.UserQuotaConfig{
		UserID:            5,
		EnforcementPolicy: models.EnforcementPolicyHardLimits,
	}

	plan := &models.QuotaPlan{
		Model:              gorm.Model{ID: 1},
		StorageLimit:       5000,
		UploadDailyLimit:   1000,
		DownloadDailyLimit: 3000,
		IsActive:           lo.ToPtr(true),
	}

	mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(plan, nil)

	limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
	require.NoError(t, err)
	assert.Equal(t, uint64(5000), *limits.StorageLimit)
	assert.Equal(t, uint64(1000), *limits.UploadDailyLimit)
	assert.Equal(t, uint64(3000), *limits.DownloadDailyLimit)
}

func TestDefaultLimitResolver_ResolveEffectiveLimits_Threshold(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	mockQuotaPlanManager := pluginCore.NewMockQuotaPlanManager(t)
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)

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

		plan := &models.QuotaPlan{
			Model:            gorm.Model{ID: 42},
			StorageLimit:     1000,
			UploadDailyLimit: 500,
			StorageThreshold: &storageThreshold,
			UploadThreshold:  &uploadThreshold,
			IsActive:         lo.ToPtr(true),
		}

		mockQuotaPlanManager.On("GetQuotaPlanByID", planID).Return(plan, nil)

		limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
		require.NoError(t, err)
		assert.Equal(t, uint64(1000), *limits.StorageLimit)
		assert.Equal(t, uint64(500), *limits.UploadDailyLimit)
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
			Model:            gorm.Model{ID: 42},
			StorageLimit:     1000,
			StorageThreshold: &planThreshold,
			IsActive:         lo.ToPtr(true),
		}

		mockQuotaPlanManager.On("GetQuotaPlanByID", planID).Return(plan, nil)

		limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
		require.NoError(t, err)
		assert.Equal(t, uint64(900), *limits.StorageThreshold) // Custom value
	})
}

func TestDefaultLimitResolver_ValidateThresholdVsLimit(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	resolver := NewLimitResolver(ctx, mockQuotaService)

	t.Run("Valid threshold", func(t *testing.T) {
		err := resolver.ValidateThresholdVsLimit(800, 1000, "storage threshold")
		assert.NoError(t, err)
	})

	t.Run("Threshold exceeds limit", func(t *testing.T) {
		err := resolver.ValidateThresholdVsLimit(1200, 1000, "storage threshold")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "threshold cannot exceed limit")
	})

	t.Run("Unlimited limit", func(t *testing.T) {
		err := resolver.ValidateThresholdVsLimit(2000, -1, "storage threshold")
		assert.NoError(t, err)
	})

	t.Run("Disabled limit", func(t *testing.T) {
		err := resolver.ValidateThresholdVsLimit(1000, 0, "storage threshold")
		assert.NoError(t, err) // Should skip validation when limit is disabled
	})
}

func TestDefaultLimitResolver_ApplyLimit(t *testing.T) {
	ctx, _ := coreTesting.NewTestContext(t)
	mockQuotaService := pluginCore.NewMockQuotaService(t)
	resolver := NewLimitResolver(ctx, mockQuotaService)

	t.Run("Positive value", func(t *testing.T) {
		var limit *uint64
		err := resolver.ApplyLimit(&limit, 1000, "test limit")
		assert.NoError(t, err)
		assert.Equal(t, uint64(1000), *limit)
	})

	t.Run("Unlimited value", func(t *testing.T) {
		var limit *uint64
		err := resolver.ApplyLimit(&limit, -1, "test limit")
		assert.NoError(t, err)
		assert.Nil(t, limit)
	})

	t.Run("Zero value", func(t *testing.T) {
		var limit *uint64
		err := resolver.ApplyLimit(&limit, 0, "test limit")
		assert.NoError(t, err)
		assert.Equal(t, uint64(0), *limit)
	})

	t.Run("Zero value with treatZeroAsNil", func(t *testing.T) {
		var limit *uint64
		err := resolver.ApplyLimit(&limit, 0, "test limit", pluginCore.WithTreatZeroAsNil())
		assert.NoError(t, err)
		assert.Nil(t, limit)
	})

	t.Run("Invalid negative value", func(t *testing.T) {
		var limit *uint64
		err := resolver.ApplyLimit(&limit, -2, "test limit")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "must be -1, 0, or positive")
	})

	t.Run("Unreasonably large value", func(t *testing.T) {
		var limit *uint64
		err := resolver.ApplyLimit(&limit, int64(2*units.PiB), "test limit")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unreasonably large")
	})
}

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
	mockQuotaService.On("GetQuotaPlanManager").Return(mockQuotaPlanManager)

	resolver := NewLimitResolver(ctx, mockQuotaService)

	t.Run("Nil config", func(t *testing.T) {
		limits, err := resolver.ResolveEffectiveLimits(nil, models.EnforcementPolicyHardLimits)
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

		mockQuotaPlanManager.On("GetQuotaPlanByID", planID).Return(nil, errors.New("plan not found"))

		limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyHardLimits)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "failed to retrieve quota plan")
	})

	t.Run("Default plan lookup error", func(t *testing.T) {
		config := &models.UserQuotaConfig{
			UserID:            1,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		mockQuotaPlanManager.On("GetDefaultQuotaPlan").Return(nil, errors.New("default plan error"))

		limits, err := resolver.ResolveEffectiveLimits(config, models.EnforcementPolicyThreshold)
		assert.Error(t, err)
		assert.Nil(t, limits)
		assert.Contains(t, err.Error(), "failed to retrieve default quota plan")
	})
}

// Helper function
func intPtr(i int64) *int64 {
	return &i
}
