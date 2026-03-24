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
)

func TestHandleQuotaStatus_Success(t *testing.T) {
	coreTesting.RunTestCase(t, func(tb coreTesting.TB, ctx coreTesting.TestContext) {
		helper := NewQuotaTestHelper(t, ctx)
		quotaSvc := helper.GetQuotaService()
		
		// Setup authentication with a default user
		helper.SetupAuth()
		
		quotaSvc.EXPECT().GetAllowanceBalance(mock.Anything, uint(1)).
			Return(&quotaCore.AllowanceBalance{
				UploadUsed:       1000,
				UploadAllowance:   10000,
				UploadRemaining:   9000,
				DownloadUsed:      2000,
				DownloadAllowance: 20000,
				DownloadRemaining: 18000,
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

		assert.Equal(t, uint64(1000), response.Upload.Used)
		assert.Equal(t, uint64(10000), *response.Upload.Limit)
		assert.Equal(t, uint64(9000), *response.Upload.Remaining)
		assert.Equal(t, 10, *response.Upload.Percentage)

		assert.Equal(t, uint64(2000), response.Download.Used)
		assert.Equal(t, uint64(20000), *response.Download.Limit)
		assert.Equal(t, uint64(18000), *response.Download.Remaining)
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
		quotaSvc.EXPECT().GetUsageHistory(mock.Anything, uint(1), 0, usageType).
			Return([]*quotaCore.UsagePoint{
				{
					Date:  parseTime(t, "2024-01-15T12:00:00Z"),
					Bytes: 5000,
				},
				{
					Date:  parseTime(t, "2024-01-16T12:00:00Z"),
					Bytes: 7500,
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
		assert.Equal(t, uint64(5000), response.Points[0].Bytes)
		assert.Equal(t, "2024-01-16T12:00:00Z", response.Points[1].Date)
		assert.Equal(t, uint64(7500), response.Points[1].Bytes)
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
