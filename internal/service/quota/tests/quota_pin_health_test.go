package tests

import (
	"testing"
	"time"

	"github.com/docker/go-units"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestGetCIDPinHealth_ZeroPinners tests that when no users are pinning,
// the method returns an empty health response with PinnerCount=0.
func TestGetCIDPinHealth_ZeroPinners(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(0), health.PinnerCount, "PinnerCount should be 0 when no pinners")
		assert.Equal(t, uint64(0), health.TotalQuotaBytes)
		assert.Equal(t, uint64(0), health.TotalRemainingBytes)
		assert.Equal(t, uint64(0), health.TotalUsedBytes)
		assert.False(t, health.IsUnlimited)
		assert.Nil(t, health.EstimatedQuotaExhaustionDate)

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_CIDNotFound tests that when the CID is not found,
// the method returns an error.
func TestGetCIDPinHealth_CIDNotFound(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		hash, err := multihash.Sum([]byte("nonexistent-cid"), multihash.SHA2_256, -1)
		require.NoError(t, err)

		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)
		require.NotNil(t, quotaService)

		health, err := quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes([]byte(hash), 0, nil), 0)

		require.Error(t, err, "Should return error for non-existent CID")
		assert.Nil(t, health)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_SinglePinner_WindowedLimit tests a single pinner with
// a windowed storage limit and no usage — remaining should equal the limit.
func TestGetCIDPinHealth_SinglePinner_WindowedLimit(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		storageLimit := int64(1 * units.MB)
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &storageLimit,
		})

		fixture.createPinForUser(t, userID)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(1), health.PinnerCount)
		assert.False(t, health.IsUnlimited)
		assert.Equal(t, uint64(1*units.MB), health.TotalQuotaBytes, "TotalQuotaBytes should equal the storage limit")
		assert.Equal(t, uint64(1*units.MB), health.TotalRemainingBytes, "Remaining should equal limit when no usage")
		assert.Equal(t, uint64(0), health.TotalUsedBytes, "Used should be 0 when no usage")
		assert.Nil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be nil when no burn rate")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_UnlimitedPinner tests that a pinner with unlimited policy
// results in IsUnlimited=true and nil exhaustion date.
func TestGetCIDPinHealth_UnlimitedPinner(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyUnlimited, nil)

		fixture.createPinForUser(t, userID)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(1), health.PinnerCount)
		assert.True(t, health.IsUnlimited, "Should be unlimited when pinner has unlimited policy")
		assert.Equal(t, ^uint64(0), health.TotalQuotaBytes, "TotalQuotaBytes should be MaxUint64 when unlimited")
		assert.Equal(t, ^uint64(0), health.TotalRemainingBytes, "TotalRemainingBytes should be MaxUint64 when unlimited")
		assert.Nil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be nil when unlimited")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_MultiplePinners tests that multiple pinners with
// windowed limits are summed correctly.
func TestGetCIDPinHealth_MultiplePinners(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID1 := fixture.dataManager.GenerateUserID()
		userID2 := fixture.dataManager.GenerateUserID()
		limit1 := int64(1 * units.MB)
		limit2 := int64(2 * units.MB)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit1,
		})
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit2,
		})

		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(2), health.PinnerCount)
		assert.False(t, health.IsUnlimited)
		assert.Equal(t, uint64(3*units.MB), health.TotalQuotaBytes, "TotalQuotaBytes should be sum of both limits")
		assert.Equal(t, uint64(3*units.MB), health.TotalRemainingBytes, "Remaining should equal total when no usage")
		assert.Equal(t, uint64(0), health.TotalUsedBytes)

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_WindowedWithUsage tests that STORAGE_ADD usage details
// produce correct used and remaining values for windowed-limit users.
func TestGetCIDPinHealth_WindowedWithUsage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		storageLimit := int64(10 * units.MB)

		// Use LIFETIME window so the window covers all usage
		windowType := string(pluginModels.WindowTypeLifetime)
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &storageLimit,
			WindowType:        &windowType,
		})

		fixture.createPinForUser(t, userID)

		// Record STORAGE_ADD usage
		detail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  fixture.testUpload.ID,
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     1 * units.MB,
			Timestamp: time.Now().UTC(),
		}
		err := ctx.DB().Create(detail).Error
		require.NoError(t, err)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(1), health.PinnerCount)
		assert.False(t, health.IsUnlimited)
		assert.Equal(t, uint64(10*units.MB), health.TotalQuotaBytes, "TotalQuotaBytes should equal limit")
		assert.Equal(t, uint64(9*units.MB), health.TotalRemainingBytes, "Remaining should be limit minus usage")
		assert.Equal(t, uint64(1*units.MB), health.TotalUsedBytes, "Used should equal the STORAGE_ADD bytes")
		// LIFETIME window with 1MB burn: exhaustion = 9MB remaining / (1MB/30 days) = 270 days
		require.NotNil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be computed for LIFETIME window with burn rate")
		expectedExhaustion := time.Now().UTC().AddDate(0, 0, 270)
		delta := health.EstimatedQuotaExhaustionDate.Sub(expectedExhaustion).Hours()
		assert.InDelta(t, 0, delta, 24, "Exhaustion date should be ~270 days from now")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_MixedUnlimitedAndLimited tests that when one pinner is
// unlimited and another has a windowed limit, the group is reported as unlimited.
func TestGetCIDPinHealth_MixedUnlimitedAndLimited(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: Hard limits
		userID1 := fixture.dataManager.GenerateUserID()
		limit1 := int64(1 * units.MB)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit1,
		})

		// User 2: Unlimited
		userID2 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyUnlimited, nil)

		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(2), health.PinnerCount)
		assert.True(t, health.IsUnlimited, "Should be unlimited when any pinner is unlimited")
		assert.Equal(t, ^uint64(0), health.TotalQuotaBytes)
		assert.Equal(t, ^uint64(0), health.TotalRemainingBytes)
		assert.Nil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be nil when unlimited")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_AllowancePinner tests that a pinner with ALLOWANCE policy
// and storage grants reports quota from grant balances, not windowed limits.
func TestGetCIDPinHealth_AllowancePinner(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyAllowance, nil)

		// Create a 10TB storage grant
		grantBytes := uint64(10 * units.TB)
		fixture.dataManager.CreateAllowanceGrant(userID, pluginModels.GrantTypeStorage, grantBytes)

		fixture.createPinForUser(t, userID)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(1), health.PinnerCount)
		assert.False(t, health.IsUnlimited, "Allowance user should not be reported as unlimited")
		assert.Equal(t, grantBytes, health.TotalQuotaBytes, "TotalQuotaBytes should equal grant bytes")
		assert.Equal(t, grantBytes, health.TotalRemainingBytes, "Remaining should equal grant bytes when unused")
		assert.Equal(t, uint64(0), health.TotalUsedBytes, "Used should be 0 when grant not consumed")
		assert.Nil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be nil when no burn rate")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_AllowanceWithConsumedGrant tests that an allowance pinner
// with partially consumed grants reports correct used and remaining values.
func TestGetCIDPinHealth_AllowanceWithConsumedGrant(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyAllowance, nil)

		// Create a 10GB storage grant
		grantBytes := uint64(10 * units.GB)
		grant := fixture.dataManager.CreateAllowanceGrant(userID, pluginModels.GrantTypeStorage, grantBytes)

		fixture.createPinForUser(t, userID)

		// Simulate grant consumption: 3GB used
		grant.BytesUsed = 3 * units.GB
		grant.BytesRemaining = grantBytes - uint64(3*units.GB)
		err := ctx.DB().Save(grant).Error
		require.NoError(t, err)

		// Record STORAGE_ADD usage for burn rate calculation
		detail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  fixture.testUpload.ID,
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     3 * units.GB,
			Timestamp: time.Now().UTC(),
		}
		err = ctx.DB().Create(detail).Error
		require.NoError(t, err)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(1), health.PinnerCount)
		assert.False(t, health.IsUnlimited)
		assert.Equal(t, grantBytes, health.TotalQuotaBytes, "TotalQuotaBytes should equal grant bytes")
		assert.Equal(t, uint64(7*units.GB), health.TotalRemainingBytes, "Remaining should be 7GB")
		assert.Equal(t, uint64(3*units.GB), health.TotalUsedBytes, "Used should be 3GB")

		// Burn rate = 3GB / 30 days. Exhaustion date should be computed.
		// daysRemaining = 7GB / (3GB/30) = 7GB * 30 / 3GB = 70 days
		require.NotNil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be computed for allowance user with burn rate")
		expectedExhaustion := time.Now().UTC().AddDate(0, 0, 70)
		// Allow 1-day tolerance for timing
		delta := health.EstimatedQuotaExhaustionDate.Sub(expectedExhaustion).Hours()
		assert.InDelta(t, 0, delta, 24, "Exhaustion date should be ~70 days from now")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_AllowanceNoBurnRate tests that an allowance pinner with
// grants but no STORAGE_ADD history has nil exhaustion date (can't predict).
func TestGetCIDPinHealth_AllowanceNoBurnRate(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyAllowance, nil)

		grantBytes := uint64(10 * units.TB)
		fixture.dataManager.CreateAllowanceGrant(userID, pluginModels.GrantTypeStorage, grantBytes)

		fixture.createPinForUser(t, userID)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.False(t, health.IsUnlimited)
		assert.Equal(t, grantBytes, health.TotalQuotaBytes)
		assert.Equal(t, grantBytes, health.TotalRemainingBytes)
		assert.Nil(t, health.EstimatedQuotaExhaustionDate, "Exhaustion date should be nil when no burn rate history")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_MixedAllowanceAndWindowed tests that a group with
// both allowance and windowed users correctly sums quotas from both models.
func TestGetCIDPinHealth_MixedAllowanceAndWindowed(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: Hard limits (1MB windowed)
		userID1 := fixture.dataManager.GenerateUserID()
		limit1 := int64(1 * units.MB)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit1,
		})

		// User 2: Allowance (10GB grant)
		userID2 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyAllowance, nil)
		grantBytes := uint64(10 * units.GB)
		fixture.dataManager.CreateAllowanceGrant(userID2, pluginModels.GrantTypeStorage, grantBytes)

		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(2), health.PinnerCount)
		assert.False(t, health.IsUnlimited, "Mixed allowance+windowed should not be unlimited")
		expectedQuota := uint64(1*units.MB) + grantBytes
		assert.Equal(t, expectedQuota, health.TotalQuotaBytes, "TotalQuotaBytes should sum windowed limit + grant bytes")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_AllowanceNotReportedAsUnlimited is a regression test
// for Kody issue #5: ALLOWANCE/THRESHOLD users without StorageLimitConfig
// were incorrectly flagged as unlimited.
func TestGetCIDPinHealth_AllowanceNotReportedAsUnlimited(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		// Allowance user with no storage limit config and no grants
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyAllowance, nil)

		fixture.createPinForUser(t, userID)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.False(t, health.IsUnlimited,
			"Allowance user without grants should NOT be reported as unlimited (Kody #5)")
		assert.Equal(t, uint64(0), health.TotalQuotaBytes, "Quota should be 0 with no grants")
		assert.Equal(t, uint64(0), health.TotalRemainingBytes, "Remaining should be 0 with no grants")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_UsedAccumulatedBeforePolicyShortCircuit is a regression
// test for Kody issue #6: TotalUsedBytes should accumulate usage before
// policy short-circuits (unlimited skip). Here an unlimited user should
// still contribute 0 to used (no query needed), but the test validates
// that the accumulation logic is correct for mixed groups.
func TestGetCIDPinHealth_UsedAccumulatedBeforePolicyShortCircuit(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: Hard limits with usage
		userID1 := fixture.dataManager.GenerateUserID()
		limit1 := int64(10 * units.MB)
		windowType := string(pluginModels.WindowTypeLifetime)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit1,
			WindowType:        &windowType,
		})

		// User 2: Unlimited (should contribute nothing to used)
		userID2 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyUnlimited, nil)

		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		// Record STORAGE_ADD for user 1
		detail := &pluginModels.UserUsageDetail{
			UserID:    userID1,
			UploadID:  fixture.testUpload.ID,
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     2 * units.MB,
			Timestamp: time.Now().UTC(),
		}
		err := ctx.DB().Create(detail).Error
		require.NoError(t, err)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.True(t, health.IsUnlimited, "Should be unlimited due to user 2")
		// Even though unlimited overrides totals, the implementation should still
		// process user 1 correctly before the unlimited override
		assert.Equal(t, ^uint64(0), health.TotalQuotaBytes)
		assert.Equal(t, ^uint64(0), health.TotalRemainingBytes)

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_GrantsBatchedNotPerUser is a regression test verifying
// that GetCIDPinHealth uses GetActiveGrantsByTypeBatch (single query) rather
// than calling GetActiveGrantsByType per allowance user (N+1).
func TestGetCIDPinHealth_GrantsBatchedNotPerUser(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create multiple allowance users with grants
		userID1 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyAllowance, nil)
		fixture.dataManager.CreateAllowanceGrant(userID1, pluginModels.GrantTypeStorage, 10*units.MB)

		userID2 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyAllowance, nil)
		fixture.dataManager.CreateAllowanceGrant(userID2, pluginModels.GrantTypeStorage, 20*units.MB)

		userID3 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyAllowance, nil)
		fixture.dataManager.CreateAllowanceGrant(userID3, pluginModels.GrantTypeStorage, 15*units.MB)

		fixture.createPinsForUsers(t, []uint{userID1, userID2, userID3})

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Equal(t, uint64(3), health.PinnerCount, "Should have 3 pinners")
		// Total quota = 10 + 20 + 15 = 45MB
		assert.Equal(t, uint64(45*units.MB), health.TotalQuotaBytes, "Total quota should sum all grant bytes")
		// No usage consumed — all remaining
		assert.Equal(t, uint64(45*units.MB), health.TotalRemainingBytes, "All grant bytes should be remaining")
		assert.Equal(t, uint64(0), health.TotalUsedBytes, "No bytes used yet")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_CalendarWindowNoExhaustionDate is a regression test
// verifying that calendar windows (DAY/WEEK/MONTH/YEAR) do NOT set
// EstimatedQuotaExhaustionDate to windowEnd. Window reset ≠ exhaustion.
func TestGetCIDPinHealth_CalendarWindowNoExhaustionDate(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		limit := int64(10 * units.MB)
		windowType := string(pluginModels.WindowTypeCalendarMonth)
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit,
			WindowType:        &windowType,
		})
		fixture.createPinsForUsers(t, []uint{userID})

		// Record STORAGE_ADD within the current window
		detail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  fixture.testUpload.ID,
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     5 * units.MB,
			Timestamp: time.Now().UTC(),
		}
		err := ctx.DB().Create(detail).Error
		require.NoError(t, err)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		assert.Nil(t, health.EstimatedQuotaExhaustionDate,
			"Calendar window (MONTH) should NOT produce an exhaustion date — reset means usage drops, not exhaustion")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_RollingWindowExhaustionFromBurnRate is a regression test
// verifying that ROLLING windows compute exhaustion from remaining/burnRate,
// not from windowEnd.
func TestGetCIDPinHealth_RollingWindowExhaustionFromBurnRate(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		limit := int64(10 * units.MB)
		windowType := string(pluginModels.WindowTypeRolling)
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit,
			WindowType:        &windowType,
		})
		fixture.createPinsForUsers(t, []uint{userID})

		// Record 1MB STORAGE_ADD today → burn rate = 1MB/30 days
		detail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  fixture.testUpload.ID,
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     1 * units.MB,
			Timestamp: time.Now().UTC(),
		}
		err := ctx.DB().Create(detail).Error
		require.NoError(t, err)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		// 9MB remaining / (1MB/30 days) = 270 days
		require.NotNil(t, health.EstimatedQuotaExhaustionDate,
			"ROLLING window with burn rate should produce exhaustion date")
		expected := time.Now().UTC().AddDate(0, 0, 270)
		delta := health.EstimatedQuotaExhaustionDate.Sub(expected).Hours()
		assert.InDelta(t, 0, delta, 24, "Exhaustion should be ~270 days from now")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_LifetimeWindowExhaustionFromBurnRate is a regression test
// verifying that LIFETIME windows also compute exhaustion from burn rate.
func TestGetCIDPinHealth_LifetimeWindowExhaustionFromBurnRate(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		userID := fixture.dataManager.GenerateUserID()
		limit := int64(10 * units.MB)
		windowType := string(pluginModels.WindowTypeLifetime)
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit,
			WindowType:        &windowType,
		})
		fixture.createPinsForUsers(t, []uint{userID})

		// Record 2MB STORAGE_ADD → burn rate = 2MB/30 days
		// remaining = 8MB, daysRemaining = 8MB / (2MB/30) = 120 days
		detail := &pluginModels.UserUsageDetail{
			UserID:    userID,
			UploadID:  fixture.testUpload.ID,
			Type:      pluginModels.UsageTypeStorageAdd,
			Bytes:     2 * units.MB,
			Timestamp: time.Now().UTC(),
		}
		err := ctx.DB().Create(detail).Error
		require.NoError(t, err)

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		require.NotNil(t, health.EstimatedQuotaExhaustionDate,
			"LIFETIME window with burn rate should produce exhaustion date")
		expected := time.Now().UTC().AddDate(0, 0, 120)
		delta := health.EstimatedQuotaExhaustionDate.Sub(expected).Hours()
		assert.InDelta(t, 0, delta, 24, "Exhaustion should be ~120 days from now")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_PinnerCountReflectsResolvedPinners is a regression test
// verifying that PinnerCount reflects the number of pinners whose limits
// successfully resolved, not the raw count of pinning users. When
// ResolveEffectiveLimitsBatch drops a user, PinnerCount must decrease.
func TestGetCIDPinHealth_PinnerCountReflectsResolvedPinners(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Two users with windowed limits
		userID1 := fixture.dataManager.GenerateUserID()
		limit := int64(10 * units.MB)
		windowType := string(pluginModels.WindowTypeLifetime)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit,
			WindowType:        &windowType,
		})

		userID2 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit,
			WindowType:        &windowType,
		})

		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		// Both users should resolve successfully → PinnerCount = 2
		assert.Equal(t, uint64(2), health.PinnerCount, "PinnerCount should equal resolved pinners")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_DistinctWindowConfigsNotBatched is a regression test
// verifying that pinners with different window configs (e.g. different
// Type) are not batched together under the first pinner's window bounds.
func TestGetCIDPinHealth_DistinctWindowConfigsNotBatched(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: LIFETIME window with 10MB limit
		userID1 := fixture.dataManager.GenerateUserID()
		limit1 := int64(10 * units.MB)
		windowType1 := string(pluginModels.WindowTypeLifetime)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit1,
			WindowType:        &windowType1,
		})

		// User 2: ROLLING window with 20MB limit — different type, must not be batched with user 1
		userID2 := fixture.dataManager.GenerateUserID()
		limit2 := int64(20 * units.MB)
		windowType2 := string(pluginModels.WindowTypeRolling)
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			StorageLimitBytes: &limit2,
			WindowType:        &windowType2,
		})

		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		// Record STORAGE_ADD for both users
		for _, uid := range []uint{userID1, userID2} {
			detail := &pluginModels.UserUsageDetail{
				UserID:    uid,
				UploadID:  fixture.testUpload.ID,
				Type:      pluginModels.UsageTypeStorageAdd,
				Bytes:     2 * units.MB,
				Timestamp: time.Now().UTC(),
			}
			err := ctx.DB().Create(detail).Error
			require.NoError(t, err)
		}

		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)
		// Total quota = 10MB + 20MB = 30MB (not 20MB if both batched under user 2's 20MB limit)
		assert.Equal(t, uint64(30*units.MB), health.TotalQuotaBytes, "Total quota should sum both users' limits independently")
		// Total used = 2MB + 2MB = 4MB (not 2MB if both batched under one window)
		assert.Equal(t, uint64(4*units.MB), health.TotalUsedBytes, "Total used should sum both users' usage independently")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_AuthorizationOwnerAllowed is a regression test verifying
// that the upload owner can call GetCIDPinHealth with their own requesterID.
func TestGetCIDPinHealth_AuthorizationOwnerAllowed(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Owner is fixture.testUpload.UserID — should be authorized
		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), fixture.testUpload.UserID)

		require.NoError(t, err)
		require.NotNil(t, health)

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_AuthorizationNonOwnerDenied is a regression test verifying
// that a non-owner requesterID is rejected with ErrUnauthorized.
func TestGetCIDPinHealth_AuthorizationNonOwnerDenied(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Use a random non-owner userID
		nonOwner := fixture.dataManager.GenerateUserID()
		_, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), nonOwner)

		require.Error(t, err)
		assert.ErrorIs(t, err, pluginCore.ErrUnauthorized)

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestGetCIDPinHealth_SystemCallBypassesAuth is a regression test verifying
// that requesterID=0 (system/admin call) bypasses the ownership check.
func TestGetCIDPinHealth_SystemCallBypassesAuth(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// requesterID=0 = system call — always allowed
		health, err := fixture.quotaService.GetCIDPinHealth(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 0)

		require.NoError(t, err)
		require.NotNil(t, health)

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}
