package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	pluginTesting "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal-plugin-quota/internal/testing/testdata"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestModelValidation_EnforcementPolicy_ValidHardLimits(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyHardLimits,
		}

		err := ctx.DB().Create(config).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_EnforcementPolicy_ValidUnlimited(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyUnlimited,
		}

		err := ctx.DB().Create(config).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_EnforcementPolicy_ValidThreshold(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyThreshold,
		}

		err := ctx.DB().Create(config).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_EnforcementPolicy_ValidAllowance(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: models.EnforcementPolicyAllowance,
		}

		err := ctx.DB().Create(config).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_EnforcementPolicy_Invalid(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		config := &models.UserQuotaConfig{
			UserID:            userID,
			EnforcementPolicy: "INVALID_POLICY",
		}

		// Test that model validation prevents creating invalid configs
		err := ctx.DB().Create(config).Error
		// The model's BeforeCreate hook should reject invalid enforcement policies
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "enforcement policy is invalid")

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_UsageType_ValidUpload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		detail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      models.UsageTypeUpload,
			Bytes:     100,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		err := ctx.DB().Create(detail).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_UsageType_ValidDownload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		detail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      models.UsageTypeDownload,
			Bytes:     100,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		err := ctx.DB().Create(detail).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_UsageType_ValidStorageAdd(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		detail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      models.UsageTypeStorageAdd,
			Bytes:     100,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		err := ctx.DB().Create(detail).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_UsageType_ValidStorageRemove(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		detail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  dataManager.NextUploadID(),
			Type:      models.UsageTypeStorageRemove,
			Bytes:     100,
			IP:        "192.168.1.1",
			Timestamp: time.Now(),
		}

		err := ctx.DB().Create(detail).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantType_ValidStorage(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeStorage,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantType_ValidUpload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantType_ValidDownload(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeDownload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantSource_ValidSubscription(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourceSubscription,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantSource_ValidPAYGAddon(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourcePAYGAddon,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantSource_ValidBonus(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourceBonus,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_GrantSource_ValidPromo(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		dataManager := testdata.NewTestDataManager(ctx)
		userID := dataManager.NextUserID()
		grant := &models.AllowanceGrant{
			UserID:         userID,
			Type:           models.GrantTypeUpload,
			Source:         models.GrantSourcePromo,
			Bytes:          1000,
			BytesUsed:      0,
			BytesRemaining: 1000,
			IsActive:       true,
		}

		err := ctx.DB().Create(grant).Error
		assert.NoError(t, err)

		t.Cleanup(func() {
			dataManager.Cleanup()
		})
	}, pluginTesting.TestOptions())
}

func TestModelValidation_QuotaCheckReason_OK(t *testing.T) {
	reason := models.QuotaCheckReasonOK
	assert.NotEmpty(t, string(reason))
}

func TestModelValidation_QuotaCheckReason_LimitExceeded(t *testing.T) {
	reason := models.QuotaCheckReasonLimitExceeded
	assert.NotEmpty(t, string(reason))
}

func TestModelValidation_QuotaCheckReason_AllowanceDepleted(t *testing.T) {
	reason := models.QuotaCheckReasonAllowanceDepleted
	assert.NotEmpty(t, string(reason))
}

func TestModelValidation_QuotaCheckReason_WarningThreshold(t *testing.T) {
	reason := models.QuotaCheckReasonWarningThreshold
	assert.NotEmpty(t, string(reason))
}
