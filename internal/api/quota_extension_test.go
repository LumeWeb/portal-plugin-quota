// Package api provides tests for the Quota API extension endpoints
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	coreTesting "go.lumeweb.com/portal/core/testing"
	quotaCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/api/dto"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"github.com/docker/go-units"
)

func TestHandleQuotaStatus_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()

		// Setup authentication with a default user
		helper.SetupAuth()

		// Mock config manager to support ALLOWANCE policy (default behavior)
		mockConfigManager := quotaCore.NewMockConfigManager(t)
		quotaSvc.EXPECT().GetConfigManager().Return(mockConfigManager)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, uint(1)).
			Return(&quotaCore.UserQuotaConfig{
				EnforcementPolicy: models.EnforcementPolicyAllowance,
			}, nil).Once()

		quotaSvc.EXPECT().GetAllowanceBalance(mock.Anything, uint(1)).
			Return(&quotaCore.AllowanceBalance{
				UploadUsed:       uint64(units.GiB),
				UploadAllowance:   uint64(units.GiB * 10),
				UploadRemaining:   uint64(units.GiB * 9),
				DownloadUsed:      uint64(units.GiB * 2),
				DownloadAllowance: uint64(units.GiB * 20),
				DownloadRemaining: uint64(units.GiB * 18),
			}, nil).Once()

		// Execute request using helper which properly handles auth
		rec := helper.ExecuteRequest(http.MethodGet, "/api/account/quota", nil)

		// Verify
		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.QuotaStatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		
		require.NotNil(t, response.Upload, "Upload section should not be nil")
		require.NotNil(t, response.Upload.Limit, "Upload.Limit should not be nil")

		assert.Equal(t, uint64(units.GiB), response.Upload.Used)
		assert.Equal(t, uint64(units.GiB*10), *response.Upload.Limit)
		assert.Equal(t, uint64(units.GiB*9), *response.Upload.Remaining)
		assert.Equal(t, 10, *response.Upload.Percentage)

		assert.Equal(t, uint64(units.GiB*2), response.Download.Used)
		assert.Equal(t, uint64(units.GiB*20), *response.Download.Limit)
		assert.Equal(t, uint64(units.GiB*18), *response.Download.Remaining)
		assert.Equal(t, 10, *response.Download.Percentage)
	}, QuotaTestOptions)
}

func TestHandleQuotaStatus_Unauthorized(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)

		rec := httptest.NewRecorder()
		req := helper.ctx.NewAPIRequest(http.MethodGet, "/api/account/quota", nil)
		// Remove the Authorization header to simulate unauthorized request
		req.Header.Del("Authorization")
		helper.ctx.Router().ServeHTTP(rec, req)

		// Verify
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	}, QuotaTestOptions)
}

func TestHandleQuotaHistory_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()
		
		// Setup authentication with a default user
		helper.SetupAuth()

		startDate := "2024-01-01T00:00:00Z"
		endDate := "2024-01-31T23:59:59Z"
		usageType := quotaCore.UsageTypeDownload
		startTime := parseTime(t, startDate)
		endTime := parseTime(t, endDate)
		quotaSvc.EXPECT().GetUsageHistoryDateRange(mock.Anything, uint(1), usageType, startTime, endTime).
			Return([]*quotaCore.UsagePoint{
				{
					Date:  parseTime(t, "2024-01-15T12:00:00Z"),
					Bytes: uint64(units.MiB * 100),
				},
				{
					Date:  parseTime(t, "2024-01-16T12:00:00Z"),
					Bytes: uint64(units.MiB * 150),
				},
			}, nil).Once()

		// Execute request with query params using helper
		rec := helper.ExecuteRequest(http.MethodGet, buildQuotaHistoryURL(startDate, endDate, "download"), nil)

		// Verify
		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.QuotaHistoryResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, uint(1), response.UserID)
		assert.Equal(t, 2, len(response.Points))
		assert.Equal(t, "2024-01-15T12:00:00Z", response.Points[0].Date)
		assert.Equal(t, uint64(units.MiB*100), response.Points[0].Bytes)
		assert.Equal(t, "2024-01-16T12:00:00Z", response.Points[1].Date)
		assert.Equal(t, uint64(units.MiB*150), response.Points[1].Bytes)
	}, QuotaTestOptions)
}

func TestHandleQuotaHistory_MissingRequiredParams(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)

		helper.SetupAuth()

		// Missing start_date and end_date
		rec := helper.ExecuteRequest(http.MethodGet, buildQuotaHistoryURL("", "", "download"), nil)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	}, QuotaTestOptions)
}

func TestHandleQuotaHistory_InvalidDateFormat(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)

		helper.SetupAuth()

		// Invalid date format
		rec := helper.ExecuteRequest(http.MethodGet, buildQuotaHistoryURL("invalid", "2024-01-31T23:59:59Z", "download"), nil)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	}, QuotaTestOptions)
}

// TestHandleQuotaStatus_UnlimitedPolicy tests the UNLIMITED enforcement policy
func TestHandleQuotaStatus_UnlimitedPolicy(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()

		helper.SetupAuth()

		mockConfigManager := quotaCore.NewMockConfigManager(t)
		quotaSvc.EXPECT().GetConfigManager().Return(mockConfigManager)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, uint(1)).
			Return(&quotaCore.UserQuotaConfig{
				EnforcementPolicy: models.EnforcementPolicyUnlimited,
			}, nil).Once()

		mockUsageManager := quotaCore.NewMockUsageManager(t)
		quotaSvc.EXPECT().GetUsageManager().Return(mockUsageManager)
		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, uint(1)).
			Return(&quotaCore.Usage{
				BytesUploaded:   uint64(units.GiB * 5),
				BytesDownloaded: uint64(units.GiB * 10),
			}, nil).Once()

		rec := helper.ExecuteRequest(http.MethodGet, "/api/account/quota", nil)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.QuotaStatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, uint64(units.GiB*5), response.Upload.Used)
		assert.Equal(t, uint64(units.GiB*10), response.Download.Used)
		assert.Nil(t, response.Upload.Limit)
		assert.Nil(t, response.Download.Limit)
		assert.Nil(t, response.Upload.Remaining)
		assert.Nil(t, response.Download.Remaining)
	}, QuotaTestOptions)
}

// TestHandleQuotaStatus_HardLimitsPolicy tests the HARD_LIMITS enforcement policy
func TestHandleQuotaStatus_HardLimitsPolicy(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()

		helper.SetupAuth()

		mockConfigManager := quotaCore.NewMockConfigManager(t)
		quotaSvc.EXPECT().GetConfigManager().Return(mockConfigManager)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, uint(1)).
			Return(&quotaCore.UserQuotaConfig{
				EnforcementPolicy: models.EnforcementPolicyHardLimits,
			}, nil).Once()

		uploadLimit := uint64(units.MiB * 100)
		downloadLimit := uint64(units.MiB * 500)
		mockConfigManager.EXPECT().ResolveEffectiveLimits(mock.Anything, uint(1)).
			Return(&quotaCore.EffectiveLimits{
				UploadDailyLimit:   &uploadLimit,
				DownloadDailyLimit: &downloadLimit,
				HasUploadDailyLimitConfig:   true,
				HasDownloadDailyLimitConfig: true,
			}, nil).Once()

		mockUsageManager := quotaCore.NewMockUsageManager(t)
		quotaSvc.EXPECT().GetUsageManager().Return(mockUsageManager)
		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, uint(1)).
			Return(&quotaCore.Usage{
				BytesUploaded:   uint64(units.MiB * 30),
				BytesDownloaded: uint64(units.MiB * 150),
			}, nil).Once()

		rec := helper.ExecuteRequest(http.MethodGet, "/api/account/quota", nil)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.QuotaStatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, uint64(units.MiB*30), response.Upload.Used)
		assert.Equal(t, uint64(units.MiB*100), *response.Upload.Limit)
		assert.Equal(t, uint64(units.MiB*70), *response.Upload.Remaining)
		assert.Equal(t, 30, *response.Upload.Percentage)
	}, QuotaTestOptions)
}

// TestHandleQuotaStatus_ThresholdPolicy tests the THRESHOLD enforcement policy
func TestHandleQuotaStatus_ThresholdPolicy(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()

		helper.SetupAuth()

		mockConfigManager := quotaCore.NewMockConfigManager(t)
		quotaSvc.EXPECT().GetConfigManager().Return(mockConfigManager)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, uint(1)).
			Return(&quotaCore.UserQuotaConfig{
				EnforcementPolicy: models.EnforcementPolicyThreshold,
			}, nil).Once()

		uploadLimit := uint64(units.MiB * 100)
		downloadLimit := uint64(units.MiB * 500)
		uploadThreshold := uint64(80)
		downloadThreshold := uint64(90)
		mockConfigManager.EXPECT().ResolveEffectiveLimits(mock.Anything, uint(1)).
			Return(&quotaCore.EffectiveLimits{
				UploadDailyLimit:   &uploadLimit,
				DownloadDailyLimit: &downloadLimit,
				UploadThreshold:    &uploadThreshold,
				DownloadThreshold:  &downloadThreshold,
				HasUploadDailyLimitConfig:        true,
				HasDownloadDailyLimitConfig:      true,
				HasUploadThresholdConfig:         true,
				HasDownloadThresholdConfig:       true,
			}, nil).Once()

		mockUsageManager := quotaCore.NewMockUsageManager(t)
		quotaSvc.EXPECT().GetUsageManager().Return(mockUsageManager)
		mockUsageManager.EXPECT().GetCurrentUsage(mock.Anything, uint(1)).
			Return(&quotaCore.Usage{
				BytesUploaded:   uint64(units.MiB * 80),
				BytesDownloaded: uint64(units.MiB * 450),
			}, nil).Once()

		rec := helper.ExecuteRequest(http.MethodGet, "/api/account/quota", nil)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.QuotaStatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, uint64(units.MiB*80), response.Upload.Used)
		assert.Equal(t, uint64(units.MiB*100), *response.Upload.Limit)
		assert.Equal(t, uint64(units.MiB*20), *response.Upload.Remaining)
		assert.Equal(t, 80, *response.Upload.Percentage)
	}, QuotaTestOptions)
}

// TestHandleQuotaStatus_AllowancePolicy tests the ALLOWANCE enforcement policy
func TestHandleQuotaStatus_AllowancePolicy(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()

		helper.SetupAuth()

		mockConfigManager := quotaCore.NewMockConfigManager(t)
		quotaSvc.EXPECT().GetConfigManager().Return(mockConfigManager)
		mockConfigManager.EXPECT().GetUserQuotaConfig(mock.Anything, uint(1)).
			Return(&quotaCore.UserQuotaConfig{
				EnforcementPolicy: models.EnforcementPolicyAllowance,
			}, nil).Once()

		quotaSvc.EXPECT().GetAllowanceBalance(mock.Anything, uint(1)).
			Return(&quotaCore.AllowanceBalance{
				UploadUsed:        uint64(units.GiB * 5),
				UploadAllowance:   uint64(units.GiB * 10),
				UploadRemaining:    uint64(units.GiB * 5),
				DownloadUsed:      uint64(units.GiB * 12),
				DownloadAllowance: uint64(units.GiB * 20),
				DownloadRemaining:  uint64(units.GiB * 8),
			}, nil).Once()

		rec := helper.ExecuteRequest(http.MethodGet, "/api/account/quota", nil)

		assert.Equal(t, http.StatusOK, rec.Code)

		var response dto.QuotaStatusResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, uint64(units.GiB*5), response.Upload.Used)
		assert.Equal(t, uint64(units.GiB*10), *response.Upload.Limit)
		assert.Equal(t, uint64(units.GiB*5), *response.Upload.Remaining)
		assert.Equal(t, 50, *response.Upload.Percentage)
	}, QuotaTestOptions)
}

// test helpers

// parseTime is a helper to parse RFC3339 timestamps in tests
func parseTime(t *testing.T, timestamp string) time.Time {
	parsed, err := time.Parse(time.RFC3339, timestamp)
	require.NoError(t, err)
	return parsed
}

// buildQuotaHistoryURL constructs a URL for quota history endpoint with query parameters
func buildQuotaHistoryURL(startDate, endDate, quotaType string) string {
	values := url.Values{}
	if startDate != "" {
		values.Add("start_date", startDate)
	}
	if endDate != "" {
		values.Add("end_date", endDate)
	}
	if quotaType != "" {
		values.Add("type", quotaType)
	}
	return "/api/account/quota/history?" + values.Encode()
}
