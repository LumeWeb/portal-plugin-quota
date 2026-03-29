package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	quotaCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal"
	quotaModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	quota_testing "go.lumeweb.com/portal-plugin-quota/internal/testing"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

// baseTestOptions provides common test configuration for all quota extension tests
var baseTestOptions = coreTesting.CombineOptions(
	// Use the base test options from internal/testing which includes service factory and mocks
	quota_testing.TestOptions(),
)

// AdminTestOptions provides test configuration for admin extension tests
var AdminTestOptions = coreTesting.CombineOptions(
	baseTestOptions,
	coreTesting.WithAPIExtension(NewQuotaAdminExtension()),
	coreTesting.WithAPIID("admin"),
)

// QuotaTestOptions provides test configuration for user quota extension tests
var QuotaTestOptions = coreTesting.CombineOptions(
	baseTestOptions,
	coreTesting.WithAPIExtension(NewQuotaExtension()),
	coreTesting.WithAPIID("dashboard"),
	coreTesting.WithConfig("plugin.dashboard.api.subdomain", "account"),
)


// QuotaTestHelper provides common test utilities for quota admin API tests
type QuotaTestHelper struct {
	t         *testing.T
	tb        coreTesting.TB
	ctx       coreTesting.TestContext
	authToken string
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
	req := h.ctx.NewAPIRequest(method, url, body)
	if h.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+h.authToken)
	}
	return req
}

// SetupAuth sets up authentication with a default user for testing
func (h *QuotaTestHelper) SetupAuth() {
	mockAuthService := core.GetService[*coreTesting.MockAuthService](h.ctx, core.AUTH_SERVICE)
	userService := mockAuthService.GetUserService()
	userService.UserExists(1)
	jwtHelper := coreTesting.NewJWTHelper(h.ctx)
	token, err := jwtHelper.CreateLoginToken(1)
	require.NoError(h.t, err)
	h.authToken = token
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
	storageLimit := uint64(10737418240) // 10GB
	uploadLimit := uint64(104857600)    // 100MB
	downloadLimit := uint64(524288000)  // 500MB
	windowType := quotaModels.WindowTypeRolling
	windowDuration := int64(86400) // 1 day in seconds
	startHour := 0
	timezone := "UTC"
	
	return &quotaModels.QuotaPlan{
		Name:              "Test Basic Plan",
		Description:       "Basic quota plan for testing",
		WindowType:        windowType,
		WindowDuration:    &windowDuration,
		WindowStartHour:   &startHour,
		WindowTimezone:    &timezone,
		StorageLimitBytes: storageLimit,
		UploadLimitBytes:  uploadLimit,
		DownloadLimitBytes: downloadLimit,
		StorageThreshold:  &[]int64{8589934592}[0],  // 8GB
		UploadThreshold:   &[]int64{94371840}[0],    // 90MB
		DownloadThreshold: &[]int64{471859200}[0],   // 450MB
		IsDefault:         false,
		IsActive:          &[]bool{true}[0],
	}
}

// createMockAllowanceGrant creates a standardized mock allowance grant
func createMockAllowanceGrant(id, userID uint) *quotaModels.AllowanceGrant {
	return &quotaModels.AllowanceGrant{
		UserID:         userID,
		Type:           quotaModels.GrantTypeStorage,
		Source:         quotaModels.GrantSourceSubscription,
		Bytes:          10737418240, // 10GB
		BytesUsed:      536870912,   // 512MB used
		BytesRemaining: 10200547328, // ~9.5GB remaining
		IsActive:       true,
	}
}
