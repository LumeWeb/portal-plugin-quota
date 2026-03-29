package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	mcontext "go.lumeweb.com/portal-middleware/context"
	"go.lumeweb.com/portal-middleware/middleware"
	"go.lumeweb.com/portal/core"
	quotaCore "go.lumeweb.com/portal-plugin-quota/core"
	router "go.lumeweb.com/portal-router"
	"go.lumeweb.com/portal-plugin-quota/internal/api/dto"
	models "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.uber.org/zap"
)

type QuotaExtension struct {
	*core.BaseComponent
	quotaService quotaCore.QuotaService
}

func NewQuotaExtension() core.APIExtensionFactory {
	return func() (core.APIExtension, []core.ContextBuilderOption, error) {
		ext := &QuotaExtension{}

		return ext, core.ContextOptions(
			core.ContextWithStartupFunc(func(ctx core.Context) error {
				ext.quotaService = core.GetService[quotaCore.QuotaService](ctx, internal.PluginName)
				return nil
			}),
		), nil
	}
}

// TargetAPI returns the API this extension extends
func (e *QuotaExtension) TargetAPI() string {
	return "dashboard"
}

// Name returns the extension name
func (e *QuotaExtension) Name() string {
	return internal.ProtocolName
}

// ID returns the extension ID
func (e *QuotaExtension) ID() string {
	return e.Name()
}



// Configure adds routes to the Dashboard API
func (e *QuotaExtension) Configure(gRouter router.Router, accessSvc core.AccessService) error {
	middlewares := e.buildMiddlewares()

	routes := []router.Route{
		router.NewRoute(http.MethodGet, "/api/account/quota", e.handleQuotaStatus,
			router.WithAccess(core.ACCESS_USER_ROLE),
			router.WithMiddlewares(middlewares...),
			router.WithCors(),
			router.WithSwaggerOptions(
				router.WithSummary("Get current quota status"),
				router.WithDescription("Retrieve the current quota status including upload and download usage, limits, and remaining allowance for the authenticated user."),
				router.WithTags("quota", "status"),
				router.WithSuccessResponse(http.StatusOK, "Quota status", router.WithJSONContent(&dto.QuotaStatusResponse{})),
			),
		),
		router.NewRoute(http.MethodGet, "/api/account/quota/history", e.handleQuotaHistory,
			router.WithAccess(core.ACCESS_USER_ROLE),
			router.WithMiddlewares(middlewares...),
			router.WithCors(),
			router.WithSwaggerOptions(
				router.WithSummary("Get quota usage history"),
				router.WithDescription("Retrieves historical quota usage data for charting and analytics."),
				router.WithQueryParam("start_date", "Start date in RFC3339 format", "2024-01-01T00:00:00Z"),
				router.WithQueryParam("end_date", "End date in RFC3339 format", "2024-01-31T23:59:59Z"),
				router.WithQueryParam("type", "Usage type (upload or download)", "upload"),
				router.WithSuccessResponse(http.StatusOK, "Quota usage history", router.WithJSONContent(&dto.QuotaHistoryResponse{})),
			),
		),
	}

	return router.RegisterRoutes(gRouter, accessSvc, core.GetAPI(e.TargetAPI()).Subdomain(), routes)
}

// buildMiddlewares builds common middleware for quota extension routes
func (e *QuotaExtension) buildMiddlewares() []echo.MiddlewareFunc {
	authMw := middleware.AuthMiddleware(e.Context(), middleware.WithAuthPurpose(jwt.PurposeLogin))
	accessMw := middleware.AccessMiddleware(e.Context())
	return []echo.MiddlewareFunc{authMw, accessMw}
}

func (e *QuotaExtension) handleQuotaStatus(c echo.Context) error {
	ctx := httputil.Context(c)
	userID, ok := e.getUser(ctx)
	if !ok {
		apiErr := NewError(ErrKeyUnauthorized, errors.New("failed to get user ID"))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	// Get user's quota config to determine policy
	config, err := e.quotaService.GetConfigManager().GetUserQuotaConfig(ctx.Request().Context(), userID)
	if err != nil {
		return e.handleQuotaError(ctx, "failed to get user quota config", err)
	}

	var response dto.QuotaStatusResponse

	// Handle based on enforcement policy
	switch config.EnforcementPolicy {
	case models.EnforcementPolicyAllowance:
		// ALLOWANCE: Use ad-hoc grants (PAYG style)
		balance, err := e.quotaService.GetAllowanceBalance(ctx.Request().Context(), userID)
		if err != nil {
			return e.handleQuotaError(ctx, "failed to get allowance balance", err)
		}

		response = dto.QuotaStatusResponse{
			Upload:   e.buildQuotaTypeStatus(balance.UploadUsed, balance.UploadAllowance, balance.UploadRemaining),
			Download: e.buildQuotaTypeStatus(balance.DownloadUsed, balance.DownloadAllowance, balance.DownloadRemaining),
			Storage:  e.buildQuotaTypeStatus(balance.StorageUsed, balance.StorageAllowance, balance.StorageRemaining),
		}

	case models.EnforcementPolicyUnlimited:
		// UNLIMITED: Show usage without limits
		usage, err := e.quotaService.GetUsageManager().GetCurrentUsage(ctx.Request().Context(), userID)
		if err != nil {
			return e.handleQuotaError(ctx, "failed to get current usage", err)
		}

		response = dto.QuotaStatusResponse{
			Upload:   e.buildUnlimitedStatus(usage.BytesUploaded),
			Download: e.buildUnlimitedStatus(usage.BytesDownloaded),
			Storage:  e.buildUnlimitedStatus(usage.BytesStored),
		}

	case models.EnforcementPolicyHardLimits, models.EnforcementPolicyThreshold:
		// HARD_LIMITS and THRESHOLD: Show effective limits and window-based usage
		limits, err := e.quotaService.GetConfigManager().ResolveEffectiveLimits(ctx.Request().Context(), userID)
		if err != nil {
			return e.handleQuotaError(ctx, "failed to resolve effective limits", err)
		}

		// Get window-specific usage and window info for each limit type
		uploadUsage, uploadWindow, err := e.getUsageForLimit(ctx, userID, limits.UploadLimitConfig, quotaCore.UsageTypeUpload)
		if err != nil {
			return e.handleQuotaError(ctx, "failed to get upload usage", err)
		}

		downloadUsage, downloadWindow, err := e.getUsageForLimit(ctx, userID, limits.DownloadLimitConfig, quotaCore.UsageTypeDownload)
		if err != nil {
			return e.handleQuotaError(ctx, "failed to get download usage", err)
		}

		storageUsage, storageWindow, err := e.getUsageForLimit(ctx, userID, limits.StorageLimitConfig, quotaCore.UsageTypeStorageAdd)
		if err != nil {
			return e.handleQuotaError(ctx, "failed to get storage usage", err)
		}

		// Build response with window information
		response = dto.QuotaStatusResponse{
			Upload:   e.buildLimitedStatusWithWindow(uploadUsage, e.getLimitBytes(limits.UploadLimitConfig), limits.UploadThreshold, uploadWindow),
			Download: e.buildLimitedStatusWithWindow(downloadUsage, e.getLimitBytes(limits.DownloadLimitConfig), limits.DownloadThreshold, downloadWindow),
			Storage:  e.buildLimitedStatusWithWindow(storageUsage, e.getLimitBytes(limits.StorageLimitConfig), limits.StorageThreshold, storageWindow),
		}

	default:
		e.Logger().Warn("unknown enforcement policy", zap.String("policy", string(config.EnforcementPolicy)))
		apiErr := NewError(ErrKeyQuotaFetchFailed, fmt.Errorf("unknown enforcement policy: %s", config.EnforcementPolicy))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return httputil.EncodeResponse(ctx, response, response)
}

func (e *QuotaExtension) handleQuotaHistory(c echo.Context) error {
	ctx := httputil.Context(c)
	userID, ok := e.getUser(ctx)
	if !ok {
		apiErr := NewError(ErrKeyUnauthorized, errors.New("failed to get user ID"))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	// Parse query parameters directly
	startDate := c.QueryParam("start_date")
	endDate := c.QueryParam("end_date")
	quotaType := c.QueryParam("type")

	// Validate required parameters
	if startDate == "" || endDate == "" {
		apiErr := NewError(ErrKeyInvalidRequest, errors.New("startDate and endDate are required"))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	// Parse and validate dates
	startTime, err := time.Parse(time.RFC3339, startDate)
	if err != nil {
		apiErr := NewError(ErrKeyInvalidRequest, fmt.Errorf("invalid startDate format: %w", err))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	endTime, err := time.Parse(time.RFC3339, endDate)
	if err != nil {
		apiErr := NewError(ErrKeyInvalidRequest, fmt.Errorf("invalid endDate format: %w", err))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	// Determine usage type
	var usageType quotaCore.UsageType
	switch quotaType {
	case "upload":
		usageType = quotaCore.UsageTypeUpload
	default:
		usageType = quotaCore.UsageTypeDownload
	}

	usagePoints, err := e.quotaService.GetUsageHistoryDateRange(ctx.Request().Context(), userID, usageType, startTime, endTime)
	if err != nil {
		e.Logger().Error("failed to get quota history", zap.Error(err))
		apiErr := NewError(ErrKeyHistoryFetchFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	points := e.convertUsagePoints(usagePoints)

	response := dto.QuotaHistoryResponse{
		UserID: userID,
		Points: points,
	}

	return httputil.EncodeResponse(ctx, response, response)
}

func (e *QuotaExtension) buildQuotaTypeStatus(used, limit, remaining uint64) dto.QuotaTypeStatus {
	percentage := e.calculateProgress(used, limit)
	return dto.QuotaTypeStatus{
		Used:       used,
		Limit:      &limit,
		Remaining:  &remaining,
		Percentage: &percentage,
	}
}

func (e *QuotaExtension) calculateProgress(used uint64, limit uint64) int {
	if limit == 0 {
		return 0
	}
	percentage := int((float64(used) / float64(limit)) * 100)
	if percentage > 100 {
		return 100
	}
	return percentage
}

func (e *QuotaExtension) convertUsagePoints(points []*quotaCore.UsagePoint) []dto.UsagePoint {
	result := make([]dto.UsagePoint, len(points))
	for i, point := range points {
		result[i] = dto.UsagePoint{
			Date:  point.Date.Format(time.RFC3339),
			Bytes: point.Bytes,
		}
	}
	return result
}

func (e *QuotaExtension) getUser(ctx httputil.RequestContext) (uint, bool) {
	user, err := mcontext.GetUserID(ctx.Context)
	if err != nil {
		return 0, false
	}
	return user, true
}

// handleQuotaError handles quota-related errors with consistent logging and response formatting
func (e *QuotaExtension) handleQuotaError(ctx httputil.RequestContext, msg string, err error) error {
	e.Logger().Error(msg, zap.Error(err))
	apiErr := NewError(ErrKeyQuotaFetchFailed, err)
	return ctx.Error(apiErr, apiErr.HttpStatus())
}

// buildUnlimitedStatus builds a quota type status for unlimited usage
func (e *QuotaExtension) buildUnlimitedStatus(used uint64) dto.QuotaTypeStatus {
	return dto.QuotaTypeStatus{
		Used:       used,
		Limit:      nil, // nil indicates unlimited
		Remaining:  nil,
		Percentage: nil,
	}
}

// buildLimitedStatus builds a quota type status for limits and thresholds
func (e *QuotaExtension) buildLimitedStatus(used uint64, limit, threshold *uint64) dto.QuotaTypeStatus {
	var remaining *uint64
	var percentage *int

	// Calculate remaining if limit is set
	if limit != nil {
		if used < *limit {
			r := *limit - used
			remaining = &r
		} else {
			zero := uint64(0)
			remaining = &zero
		}

		// Calculate percentage
		p := e.calculateProgress(used, *limit)
		percentage = &p
	}

	return dto.QuotaTypeStatus{
		Used:       used,
		Limit:      limit,
		Remaining:  remaining,
		Percentage: percentage,
		Threshold:  threshold,
	}
}

// getLimitBytes extracts the byte limit from a LimitConfig
func (e *QuotaExtension) getLimitBytes(limitConfig *quotaCore.Limit) *uint64 {
	if limitConfig == nil {
		return nil
	}
	return &limitConfig.Bytes
}

// getUsageForLimit gets usage and window info for a specific limit
func (e *QuotaExtension) getUsageForLimit(ctx httputil.RequestContext, userID uint, limitConfig *quotaCore.Limit, usageType quotaCore.UsageType) (uint64, *dto.WindowInfo, error) {
	if limitConfig == nil || limitConfig.Window.IsNil() {
		return 0, nil, nil
	}

	// Get usage for this window
	currentUsage, startTime, endTime, err := e.quotaService.GetUsageManager().GetUsageForWindow(
		ctx.Request().Context(),
		userID,
		usageType,
		limitConfig.Window,
	)
	if err != nil {
		return 0, nil, err
	}

	// Build window info
	windowInfo := &dto.WindowInfo{
		Type: string(limitConfig.Window.Type),
	}

	// Add start/end dates for all windows
	if !startTime.IsZero() {
		startDate := startTime.Format(time.RFC3339)
		windowInfo.StartDate = &startDate
	}
	if !endTime.IsZero() {
		endDate := endTime.Format(time.RFC3339)
		windowInfo.EndDate = &endDate
	}

	// Add duration for rolling windows
	if limitConfig.Window.Type == quotaCore.WindowTypeRolling && limitConfig.Window.Duration != nil {
		windowInfo.Duration = limitConfig.Window.Duration
	}

	// Add timezone if configured
	if limitConfig.Window.Timezone != nil {
		windowInfo.Timezone = limitConfig.Window.Timezone
	}

	return currentUsage, windowInfo, nil
}

// buildLimitedStatusWithWindow builds quota type status with window information
func (e *QuotaExtension) buildLimitedStatusWithWindow(used uint64, limit, threshold *uint64, window *dto.WindowInfo) dto.QuotaTypeStatus {
	var remaining *uint64
	var percentage *int

	// Calculate remaining if limit is set
	if limit != nil {
		if used < *limit {
			r := *limit - used
			remaining = &r
		} else {
			zero := uint64(0)
			remaining = &zero
		}

		// Calculate percentage
		p := e.calculateProgress(used, *limit)
		percentage = &p
	}

	return dto.QuotaTypeStatus{
		Used:       used,
		Limit:      limit,
		Remaining:  remaining,
		Percentage: percentage,
		Threshold:  threshold,
		Window:     window,
	}
}
