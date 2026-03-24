package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	quotaCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal"
	quotaModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// AdminTestOptions provides test configuration for admin extension tests
var AdminTestOptions = coreTesting.CombineOptions(
	coreTesting.WithMockServiceFactory(quotaCore.QUOTA_SERVICE, quotaCore.NewMockQuotaService),
	coreTesting.WithAPIExtension(NewQuotaAdminExtension()),
	coreTesting.WithAPIID("admin"),
	// Setup QuotaService GetConfig mock after context creation
	setupQuotaServiceMocks,
)

// setupQuotaServiceMocks creates a TestContextBuilderOption that sets up
// common mock expectations for QuotaService
func setupQuotaServiceMocks(ctx coreTesting.TestContext) (coreTesting.TestContext, error) {
	mockQuotaService := core.GetService[*quotaCore.MockQuotaService](ctx, internal.PluginName)
	
	// Setup framework-required methods that will be called during context initialization
	// These MUST use Maybe() because they may be called multiple times
	mockQuotaService.EXPECT().GetConfig().Return(&config.QuotaConfig{}, nil).Maybe()
	
	return ctx, nil
}

// QuotaTestHelper provides common test utilities for quota admin API tests
type QuotaTestHelper struct {
	t   *testing.T
	tb  coreTesting.TB
	ctx coreTesting.TestContext
}

// NewQuotaTestHelper creates a new test helper for quota admin tests
func NewQuotaTestHelper(t *testing.T, ctx coreTesting.TestContext) *QuotaTestHelper {
	return &QuotaTestHelper{
		t:   t,
		tb:  t,
		ctx: ctx,
	}
}

// GetQuotaService returns the mocked QuotaService
func (h *QuotaTestHelper) GetQuotaService() *quotaCore.MockQuotaService {
	return core.GetService[*quotaCore.MockQuotaService](h.ctx, internal.PluginName)
}

// SetupGrantManagerMock sets up the GrantManager mock and returns it
func (h *QuotaTestHelper) SetupGrantManagerMock() *quotaCore.MockGrantManager {
	mockGrantManager := quotaCore.NewMockGrantManager(h.t)
	h.GetQuotaService().EXPECT().GetGrantManager().Return(mockGrantManager).Maybe()
	return mockGrantManager
}

// NewAuthorizedRequest creates a new authorized API request
func (h *QuotaTestHelper) NewAuthorizedRequest(method, url string, body []byte) *http.Request {
	return h.ctx.NewAPIRequest(method, url, body)
}

// ExecuteRequest executes an API request and records the response
func (h *QuotaTestHelper) ExecuteRequest(method, url string, body []byte) *httptest.ResponseRecorder {
	req := h.NewAuthorizedRequest(method, url, body)
	rec := httptest.NewRecorder()
	h.ctx.Router().ServeHTTP(rec, req)
	return rec
}

// createMockQuotaPlan creates a standardized mock quota plan
func createMockQuotaPlan(id uint) *quotaModels.QuotaPlan {
	return &quotaModels.QuotaPlan{
		
		Name:               "Test Basic Plan",
		Description:        "Basic quota plan for testing",
		StorageLimit:       10737418240, // 10GB
		UploadDailyLimit:   104857600,   // 100MB
		DownloadDailyLimit: 524288000,   // 500MB
		UploadTotalLimit:   1073741824,  // 1GB
		DownloadTotalLimit: 1073741824,  // 1GB
		StorageThreshold:   &[]int64{80}[0],
		UploadThreshold:    &[]int64{90}[0],
		DownloadThreshold:  &[]int64{90}[0],
		IsDefault:          false,
		IsActive:           &[]bool{true}[0],
	}
}

// createMockAllowanceGrant creates a standardized mock allowance grant
func createMockAllowanceGrant(id, userID uint) *quotaModels.AllowanceGrant {
	return &quotaModels.AllowanceGrant{
		UserID:         userID,
		Type:           quotaModels.GrantTypeStorage,
		Source:         quotaModels.GrantSourceSubscription,
		Bytes:         10737418240, // 10GB
		BytesUsed:     536870912,   // 512MB used
		BytesRemaining: 10200547328,  // ~9.5GB remaining
		IsActive:      true,
	}
}

