package tests

import (
	"testing"

	"github.com/docker/go-units"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corePlugin "go.lumeweb.com/portal-plugin-core"
	dashboard "go.lumeweb.com/portal-plugin-dashboard"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	plugin "go.lumeweb.com/portal-plugin-quota/internal/plugin"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	coreModels "go.lumeweb.com/portal/db/models"
	serviceTesting "go.lumeweb.com/portal/service/testing"
	"gorm.io/datatypes"
)

// QuotaIntegrationTestFixture provides reusable test setup for integration tests
type QuotaIntegrationTestFixture struct {
	ctx          coreTesting.TestContext
	dataManager  *testdata.TestDataManager
	quotaService pluginCore.QuotaService
	testUpload   *coreModels.Upload
	testCID      cid.Cid
	testHash     []byte // Multihash bytes for StorageHash creation
}

// newQuotaIntegrationTestFixture creates a new test fixture
func newQuotaIntegrationTestFixture(t *testing.T, ctx coreTesting.TestContext) *QuotaIntegrationTestFixture {
	fixture := &QuotaIntegrationTestFixture{
		ctx:         ctx,
		dataManager: testdata.NewTestDataManager(ctx),
	}

	// Get quota service
	fixture.quotaService = core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)
	require.NotNil(t, fixture.quotaService)

	// Create default quota plan
	fixture.dataManager.CreateQuotaPlan("default", 100*units.GB, 5000, 5000, 100*units.MB, 100*units.MB, true)

	// Setup test upload
	fixture.setupTestUpload(t)

	return fixture
}

// setupTestUpload creates a test upload with a CID
func (f *QuotaIntegrationTestFixture) setupTestUpload(t *testing.T) {
	// Generate test CID
	hash, err := multihash.Sum([]byte("test-data"), multihash.SHA2_256, -1)
	require.NoError(t, err)
	f.testCID = cid.NewCidV1(cid.DagCBOR, hash)
	f.testHash = []byte(hash)
	require.NoError(t, err)

	// Create test upload in database
	f.testUpload = &coreModels.Upload{
		UserID:     f.dataManager.GenerateUserID(),
		Hash:       f.testCID.Hash(),
		CIDType:    uint64(f.testCID.Type()),
		Size:       uint64(100 * units.KB),
		MimeType:   "application/octet-stream",
		Protocol:   "ipfs",
		UploaderIP: "127.0.0.1",
		Metadata:   datatypes.JSON("{}"),
	}

	err = f.ctx.DB().Model(f.testUpload).Create(f.testUpload).Error
	require.NoError(t, err)
}

// createPinForUser creates a pin for the test upload
func (f *QuotaIntegrationTestFixture) createPinForUser(t *testing.T, userID uint) *coreModels.Pin {
	pin := &coreModels.Pin{
		UploadID: f.testUpload.ID,
		UserID:   userID,
	}
	err := f.ctx.DB().Create(pin).Error
	require.NoError(t, err)
	return pin
}

// createPinsForUsers creates pins for multiple users
func (f *QuotaIntegrationTestFixture) createPinsForUsers(t *testing.T, userIDs []uint) {
	for _, userID := range userIDs {
		f.createPinForUser(t, userID)
	}
}

// TestCheckCIDGroupQuotaAvailability_AllUsersHaveSufficientQuota tests that when all users
// pinning content have sufficient quota, the method returns true without multiple iterations.
func TestCheckCIDGroupQuotaAvailability_AllUsersHaveSufficientQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create users with high limits
		userID1 := fixture.dataManager.GenerateUserID()
		userID2 := fixture.dataManager.GenerateUserID()
		userID3 := fixture.dataManager.GenerateUserID()

		uploadLimit := int64(1 * units.MB)
		storageLimit := uploadLimit

		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &uploadLimit,
			StorageLimit:     &storageLimit,
		})
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &uploadLimit,
			StorageLimit:     &storageLimit,
		})
		fixture.dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &uploadLimit,
			StorageLimit:     &storageLimit,
		})

		// Create pins for all users
		fixture.createPinsForUsers(t, []uint{userID1, userID2, userID3})

		// Test: 1 KB for 3 users = ~333 bytes each
		// All 3 users have sufficient quota (1 MB each), should return true
		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 1000, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true when all users have sufficient quota")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_SomeUsersInsufficient_MultipleIterations tests that when
// some users lack sufficient quota, the algorithm iteratively filters and re-calculates.
func TestCheckCIDGroupQuotaAvailability_SomeUsersInsufficient_MultipleIterations(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: High quota (10 KB), should handle final cost alone
		userID1 := fixture.dataManager.GenerateUserID()
		highLimit := int64(10 * units.KB)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &highLimit,
			StorageLimit:     &highLimit,
		})

		// User 2: Medium quota (3 KB), may get filtered out eventually
		userID2 := fixture.dataManager.GenerateUserID()
		medLimit := int64(5 * units.KB)
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &medLimit,
			StorageLimit:     &medLimit,
		})

		// User 3: Low quota (1 KB), gets filtered out first
		userID3 := fixture.dataManager.GenerateUserID()
		lowLimit := int64(1 * units.KB)
		fixture.dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &lowLimit,
			StorageLimit:     &lowLimit,
		})

		// Create pins for all users
		fixture.createPinsForUsers(t, []uint{userID1, userID2, userID3})

		// Test: 10 KB for 3 users = 3333 bytes each
		// - User 1 (10 KB): ✓
		// - User 2 (5 KB): ✓
		// - User 3 (1 KB): ✗
		//
		// Iteration 2: 10 KB for 2 users = 5 KB each
		// - User 1 (10 KB): ✓
		// - User 2 (5 KB): ✓
		//
		// Result: Users 1 and 2 can handle → true

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), uint64(10*units.KB), pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true after iterative filtering")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_AllUsersInsufficient_ReturnsFalse tests that when
// all users lack sufficient quota, the method returns false.
func TestCheckCIDGroupQuotaAvailability_AllUsersInsufficient_ReturnsFalse(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		lowLimit := int64(500)

		userID1 := fixture.dataManager.GenerateUserID()
		userID2 := fixture.dataManager.GenerateUserID()
		userID3 := fixture.dataManager.GenerateUserID()

		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &lowLimit,
			StorageLimit:     &lowLimit,
		})
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &lowLimit,
			StorageLimit:     &lowLimit,
		})
		fixture.dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &lowLimit,
			StorageLimit:     &lowLimit,
		})

		// Create pins for all users
		fixture.createPinsForUsers(t, []uint{userID1, userID2, userID3})

		// Test: 5000 bytes for 3 users = 1667 bytes each
		// All users only have 500 bytes → all filtered out → false

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 5000, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.False(t, available, "Should return false when all users have insufficient quota")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_NoUsersPinning_ReturnsFalse tests that when no users
// are pinning the content, the method returns false.
func TestCheckCIDGroupQuotaAvailability_NoUsersPinning_ReturnsFalse(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Create test upload that has no pins
		hash, err := multihash.Sum([]byte("no-pins-data"), multihash.SHA2_256, -1)
		require.NoError(t, err)
		testCID := cid.NewCidV1(cid.DagCBOR, hash)
		testHash := []byte(hash)

		dataManager := testdata.NewTestDataManager(ctx)
		quotaService := core.GetService[pluginCore.QuotaService](ctx, pluginCore.QUOTA_SERVICE)
		require.NotNil(t, quotaService)

		upload := &coreModels.Upload{
			UserID:     dataManager.GenerateUserID(),
			Hash:       testHash,
			CIDType:    uint64(testCID.Type()),
			Size:       uint64(100 * units.KB),
			MimeType:   "application/octet-stream",
			Protocol:   "ipfs",
			UploaderIP: "127.0.0.1",
			Metadata:   datatypes.JSON("{}"),
		}

		err = ctx.DB().Create(upload).Error
		require.NoError(t, err)

		// Test checking quota for upload with no pinners
		available, err := quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(testHash, 0, nil), 1000, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.False(t, available, "Should return false when no users are pinning the content")

		dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_SingleUserPinning tests the edge case
// where only a single user is pinning content and gets the full share.
func TestCheckCIDGroupQuotaAvailability_SingleUserPinning(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create test user
		userID := fixture.dataManager.GenerateUserID()
		uploadLimit := int64(1 * units.MB)
		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &uploadLimit,
			StorageLimit:     &uploadLimit,
		})

		// Create pin for single user
		fixture.createPinForUser(t, userID)

		// Test: 50 KB for 1 user = 50 KB (100% due to shared usage precision)
		// User has 1 * units.MB → should be allowed

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), uint64(50*units.KB), pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true when single user has sufficient quota")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_TwoUsersMixedQuota tests when users have
// different quota levels - one should be filtered out.
func TestCheckCIDGroupQuotaAvailability_TwoUsersMixedQuota(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: High quota user
		userID1 := fixture.dataManager.GenerateUserID()
		limit1 := int64(1 * units.MB)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit1,
			StorageLimit:     &limit1,
		})

		// User 2: Low quota user
		userID2 := fixture.dataManager.GenerateUserID()
		limit2 := int64(100)
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit2,
			StorageLimit:     &limit2,
		})

		// Create pins for both users
		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		// Test: 500 KB for 2 users = 250 KB each
		// - User 1 (1 MB): ✓
		// - User 2 (100 bytes): ✗
		//
		// Iteration 2: 500 KB for 1 user = 500 KB (100% precision)
		// - User 1 (1 MB): ✓
		//
		// Result: User 1 can handle all → true

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), uint64(500*units.KB), pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true when at least one user can handle the load")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_UsageTypes tests checking for different usage types
// (upload, download, storage) to ensure the correct path is taken.
func TestCheckCIDGroupQuotaAvailability_UsageTypes(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create users with limits for all types
		userID := fixture.dataManager.GenerateUserID()
		uploadLimit := int64(10 * units.MB)
		downloadLimit := int64(10 * units.MB)
		storageLimit := int64(10 * units.MB)

		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit:   &uploadLimit,
			DownloadDailyLimit: &downloadLimit,
			StorageLimit:       &storageLimit,
		})

		// Create pin
		fixture.createPinForUser(t, userID)

		// Test upload type
		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 1000, pluginCore.UsageTypeUpload)
		require.NoError(t, err)
		assert.True(t, available, "Should return true for upload type")

		// Test download type
		available, err = fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 1000, pluginCore.UsageTypeDownload)
		require.NoError(t, err)
		assert.True(t, available, "Should return true for download type")

		// Test storage type
		available, err = fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 1000, pluginCore.UsageTypeStorageAdd)
		require.NoError(t, err)
		assert.True(t, available, "Should return true for storage type")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_UnlimitedPolicy tests that unlimited policy
// users always pass quota checks.
func TestCheckCIDGroupQuotaAvailability_UnlimitedPolicy(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// User 1: Hard limits with sufficient quota
		userID1 := fixture.dataManager.GenerateUserID()
		limit1 := int64(10 * units.KB)
		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit1,
			StorageLimit:     &limit1,
		})

		// User 2: Unlimited policy
		userID2 := fixture.dataManager.GenerateUserID()
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyUnlimited, nil)

		// Create pins for both users
		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		// Test: 5 MB for 2 users = 2.5 MB each
		// User 1 (10 KB): ✗ would fail without unlimited user
		// User 2 (unlimited): ✓
		// Result: At least user 2 can handle → true

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), uint64(5*units.MB), pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true because unlimited user can handle any quota")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAccuracy tests edge cases with precision
func TestCheckCIDGroupQuotaAccuracy(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create users with low limits
		userID1 := fixture.dataManager.GenerateUserID()
		userID2 := fixture.dataManager.GenerateUserID()
		limit := int64(2 * units.KB)

		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit,
			StorageLimit:     &limit,
		})
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit,
			StorageLimit:     &limit,
		})

		// Create pins for both users
		fixture.createPinsForUsers(t, []uint{userID1, userID2})

		// Test: 3 KB for 2 users = 1500 bytes each = ceil(1500) = 1500 bytes each
		// With precision=2: 3 KB / 2 = 1.5 KB each

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 3000, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		// Each user gets 1500 bytes with precision=2, which is under 2048 limit
		assert.True(t, available, "Should return true when users have enough quota after precision calculation")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_QuotaDestruction tests when removing users
// leads to quota insufficiency for remaining users.
func TestCheckCIDGroupQuotaAvailability_QuotaDestruction(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create users with moderate quota
		userID1 := fixture.dataManager.GenerateUserID()
		userID2 := fixture.dataManager.GenerateUserID()
		userID3 := fixture.dataManager.GenerateUserID()

		limit := int64(5 * units.KB)

		fixture.dataManager.CreateUser(userID1, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit,
			StorageLimit:     &limit,
		})
		fixture.dataManager.CreateUser(userID2, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit,
			StorageLimit:     &limit,
		})
		fixture.dataManager.CreateUser(userID3, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit,
			StorageLimit:     &limit,
		})

		// Create pins for all users
		fixture.createPinsForUsers(t, []uint{userID1, userID2, userID3})

		// Test: 12 KB for 3 users = 4 KB each
		// All users have 5 KB → all pass → true

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 12000, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true when users share the load evenly")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// TestCheckCIDGroupQuotaAvailability_QuotaBoundary tests exact boundary conditions
func TestCheckCIDGroupQuotaAvailability_QuotaBoundary(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		fixture := newQuotaIntegrationTestFixture(t, ctx)

		// Create user
		userID := fixture.dataManager.GenerateUserID()
		limit := int64(1024 * 1) // Exactly 1 KB

		fixture.dataManager.CreateUser(userID, pluginModels.EnforcementPolicyHardLimits, &testdata.TestUserLimits{
			UploadDailyLimit: &limit,
			StorageLimit:     &limit,
		})

		// Create pin
		fixture.createPinForUser(t, userID)

		// Test: Exactly 1024 bytes for 1 user
		// User has exactly 1024 bytes → should pass

		available, err := fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 1024, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.True(t, available, "Should return true at exact quota boundary")

		// Test: 1025 bytes for 1 user (just over limit)
		available, err = fixture.quotaService.CheckCIDGroupQuotaAvailability(
			ctx, core.NewStorageHashFromMultihashBytes(fixture.testHash, 0, nil), 1025, pluginCore.UsageTypeUpload)

		require.NoError(t, err)
		assert.False(t, available, "Should return false when exceeding quota boundary by 1 byte")

		fixture.dataManager.CleanupWithContext(ctx)
	}, testOptionsForIntegration())
}

// testOptionsForIntegration creates test options for integration tests
// with full E2E service setup and plugin registration.
func testOptionsForIntegration() coreTesting.TestContextBuilderOption {
	return coreTesting.CombineOptions(
		serviceTesting.PresetE2E(),
		coreTesting.WithConfig("core.mail.host", "127.0.0.1"),
		coreTesting.WithConfig("core.mail.port", 1025),
		coreTesting.WithConfig("core.mail.ssl", false),
		coreTesting.WithConfig("core.mail.auth_type", "none"),
		coreTesting.WithConfig("core.mail.username", "test"),
		coreTesting.WithConfig("core.mail.password", "test"),
		coreTesting.WithConfig("core.mail.from", "test@localhost"),
		coreTesting.WithPlugins(
			corePlugin.GetPluginInfo(),
			plugin.GetPluginInfo(),
			dashboard.GetPluginInfo(),
		),
	)

}
