package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	quotaCore "go.lumeweb.com/portal-plugin-quota/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
	"go.lumeweb.com/queryutil"
	quotaModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	quotaDTO "go.lumeweb.com/portal-plugin-quota/internal/api/dto"
	"gorm.io/gorm"
)

func TestQuotaAdminExtension_TargetAPI(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		extFactory := NewQuotaAdminExtension()
		ext, _, err := extFactory()
		assert.NoError(t, err)

		adminExt := ext.(*QuotaAdminExtension)

		// Assert
		assert.Equal(t, "admin", adminExt.TargetAPI())
	}, AdminTestOptions)
}

// Tests for handleListPlans
func TestQuotaAdminExtension_ListPlans_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		mockPlans := []*quotaModels.QuotaPlan{
			createMockQuotaPlan(1),
			createMockQuotaPlan(2),
		}
		
		quotaService.EXPECT().
			ListQuotaPlans(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(mockPlans, int64(2), nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/plans", nil)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response queryutil.Response[[]quotaDTO.QuotaPlanResponse]
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), response.Total)
		assert.Len(t, response.Data, 2)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_ListPlans_Empty(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		quotaService.EXPECT().
			ListQuotaPlans(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]*quotaModels.QuotaPlan{}, int64(0), nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/plans", nil)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response queryutil.Response[[]quotaDTO.QuotaPlanResponse]
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), response.Total)
		assert.Empty(t, response.Data)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_ListPlans_WithError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		quotaService.EXPECT().
			ListQuotaPlans(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return([]*quotaModels.QuotaPlan{}, int64(0), assert.AnError)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/plans", nil)

		// Assert
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleCreatePlan
func TestQuotaAdminExtension_CreatePlan_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		_ = createMockQuotaPlan(1)
		
		quotaService.EXPECT().
			CreateQuotaPlan(mock.Anything, mock.AnythingOfType("*models.QuotaPlan")).
			Return(nil).
			Run(func(_ context.Context, newPlan *quotaModels.QuotaPlan) {
				newPlan.ID = 1
				newPlan.CreatedAt = time.Now()
				newPlan.UpdatedAt = time.Now()
			})

		body := map[string]interface{}{
			"name":                 "Test Plan",
			"storage_limit":        10737418240,
			"upload_daily_limit":   104857600,
			"download_daily_limit": 524288000,
			"is_active":            true,
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPost, "/api/quota/plans", jsonBody)

		// Assert
		assert.Equal(t, http.StatusCreated, rec.Code)
		
		var response quotaDTO.QuotaPlanResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), response.ID)
		assert.Equal(t, "Test Plan", response.Name)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_CreatePlan_ValidationError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		// Invalid request - missing required name field
		body := map[string]interface{}{
			"storage_limit": 10737418240,
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPost, "/api/quota/plans", jsonBody)

		// Assert
		assert.NotEqual(t, http.StatusCreated, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleGetPlan
func TestQuotaAdminExtension_GetPlan_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		plan := createMockQuotaPlan(1)
		plan.CreatedAt = time.Now()
		plan.UpdatedAt = time.Now()
		
		plan.ID = 1
		quotaService.EXPECT().
			GetQuotaPlan(mock.Anything, uint(1)).
			Return(plan, nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/plans/1", nil)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response quotaDTO.QuotaPlanResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), response.ID)
		assert.Equal(t, "Test Basic Plan", response.Name)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_GetPlan_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		quotaService.EXPECT().
			GetQuotaPlan(mock.Anything, uint(999)).
			Return(nil, gorm.ErrRecordNotFound)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/plans/999", nil)

		// Assert
		assert.Equal(t, http.StatusNotFound, rec.Code)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_GetPlan_InvalidID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/plans/invalid", nil)

		// Assert
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleUpdatePlan
func TestQuotaAdminExtension_UpdatePlan_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		existingPlan := createMockQuotaPlan(1)
		existingPlan.CreatedAt = time.Now()
		existingPlan.UpdatedAt = time.Now()
		
		quotaService.EXPECT().
			GetQuotaPlan(mock.Anything, uint(1)).
			Return(existingPlan, nil)
		
		quotaService.EXPECT().
			UpdateQuotaPlan(mock.Anything, uint(1), mock.AnythingOfType("*models.QuotaPlan")).
			Return(nil).
			Run(func(_ context.Context, _ uint, updatedPlan *quotaModels.QuotaPlan) {
				updatedPlan.ID = 1
				updatedPlan.UpdatedAt = time.Now()
			})

		body := map[string]interface{}{
			"name":                 "Updated Plan Name",
			"storage_limit":        21474836480,
			"is_active":            true,
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPut, "/api/quota/plans/1", jsonBody)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response quotaDTO.QuotaPlanResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), response.ID)
		assert.Equal(t, "Updated Plan Name", response.Name)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_UpdatePlan_NotFound(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		quotaService.EXPECT().
			GetQuotaPlan(mock.Anything, uint(999)).
			Return(nil, gorm.ErrRecordNotFound)
		
		body := map[string]interface{}{
			"name": "Updated Name",
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPut, "/api/quota/plans/999", jsonBody)

		// Assert
		assert.Equal(t, http.StatusNotFound, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleCreatePlan for UpdatePlan scenario would go here...

// Tests for handleDeletePlan
func TestQuotaAdminExtension_DeletePlan_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		quotaService.EXPECT().
			DeleteQuotaPlan(mock.Anything, uint(1)).
			Return(nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodDelete, "/api/quota/plans/1", nil)

		// Assert
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_DeletePlan_InvalidID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)

		// Act
		rec := helper.ExecuteRequest(http.MethodDelete, "/api/quota/plans/invalid", nil)

		// Assert
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleSetDefaultPlan
func TestQuotaAdminExtension_SetDefaultPlan_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		quotaService := helper.GetQuotaService()
		
		quotaService.EXPECT().
			SetDefaultQuotaPlan(mock.Anything, uint(1)).
			Return(nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodPost, "/api/quota/plans/1/default", nil)

		// Assert
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleListAllowances
func TestQuotaAdminExtension_ListAllowances_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		mockGrants := []*quotaModels.AllowanceGrant{
			createMockAllowanceGrant(1, 1),
			createMockAllowanceGrant(2, 2),
		}
		
		grantManager := helper.SetupGrantManagerMock()
		
		grantManager.EXPECT().
			ListGrants(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(mockGrants, int64(2), nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/allowances", nil)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response queryutil.Response[[]quotaDTO.AllowanceGrantResponse]
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), response.Total)
		assert.Len(t, response.Data, 2)
	}, AdminTestOptions)
}

// Tests for handleCreateGrant
func TestQuotaAdminExtension_CreateGrant_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		grant := createMockAllowanceGrant(1, 1)
		grant.CreatedAt = time.Now()
		grant.UpdatedAt = time.Now()
		
		grantManager := helper.SetupGrantManagerMock()
		
		grantManager.EXPECT().
			CreateAllowanceGrant(mock.Anything, uint(1), mock.AnythingOfType("*models.AllowanceGrant")).
			Return(nil).
			Run(func(_ context.Context, _ uint, newGrant *quotaModels.AllowanceGrant) {
				newGrant.ID = 1
				newGrant.CreatedAt = time.Now()
				newGrant.UpdatedAt = time.Now()
			})

		body := map[string]interface{}{
			"user_id":  1,
			"type":     "STORAGE",
			"source":   "SUBSCRIPTION",
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPost, "/api/quota/allowances", jsonBody)

		// Assert
		assert.Equal(t, http.StatusCreated, rec.Code)
		
		var response quotaDTO.AllowanceGrantResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), response.ID)
		assert.Equal(t, uint(1), response.UserID)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_CreateGrant_ValidationError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		// Invalid request - missing required user_id and type
		body := map[string]interface{}{
			"storage": 10737418240,
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPost, "/api/quota/allowances", jsonBody)

		// Assert
		assert.NotEqual(t, http.StatusCreated, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleDeleteGrant
func TestQuotaAdminExtension_DeleteGrant_Success(t *testing.T) {
		coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		mockGrantManager := helper.SetupGrantManagerMock()
		mockGrantManager.EXPECT().DeactivateGrant(mock.Anything, uint(1)).Return(nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodDelete, "/api/quota/allowances/1", nil)

		// Assert
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_DeleteGrant_InvalidID(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)

		// Act
		rec := helper.ExecuteRequest(http.MethodDelete, "/api/quota/allowances/invalid", nil)

		// Assert
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	}, AdminTestOptions)
}

// Tests for handleSystemStats
func TestQuotaAdminExtension_SystemStats_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)

		expectedStats := &quotaCore.SystemStats{
			TotalUsers:   100,
			ActiveUsers:  75,
			TotalPlans:   10,
			ActivePlans:  8,
			TotalGrants:  50,
			ActiveGrants: 45,
			CurrentUsage: quotaCore.Usage{
				UserID:          0,
				BytesUploaded:   1073741824,  // 1GB
				BytesDownloaded: 536870912,   // 512MB
				BytesStored:     2147483648,  // 2GB
			},
			TotalUsageBytes: 3760699264, // Sum of all usage
		}

		helper.GetQuotaService().EXPECT().GetSystemStats(mock.Anything).Return(expectedStats, nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/system/stats", nil)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)

		var response quotaDTO.SystemStatsResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, int64(100), response.TotalUsers)
		assert.Equal(t, int64(75), response.ActiveUsers)
		assert.Equal(t, int64(10), response.TotalPlans)
		assert.Equal(t, int64(8), response.ActivePlans)
		assert.Equal(t, int64(50), response.TotalGrants)
		assert.Equal(t, int64(45), response.ActiveGrants)
		assert.Equal(t, uint64(2147483648), response.CurrentUsage.StorageBytes)
		assert.Equal(t, uint64(1073741824), response.CurrentUsage.UploadBytes)
		assert.Equal(t, uint64(536870912), response.CurrentUsage.DownloadBytes)
		assert.Equal(t, uint64(3760699264), response.TotalUsageBytes)
	}, AdminTestOptions)
}

// Tests for handleGetSystemConfig
func TestQuotaAdminExtension_GetSystemConfig_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		defaultPlan := &quotaModels.QuotaPlan{
			Model:  gorm.Model{ID: 1},
			Name:   "Test Plan",
		}
		helper.GetQuotaService().EXPECT().GetDefaultQuotaPlan(mock.Anything).Return(defaultPlan, nil)

		// Act
		rec := helper.ExecuteRequest(http.MethodGet, "/api/quota/system/config", nil)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
		
		var response quotaDTO.QuotaConfigResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, uint(1), *response.DefaultPlanID)
		assert.Equal(t, "Test Plan", response.DefaultPlanName)
	}, AdminTestOptions)
}

// Tests for handleUpdateSystemConfig
func TestQuotaAdminExtension_UpdateSystemConfig_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		helper.GetQuotaService().EXPECT().SetDefaultQuotaPlan(mock.Anything, uint(1)).Return(nil)
		
		body := map[string]interface{}{
			"default_plan_id":             1,
			"enable_quota_enforcement":    true,
			"storage_retention_days":      90,
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPut, "/api/quota/system/config", jsonBody)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)
	}, AdminTestOptions)
}

func TestQuotaAdminExtension_UpdateSystemConfig_ValidationError(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		// Arrange
		helper := NewQuotaTestHelper(t, ctx)
		
		body := map[string]interface{}{
			"default_plan_id":             1,
			"storage_retention_days":      -1, // Invalid - must be positive
		}
		jsonBody, _ := json.Marshal(body)

		// Act
		rec := helper.ExecuteRequest(http.MethodPut, "/api/quota/system/config", jsonBody)

		// Assert
		assert.NotEqual(t, http.StatusOK, rec.Code)
	}, AdminTestOptions)
}
