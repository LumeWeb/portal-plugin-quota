package managers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// Test constants for byte values
const (
	testGrantBytesSmall  = 1000
	testGrantBytesMedium = 5000
	testGrantBytesLarge  = 10000
	testGrantBytesHuge   = 50000
	testConsumptionBytes = 1500
)

// TestGrantManager_CreateAllowanceGrant_ValidInput_Success tests the CreateAllowanceGrant method
func TestGrantManager_CreateAllowanceGrant_ValidInput_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		grant := &pluginModels.AllowanceGrant{
			Type:   pluginModels.GrantTypeUpload,
			Source: pluginModels.GrantSourceSubscription,
			Bytes:  testGrantBytesLarge,
		}

		err := grantManager.CreateAllowanceGrant(ctx, userID, grant)
		require.NoError(t, err)

		// Verify the grant was created
		var savedGrant pluginModels.AllowanceGrant
		err = ctx.DB().Where("user_id = ?", userID).First(&savedGrant).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedGrant.UserID)
		assert.Equal(t, pluginModels.GrantTypeUpload, savedGrant.Type)
		assert.Equal(t, pluginModels.GrantSourceSubscription, savedGrant.Source)
		assert.Equal(t, uint64(testGrantBytesLarge), savedGrant.Bytes)
		assert.Equal(t, uint64(0), savedGrant.BytesUsed)
		assert.Equal(t, uint64(testGrantBytesLarge), savedGrant.BytesRemaining)
		assert.True(t, savedGrant.IsActive)
		assert.Nil(t, savedGrant.ExpiryDate)

	}, pluginTesting.TestOptions())
}

// createUserUsageDetail creates a UserUsageDetail record for testing consumption
func createUserUsageDetail(ctx coreTesting.TestContext, userID uint, usageType pluginModels.UsageType) (*pluginModels.UserUsageDetail, error) {
	usageDetail := &pluginModels.UserUsageDetail{
		UserID:     userID,
		UploadID:   1, // Default upload ID for tests
		Type:       usageType,
		Bytes:      1000,
		IP:         "127.0.0.1",
		SharedWith: 1,
		Timestamp:  time.Now().UTC(),
	}

	err := ctx.DB().Create(usageDetail).Error
	if err != nil {
		return nil, err
	}

	return usageDetail, nil
}

// TestGrantManager_CreateAllowanceGrant_WithExpiryDate_Success tests creating a grant with expiry date
func TestGrantManager_CreateAllowanceGrant_WithExpiryDate_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		expiryDate := time.Now().UTC().Add(30 * 24 * time.Hour) // 30 days from now
		grant := &pluginModels.AllowanceGrant{
			Type:       pluginModels.GrantTypeStorage,
			Source:     pluginModels.GrantSourcePAYGAddon,
			Bytes:      testGrantBytesLarge,
			ExpiryDate: &expiryDate,
		}

		err := grantManager.CreateAllowanceGrant(ctx, userID, grant)
		require.NoError(t, err)

		// Verify the grant was created with expiry date
		var savedGrant pluginModels.AllowanceGrant
		err = ctx.DB().Where("user_id = ?", userID).First(&savedGrant).Error
		require.NoError(t, err)
		assert.Equal(t, userID, savedGrant.UserID)
		assert.Equal(t, pluginModels.GrantTypeStorage, savedGrant.Type)
		assert.Equal(t, pluginModels.GrantSourcePAYGAddon, savedGrant.Source)
		assert.Equal(t, uint64(testGrantBytesLarge), savedGrant.Bytes)
		assert.Equal(t, uint64(0), savedGrant.BytesUsed)
		assert.Equal(t, uint64(testGrantBytesLarge), savedGrant.BytesRemaining)
		assert.True(t, savedGrant.IsActive)
		assert.Equal(t, expiryDate.UTC().Truncate(time.Second), savedGrant.ExpiryDate.UTC().Truncate(time.Second))

	}, pluginTesting.TestOptions())
}

// TestGrantManager_CreateAllowanceGrant_InvalidInput_Error tests error cases
func TestGrantManager_CreateAllowanceGrant_InvalidInput_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(0) // Invalid user ID

		grant := &pluginModels.AllowanceGrant{
			Type:   pluginModels.GrantTypeUpload,
			Source: pluginModels.GrantSourceSubscription,
			Bytes:  testGrantBytesLarge,
		}

		err := grantManager.CreateAllowanceGrant(ctx, userID, grant)
		assert.Error(t, err)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidUserID)

	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		grant := &pluginModels.AllowanceGrant{
			Type:   "INVALID_TYPE", // Invalid grant type
			Source: pluginModels.GrantSourceSubscription,
			Bytes:  testGrantBytesLarge,
		}

		err := grantManager.CreateAllowanceGrant(ctx, userID, grant)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "grant type is invalid")

	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		grant := &pluginModels.AllowanceGrant{
			Type:   pluginModels.GrantTypeUpload,
			Source: "INVALID_SOURCE", // Invalid grant source
			Bytes:  testGrantBytesLarge,
		}

		err := grantManager.CreateAllowanceGrant(ctx, userID, grant)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "grant source is invalid")

	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		grant := &pluginModels.AllowanceGrant{
			Type:   pluginModels.GrantTypeUpload,
			Source: pluginModels.GrantSourceSubscription,
			Bytes:  0, // Invalid bytes
		}

		err := grantManager.CreateAllowanceGrant(ctx, userID, grant)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "bytes must be greater than 0")

	}, pluginTesting.TestOptions())
}

// TestGrantManager_GetActiveGrantsByType_NoGrants_ReturnsEmpty tests getting grants when none exist
func TestGrantManager_GetActiveGrantsByType_NoGrants_ReturnsEmpty(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		grants, err := grantManager.GetActiveGrantsByType(ctx, userID, pluginCore.GrantTypeUpload)
		require.NoError(t, err)
		assert.Empty(t, grants)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_GetActiveGrantsByType_WithActiveGrants_ReturnsGrants tests getting active grants
func TestGrantManager_GetActiveGrantsByType_WithActiveGrants_ReturnsGrants(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create multiple grants of different types
		grant1 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceSubscription,
			Bytes:    testGrantBytesLarge,
			IsActive: true,
		}
		err := ctx.DB().Create(grant1).Error
		require.NoError(t, err)

		grant2 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeDownload,
			Source:   pluginModels.GrantSourceBonus,
			Bytes:    testGrantBytesMedium,
			IsActive: true,
		}
		err = ctx.DB().Create(grant2).Error
		require.NoError(t, err)

		grant3 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourcePromo,
			Bytes:    testGrantBytesSmall,
			IsActive: true,
		}
		err = ctx.DB().Create(grant3).Error
		require.NoError(t, err)

		// Get upload grants
		grants, err := grantManager.GetActiveGrantsByType(ctx, userID, pluginCore.GrantTypeUpload)
		require.NoError(t, err)
		assert.Len(t, grants, 2)

		// Verify grants are sorted by priority (Promo should come before Subscription)
		assert.Equal(t, pluginModels.GrantSourcePromo, grants[0].Source)
		assert.Equal(t, pluginModels.GrantSourceSubscription, grants[1].Source)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_GetActiveGrantsByType_WithExpiredGrants_FiltersOutExpired tests filtering expired grants
func TestGrantManager_GetActiveGrantsByType_WithExpiredGrants_FiltersOutExpired(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create an active grant
		activeGrant := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceBonus,
			Bytes:    testGrantBytesMedium,
			IsActive: true,
		}
		err := ctx.DB().Create(activeGrant).Error
		require.NoError(t, err)

		// Insert an expired grant directly using raw SQL to bypass validation
		now := time.Now().UTC()
		expiryDate := now.Add(-24 * time.Hour) // Yesterday
		result := ctx.DB().Exec(`INSERT INTO allowance_grants 
			(user_id, type, source, bytes, bytes_used, bytes_remaining, expiry_date, is_active, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			userID, pluginModels.GrantTypeUpload, pluginModels.GrantSourceSubscription,
			testGrantBytesLarge, 0, testGrantBytesLarge, expiryDate, true, now, now)
		require.NoError(t, result.Error)

		// Get upload grants - should only return the active one
		grants, err := grantManager.GetActiveGrantsByType(ctx, userID, pluginCore.GrantTypeUpload)
		require.NoError(t, err)
		assert.Len(t, grants, 1)
		assert.Equal(t, pluginModels.GrantSourceBonus, grants[0].Source)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_GetActiveGrants_WithMultipleTypes_ReturnsAll tests getting all active grants
func TestGrantManager_GetActiveGrants_WithMultipleTypes_ReturnsAll(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create grants of different types
		grant1 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceSubscription,
			Bytes:    testGrantBytesLarge,
			IsActive: true,
		}
		err := ctx.DB().Create(grant1).Error
		require.NoError(t, err)

		grant2 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeDownload,
			Source:   pluginModels.GrantSourceBonus,
			Bytes:    testGrantBytesMedium,
			IsActive: true,
		}
		err = ctx.DB().Create(grant2).Error
		require.NoError(t, err)

		grant3 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeStorage,
			Source:   pluginModels.GrantSourcePromo,
			Bytes:    testGrantBytesSmall,
			IsActive: true,
		}
		err = ctx.DB().Create(grant3).Error
		require.NoError(t, err)

		// Get all active grants
		grants, err := grantManager.GetActiveGrants(ctx, userID)
		require.NoError(t, err)
		assert.Len(t, grants, 3)

		// Verify grants are sorted by priority (based on GetGrantPriority values)
		// Promo: 3, Bonus: 2, Subscription: 1
		assert.Equal(t, pluginModels.GrantSourcePromo, grants[0].Source)        // Highest priority among our test grants
		assert.Equal(t, pluginModels.GrantSourceBonus, grants[1].Source)        // Medium priority
		assert.Equal(t, pluginModels.GrantSourceSubscription, grants[2].Source) // Lowest priority

	}, pluginTesting.TestOptions())
}

// TestGrantManager_CalculateAvailableBytes_CalculatesTotal tests calculating available bytes
func TestGrantManager_CalculateAvailableBytes_CalculatesTotal(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)

		grants := []*pluginModels.AllowanceGrant{
			{BytesRemaining: 1000},
			{BytesRemaining: 2000},
			{BytesRemaining: 3000},
		}

		total := grantManager.CalculateAvailableBytes(grants)
		assert.Equal(t, uint64(6000), total)

	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)

		// Test with empty slice
		grants := []*pluginModels.AllowanceGrant{}
		total := grantManager.CalculateAvailableBytes(grants)
		assert.Equal(t, uint64(0), total)

	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)

		// Test with grants having zero remaining bytes
		grants := []*pluginModels.AllowanceGrant{
			{BytesRemaining: 0},
			{BytesRemaining: 0},
			{BytesRemaining: 1000},
		}

		total := grantManager.CalculateAvailableBytes(grants)
		assert.Equal(t, uint64(1000), total)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_ConsumeFromGrants_SufficientAllowance_Success tests consuming from grants
func TestGrantManager_ConsumeFromGrants_SufficientAllowance_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create a usage detail record first
		usageDetail, err := createUserUsageDetail(ctx, userID, pluginModels.UsageTypeUpload)
		require.NoError(t, err)

		// Create grants with different priorities
		grant1 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceSubscription, // Lowest priority
			Bytes:    testGrantBytesLarge,
			IsActive: true,
		}
		err = ctx.DB().Create(grant1).Error
		require.NoError(t, err)

		grant2 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourcePromo, // Highest priority
			Bytes:    testGrantBytesMedium,
			IsActive: true,
		}
		err = ctx.DB().Create(grant2).Error
		require.NoError(t, err)

		// Consume bytes - should consume from highest priority grant first
		consumptions, err := grantManager.ConsumeFromGrants(ctx, userID, pluginModels.GrantTypeUpload, testConsumptionBytes, usageDetail.ID, nil)
		require.NoError(t, err)
		require.Len(t, consumptions, 1)
		assert.Equal(t, grant2.ID, consumptions[0].GrantID)
		assert.Equal(t, usageDetail.ID, consumptions[0].UsageDetailID)
		assert.Equal(t, uint64(testConsumptionBytes), consumptions[0].BytesConsumed)

		// Verify grants were updated
		var updatedGrant1, updatedGrant2 pluginModels.AllowanceGrant
		err = ctx.DB().Where("id = ?", grant1.ID).First(&updatedGrant1).Error
		require.NoError(t, err)
		err = ctx.DB().Where("id = ?", grant2.ID).First(&updatedGrant2).Error
		require.NoError(t, err)

		// Grant2 (Promo) should have been consumed from since it has higher priority
		assert.Equal(t, uint64(testConsumptionBytes), updatedGrant2.BytesUsed)
		assert.Equal(t, uint64(testGrantBytesMedium-testConsumptionBytes), updatedGrant2.BytesRemaining)

		// Grant1 (Subscription) should be unchanged
		assert.Equal(t, uint64(0), updatedGrant1.BytesUsed)
		assert.Equal(t, uint64(testGrantBytesLarge), updatedGrant1.BytesRemaining)

		// Verify consumption records were created
		var consumptionsFromDB []pluginModels.AllowanceConsumption
		err = ctx.DB().Where("usage_detail_id = ?", usageDetail.ID).Find(&consumptionsFromDB).Error
		require.NoError(t, err)
		assert.Len(t, consumptionsFromDB, 1)
		assert.Equal(t, uint64(testConsumptionBytes), consumptionsFromDB[0].BytesConsumed)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_ConsumeFromGrants_InsufficientAllowance_Error tests insufficient allowance error
func TestGrantManager_ConsumeFromGrants_InsufficientAllowance_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create a usage detail record first
		usageDetail, err := createUserUsageDetail(ctx, userID, pluginModels.UsageTypeUpload)
		require.NoError(t, err)

		// Create a grant with insufficient allowance
		grant := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceSubscription,
			Bytes:    testGrantBytesSmall,
			IsActive: true,
		}
		err = ctx.DB().Create(grant).Error
		require.NoError(t, err)

		// Try to consume more bytes than available
		consumptions, err := grantManager.ConsumeFromGrants(ctx, userID, pluginModels.GrantTypeUpload, testGrantBytesHuge, usageDetail.ID, nil)
		assert.Error(t, err)
		assert.Nil(t, consumptions)
		assert.Equal(t, pluginModels.ErrInsufficientAllowance, err)

		// Verify grant was not modified
		var updatedGrant pluginModels.AllowanceGrant
		err = ctx.DB().Where("id = ?", grant.ID).First(&updatedGrant).Error
		require.NoError(t, err)
		assert.Equal(t, uint64(0), updatedGrant.BytesUsed)
		assert.Equal(t, uint64(testGrantBytesSmall), updatedGrant.BytesRemaining)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_ConsumeFromGrants_MultipleGrants_Success tests consuming from multiple grants
func TestGrantManager_ConsumeFromGrants_MultipleGrants_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create a usage detail record first
		usageDetail, err := createUserUsageDetail(ctx, userID, pluginModels.UsageTypeUpload)
		require.NoError(t, err)

		// Create grants with enough total bytes
		grant1 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceSubscription, // Lowest priority
			Bytes:    testGrantBytesHuge,                   // 50000 bytes
			IsActive: true,
		}
		err = ctx.DB().Create(grant1).Error
		require.NoError(t, err)

		grant2 := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourcePromo, // Highest priority
			Bytes:    testGrantBytesMedium,          // 5000 bytes
			IsActive: true,
		}
		err = ctx.DB().Create(grant2).Error
		require.NoError(t, err)

		// Consume more bytes than the highest priority grant has, but less than total
		totalConsumption := uint64(testGrantBytesMedium + testConsumptionBytes) // 5000 + 1500 = 6500

		// Consume from grants using the manager method
		consumptions, err := grantManager.ConsumeFromGrants(ctx, userID, pluginModels.GrantTypeUpload, totalConsumption, usageDetail.ID, nil)
		require.NoError(t, err)
		require.Len(t, consumptions, 2)

		// Verify grants were updated
		var updatedGrant1, updatedGrant2 pluginModels.AllowanceGrant
		err = ctx.DB().Where("id = ?", grant1.ID).First(&updatedGrant1).Error
		require.NoError(t, err)
		err = ctx.DB().Where("id = ?", grant2.ID).First(&updatedGrant2).Error
		require.NoError(t, err)

		// Grant2 (Promo) should be fully consumed first as it has higher priority
		assert.Equal(t, uint64(testGrantBytesMedium), updatedGrant2.BytesUsed)
		assert.Equal(t, uint64(0), updatedGrant2.BytesRemaining)

		// Grant1 (Subscription) should be partially consumed for the remaining bytes
		expectedGrant1Used := totalConsumption - testGrantBytesMedium
		expectedGrant1Remaining := testGrantBytesHuge - expectedGrant1Used
		assert.Equal(t, expectedGrant1Used, updatedGrant1.BytesUsed)
		assert.Equal(t, expectedGrant1Remaining, updatedGrant1.BytesRemaining)

		// Verify consumption records were created
		var consumptionsFromDB []pluginModels.AllowanceConsumption
		err = ctx.DB().Where("usage_detail_id = ?", usageDetail.ID).Find(&consumptionsFromDB).Error
		require.NoError(t, err)
		assert.Len(t, consumptionsFromDB, 2)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_DeactivateGrant_ValidID_Success tests deactivating a grant
func TestGrantManager_DeactivateGrant_ValidID_Success(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create an active grant
		grant := &pluginModels.AllowanceGrant{
			UserID:   userID,
			Type:     pluginModels.GrantTypeUpload,
			Source:   pluginModels.GrantSourceSubscription,
			Bytes:    testGrantBytesLarge,
			IsActive: true,
		}
		err := ctx.DB().Create(grant).Error
		require.NoError(t, err)

		// Deactivate the grant
		err = grantManager.DeactivateGrant(ctx, grant.ID)
		require.NoError(t, err)

		// Verify the grant was deactivated
		var updatedGrant pluginModels.AllowanceGrant
		err = ctx.DB().Where("id = ?", grant.ID).First(&updatedGrant).Error
		require.NoError(t, err)
		assert.False(t, updatedGrant.IsActive)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_DeactivateGrant_InvalidID_Error tests deactivating with invalid ID
func TestGrantManager_DeactivateGrant_InvalidID_Error(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)

		// Try to deactivate a non-existent grant
		err := grantManager.DeactivateGrant(ctx, 999999)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "grant not found")

	}, pluginTesting.TestOptions())

	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)

		// Try to deactivate with invalid ID (0)
		err := grantManager.DeactivateGrant(ctx, 0)
		assert.Error(t, err)
		assert.ErrorIs(t, err, pluginModels.ErrInvalidGrantID)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_GetExpiringGrants_WithExpiringGrants_ReturnsGrants tests getting expiring grants
func TestGrantManager_GetExpiringGrants_WithExpiringGrants_ReturnsGrants(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)

		// Create grants expiring at different times
		now := time.Now().UTC()
		expiry1 := now.Add(24 * time.Hour).UTC() // 1 day from now
		expiry2 := now.Add(48 * time.Hour).UTC() // 2 days from now
		expiry3 := now.Add(72 * time.Hour).UTC() // 3 days from now

		grant1 := &pluginModels.AllowanceGrant{
			UserID:     1,
			Type:       pluginModels.GrantTypeUpload,
			Source:     pluginModels.GrantSourceSubscription,
			Bytes:      testGrantBytesLarge,
			ExpiryDate: &expiry1,
			IsActive:   true,
		}
		err := ctx.DB().Create(grant1).Error
		require.NoError(t, err)

		grant2 := &pluginModels.AllowanceGrant{
			UserID:     2,
			Type:       pluginModels.GrantTypeDownload,
			Source:     pluginModels.GrantSourceBonus,
			Bytes:      testGrantBytesMedium,
			ExpiryDate: &expiry2,
			IsActive:   true,
		}
		err = ctx.DB().Create(grant2).Error
		require.NoError(t, err)

		grant3 := &pluginModels.AllowanceGrant{
			UserID:     3,
			Type:       pluginModels.GrantTypeStorage,
			Source:     pluginModels.GrantSourcePromo,
			Bytes:      testGrantBytesSmall,
			ExpiryDate: &expiry3,
			IsActive:   true,
		}
		err = ctx.DB().Create(grant3).Error
		require.NoError(t, err)

		// Get grants expiring within 3 days
		grants, err := grantManager.GetExpiringGrants(ctx, 72*time.Hour)
		require.NoError(t, err)
		assert.Len(t, grants, 3)

		// Verify grants are sorted by expiration date (earliest first)
		assert.Equal(t, grant1.ID, grants[0].ID)
		assert.Equal(t, grant2.ID, grants[1].ID)
		assert.Equal(t, grant3.ID, grants[2].ID)

	}, pluginTesting.TestOptions())
}

// TestGrantManager_GetExpiringGrantsForUser_WithUserGrants_ReturnsGrants tests getting expiring grants for a user
func TestGrantManager_GetExpiringGrantsForUser_WithUserGrants_ReturnsGrants(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		grantManager := NewGrantManager(ctx)
		userID := uint(1)

		// Create grants for the user, some expiring soon
		now := time.Now().UTC()
		expiry1 := now.Add(24 * time.Hour).UTC() // 1 day from now
		expiry2 := now.Add(48 * time.Hour).UTC() // 2 days from now

		grant1 := &pluginModels.AllowanceGrant{
			UserID:     userID,
			Type:       pluginModels.GrantTypeUpload,
			Source:     pluginModels.GrantSourceSubscription,
			Bytes:      testGrantBytesLarge,
			ExpiryDate: &expiry1,
			IsActive:   true,
		}
		err := ctx.DB().Create(grant1).Error
		require.NoError(t, err)

		grant2 := &pluginModels.AllowanceGrant{
			UserID:     userID,
			Type:       pluginModels.GrantTypeUpload,
			Source:     pluginModels.GrantSourceBonus,
			Bytes:      testGrantBytesMedium,
			ExpiryDate: &expiry2,
			IsActive:   true,
		}
		err = ctx.DB().Create(grant2).Error
		require.NoError(t, err)

		// Create a grant for another user
		grant3 := &pluginModels.AllowanceGrant{
			UserID:     2,
			Type:       pluginModels.GrantTypeUpload,
			Source:     pluginModels.GrantSourcePromo,
			Bytes:      testGrantBytesSmall,
			ExpiryDate: &expiry1,
			IsActive:   true,
		}
		err = ctx.DB().Create(grant3).Error
		require.NoError(t, err)

		// Get expiring grants for the user within 3 days
		grants, err := grantManager.GetExpiringGrantsForUser(ctx, userID, 72*time.Hour)
		require.NoError(t, err)
		assert.Len(t, grants, 2)

		// Verify only user's grants are returned, sorted by expiration date
		assert.Equal(t, grant1.ID, grants[0].ID)
		assert.Equal(t, grant2.ID, grants[1].ID)

	}, pluginTesting.TestOptions())
}
