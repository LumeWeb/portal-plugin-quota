package policies

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// TestAllowancePolicyEnforcer_GetUsageHistory tests the GetUsageHistory method
func TestAllowancePolicyEnforcer_GetUsageHistory(t *testing.T) {
	coreTesting.RunTestCaseWithDB(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		userID := uint(1)
		createTestUser(t, ctx, userID, models.EnforcementPolicyAllowance, &testUserLimits{})

		// Create usage records with different timestamps
		now := time.Now()
		createTestUsageRecord(t, ctx, userID, models.UsageTypeUpload, 100)

		// Create a record in the past
		oldDetail := &models.UserUsageDetail{
			UserID:    userID,
			UploadID:  2,
			Type:      models.UsageTypeUpload,
			Bytes:     200,
			IP:        "192.168.1.1",
			Timestamp: now.Add(-48 * time.Hour), // 2 days ago
		}
		err := ctx.DB().Create(oldDetail).Error
		require.NoError(t, err)

		mockGrantManager := createMockGrantManager(t)
		enforcer := NewAllowancePolicyEnforcer(ctx, mockGrantManager)

		t.Run("Get recent usage history", func(t *testing.T) {
			history, err := enforcer.GetUsageHistory(userID, 1, pluginCore.UsageType(models.UsageTypeUpload))
			require.NoError(t, err)
			assert.Len(t, history, 1) // Only the recent record
			assert.Equal(t, uint64(100), history[0].Bytes)
		})

		t.Run("Get all usage history", func(t *testing.T) {
			history, err := enforcer.GetUsageHistory(userID, 3, pluginCore.UsageType(models.UsageTypeUpload))
			require.NoError(t, err)
			assert.Len(t, history, 2)                      // Both records
			assert.Equal(t, uint64(200), history[0].Bytes) // Older record first
			assert.Equal(t, uint64(100), history[1].Bytes) // Newer record second
		})
	}, testOptions())
}
