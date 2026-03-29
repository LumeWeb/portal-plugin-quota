// Package testdata provides a centralized test data management system
// for creating and tracking test entities with automatic cleanup capabilities.
package testdata

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestDataManager provides centralized test data management with atomic ID generation,
// resource tracking, and automatic cleanup capabilities.
type TestDataManager struct {
	// Atomic counters for unique ID generation
	userIDCounter   atomic.Uint64
	uploadIDCounter atomic.Uint64
	planIDCounter   atomic.Uint64
	grantIDCounter  atomic.Uint64

	// Resource tracking for cleanup
	createdUsers        sync.Map
	createdPlans        sync.Map
	createdGrants       sync.Map
	createdUsageDetails sync.Map

	// Context reference
	ctx coreTesting.TestContext
}

// NewTestDataManager creates a new TestDataManager instance
func NewTestDataManager(ctx coreTesting.TestContext) *TestDataManager {
	manager := &TestDataManager{
		ctx: ctx,
	}

	// Initialize counters with non-zero values to avoid conflicts with real data
	manager.userIDCounter.Store(10000)
	manager.uploadIDCounter.Store(10000)
	manager.planIDCounter.Store(10000)
	manager.grantIDCounter.Store(10000)

	return manager
}

// GenerateUserID generates a unique user ID for testing
func (tdm *TestDataManager) GenerateUserID() uint {
	return uint(tdm.userIDCounter.Add(1))
}

// GenerateUploadID generates a unique upload ID for testing
func (tdm *TestDataManager) GenerateUploadID() uint {
	return uint(tdm.uploadIDCounter.Add(1))
}

// GeneratePlanID generates a unique plan ID for testing
func (tdm *TestDataManager) GeneratePlanID() uint {
	return uint(tdm.planIDCounter.Add(1))
}

// GenerateGrantID generates a unique grant ID for testing
func (tdm *TestDataManager) GenerateGrantID() uint {
	return uint(tdm.grantIDCounter.Add(1))
}

// CreateUser creates a test user with quota configuration and tracks it for cleanup
func (tdm *TestDataManager) CreateUser(userID uint, policy pluginModels.EnforcementPolicy, limits *TestUserLimits) *pluginModels.UserQuotaConfig {
	cfg := &pluginModels.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: policy,
	}

	if limits != nil {
		if limits.StorageLimitBytes != nil {
			cfg.StorageLimitBytes = uint64(*limits.StorageLimitBytes)
		}
		if limits.UploadLimitBytes != nil {
			cfg.UploadLimitBytes = uint64(*limits.UploadLimitBytes)
		}
		if limits.DownloadLimitBytes != nil {
			cfg.DownloadLimitBytes = uint64(*limits.DownloadLimitBytes)
		}
		if limits.StorageThreshold != nil {
			cfg.StorageThreshold = limits.StorageThreshold
		}
		if limits.UploadThreshold != nil {
			cfg.UploadThreshold = limits.UploadThreshold
		}
		if limits.DownloadThreshold != nil {
			cfg.DownloadThreshold = limits.DownloadThreshold
		}
		if limits.QuotaPlanID != nil {
			cfg.QuotaPlanID = limits.QuotaPlanID
		}
		if limits.WindowType != nil {
			cfg.WindowType = pluginModels.WindowType(*limits.WindowType)
		} else {
			// Set default window type to prevent validation errors during window-based limits
			cfg.WindowType = pluginModels.WindowTypeLifetime
		}
		if limits.WindowDuration != nil {
			cfg.WindowDuration = limits.WindowDuration
		} else {
			// Set default window duration (24 hours for DAY type)
			duration := int64(86400)
			cfg.WindowDuration = &duration
		}
		if limits.WindowStartHour != nil {
			cfg.WindowStartHour = limits.WindowStartHour
		} else {
			// Set default start hour
			startHour := 0
			cfg.WindowStartHour = &startHour
		}
		if limits.WindowTimezone != nil {
			cfg.WindowTimezone = limits.WindowTimezone
		} else {
			// Set default timezone
			timezone := "UTC"
			cfg.WindowTimezone = &timezone
		}
	}

	err := tdm.ctx.DB().Create(cfg).Error
	require.NoError(tdm.ctx.T(), err, "Failed to create user quota config")

	// Track for cleanup
	tdm.createdUsers.Store(userID, cfg)

	return cfg
}

// CreateDefaultUser creates a test user with default hard limits policy
func (tdm *TestDataManager) CreateDefaultUser() *pluginModels.UserQuotaConfig {
	userID := tdm.GenerateUserID()
	return tdm.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, nil)
}

// CreateQuotaPlan creates a test quota plan and tracks it for cleanup
func (tdm *TestDataManager) CreateQuotaPlan(name string, storageLimit, uploadLimit, downloadLimit int64, isDefault bool) *pluginModels.QuotaPlan {
	duration := int64(24)
	startHour := 0
	timezone := "UTC"
	plan := &pluginModels.QuotaPlan{
		Name:               name,
		Description:        "Test plan",
		StorageLimitBytes:  uint64(storageLimit),
		UploadLimitBytes:   uint64(uploadLimit),
		DownloadLimitBytes: uint64(downloadLimit),
		WindowType:         pluginModels.WindowTypeRolling,
		WindowDuration:     &duration,
		WindowStartHour:    &startHour,
		WindowTimezone:     &timezone,
		IsDefault:          isDefault,
		IsActive:           lo.ToPtr(true),
	}

	err := tdm.ctx.DB().Create(plan).Error
	require.NoError(tdm.ctx.T(), err, "Failed to create quota plan")

	// Track for cleanup
	tdm.createdPlans.Store(plan.ID, plan)

	return plan
}

// CreateAllowanceGrant creates a test allowance grant and tracks it for cleanup
func (tdm *TestDataManager) CreateAllowanceGrant(userID uint, grantType pluginModels.GrantType, bytes uint64) *pluginModels.AllowanceGrant {
	grant := &pluginModels.AllowanceGrant{
		UserID:         userID,
		Type:           grantType,
		Source:         pluginModels.GrantSourcePAYGAddon,
		Bytes:          bytes,
		BytesUsed:      0,
		BytesRemaining: bytes,
		IsActive:       true,
	}

	err := tdm.ctx.DB().Create(grant).Error
	require.NoError(tdm.ctx.T(), err, "Failed to create allowance grant")

	// Track for cleanup
	tdm.createdGrants.Store(grant.ID, grant)

	return grant
}

// CreateUsageDetail creates a test usage detail record and tracks it for cleanup
func (tdm *TestDataManager) CreateUsageDetail(userID uint, uploadID uint, usageType pluginModels.UsageType, bytes uint64, ip string) *pluginModels.UserUsageDetail {
	detail := &pluginModels.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      usageType,
		Bytes:     bytes,
		IP:        pluginModels.IPAddr(ip),
		Timestamp: time.Now().UTC(),
	}

	err := tdm.ctx.DB().Create(detail).Error
	require.NoError(tdm.ctx.T(), err, "Failed to create usage detail")

	// Track for cleanup
	tdm.createdUsageDetails.Store(detail.ID, detail)

	return detail
}

// CreateUsageDetailWithTimestamp creates a test usage detail record with a specific timestamp
func (tdm *TestDataManager) CreateUsageDetailWithTimestamp(userID uint, uploadID uint, usageType pluginModels.UsageType, bytes uint64, ip string, timestamp time.Time) *pluginModels.UserUsageDetail {
	detail := &pluginModels.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      usageType,
		Bytes:     bytes,
		IP:        pluginModels.IPAddr(ip),
		Timestamp: timestamp,
	}

	err := tdm.ctx.DB().Create(detail).Error
	require.NoError(tdm.ctx.T(), err, "Failed to create usage detail")

	// Track for cleanup
	tdm.createdUsageDetails.Store(detail.ID, detail)

	return detail
}

// Cleanup removes all tracked test entities from the database
func (tdm *TestDataManager) Cleanup() {
	// Clean up usage details
	tdm.createdUsageDetails.Range(func(key, value interface{}) bool {
		detail := value.(*pluginModels.UserUsageDetail)
		tdm.ctx.DB().Delete(detail)
		return true
	})

	// Clean up grants
	tdm.createdGrants.Range(func(key, value interface{}) bool {
		grant := value.(*pluginModels.AllowanceGrant)
		tdm.ctx.DB().Delete(grant)
		return true
	})

	// Clean up plans
	tdm.createdPlans.Range(func(key, value interface{}) bool {
		plan := value.(*pluginModels.QuotaPlan)
		tdm.ctx.DB().Delete(plan)
		return true
	})

	// Clean up users
	tdm.createdUsers.Range(func(key, value interface{}) bool {
		user := value.(*pluginModels.UserQuotaConfig)
		tdm.ctx.DB().Delete(user)
		return true
	})

	// Clear all tracking maps
	tdm.createdUsageDetails = sync.Map{}
	tdm.createdGrants = sync.Map{}
	tdm.createdPlans = sync.Map{}
	tdm.createdUsers = sync.Map{}
}

// CleanupWithContext removes all tracked test entities from the database with context parameter
func (tdm *TestDataManager) CleanupWithContext(ctx coreTesting.TestContext) {
	tdm.Cleanup()
}

// Convenience methods that match the old interface
func (tdm *TestDataManager) NextUserID() uint {
	return tdm.GenerateUserID()
}

func (tdm *TestDataManager) NextUploadID() uint {
	return tdm.GenerateUploadID()
}

func (tdm *TestDataManager) NextPlanID() uint {
	return tdm.GeneratePlanID()
}

func (tdm *TestDataManager) NextGrantID() uint {
	return tdm.GenerateGrantID()
}

// TrackCreatedUser tracks a user ID for cleanup (for backward compatibility)
func (tdm *TestDataManager) TrackCreatedUser(userID uint) {
	// This is a no-op since users are now tracked internally
}

// TrackCreatedPlan tracks a plan ID for cleanup (for backward compatibility)
func (tdm *TestDataManager) TrackCreatedPlan(planID uint) {
	// This is a no-op since plans are now tracked internally
}

// TrackCreatedGrant tracks a grant ID for cleanup (for backward compatibility)
func (tdm *TestDataManager) TrackCreatedGrant(grantID uint) {
	// This is a no-op since grants are now tracked internally
}

// TrackCreatedUsageDetail tracks a usage detail ID for cleanup (for backward compatibility)
func (tdm *TestDataManager) TrackCreatedUsageDetail(usageDetailID uint) {
	// This is a no-op since usage details are now tracked internally
}

// TestUserLimits represents test user quota limits
type TestUserLimits struct {
	StorageLimitBytes   *int64
	UploadLimitBytes    *int64
	DownloadLimitBytes  *int64
	StorageThreshold    *int64
	UploadThreshold     *int64
	DownloadThreshold   *int64
	QuotaPlanID         *uint64
	WindowType          *string
	WindowDuration      *int64
	WindowStartHour     *int
	WindowTimezone      *string
}
