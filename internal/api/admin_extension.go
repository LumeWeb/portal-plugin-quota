package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"go.lumeweb.com/portal-middleware/auth/jwt"
	"go.lumeweb.com/portal-middleware/middleware"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/queryutil"
	queryutilHttp "go.lumeweb.com/queryutil/http"
	quotaCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.lumeweb.com/portal-plugin-quota/internal/api/dto"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	router "go.lumeweb.com/portal-router"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type QuotaAdminExtension struct {
	*core.BaseComponent
	quotaService quotaCore.QuotaService
}

func NewQuotaAdminExtension() core.APIExtensionFactory {
	return func() (core.APIExtension, []core.ContextBuilderOption, error) {
		ext := &QuotaAdminExtension{}

		return ext, core.ContextOptions(
			core.ContextWithStartupFunc(func(ctx core.Context) error {
				ext.quotaService = core.GetService[quotaCore.QuotaService](ctx, internal.PluginName)
				return nil
			}),
		), nil
	}
}

// TargetAPI returns the API this extension extends
func (e *QuotaAdminExtension) TargetAPI() string {
	return "admin"
}

// Name returns the extension name
func (e *QuotaAdminExtension) Name() string {
	return internal.PluginName
}

// ID returns the extension ID
func (e *QuotaAdminExtension) ID() string {
	return e.Name()
}

// Configure adds routes to the Admin API
func (e *QuotaAdminExtension) Configure(gRouter router.Router, accessSvc core.AccessService) error {
	// Create a subrouter for quota admin routes
	quotaRouter, err := gRouter.Group("/api/quota")
	if err != nil {
		e.Logger().Error("Failed to create quota router group", zap.Error(err))
		return err
	}

	// Register quota admin routes
	if err := e.registerQuotaHandlers(quotaRouter, accessSvc); err != nil {
		return err
	}

	return nil
}

// registerQuotaHandlers registers quota management routes
func (e *QuotaAdminExtension) registerQuotaHandlers(gRouter router.Router, accessSvc core.AccessService) error {
	routes := e.buildRoutes()
	apiGroup := core.GetAPI(e.TargetAPI()).Subdomain()
	
	e.Logger().Info("Registering admin extension routes", 
		zap.String("group", apiGroup),
		zap.Int("routes", len(routes)))
	
	if err := router.RegisterRoutes(gRouter, accessSvc, apiGroup, routes); err != nil {
		e.Logger().Error("Failed to register routes", zap.Error(err))
		return err
	}
	
	e.Logger().Info("Successfully registered admin extension routes")
	
	return nil
}

func (e *QuotaAdminExtension) buildRoutes() []router.Route {
	// Create reusable schema for QuotaPlanResponse
	planSchema := queryutil.NewSchemaProvider().ForType(&dto.QuotaPlanResponse{})

	// Create reusable schema for AllowanceGrantResponse
	grantSchema := queryutil.NewSchemaProvider().ForType(&dto.AllowanceGrantResponse{})

	// Create reusable schema for UserQuotaConfigResponse
	userConfigSchema := queryutil.NewSchemaProvider().ForType(&dto.UserQuotaConfigResponse{})

	return []router.Route{
		// Quota Plan Management
		e.newRoute(http.MethodGet, "/plans", e.handleListPlans,
			router.WithSummary("List quota plans"),
			router.WithDescription("Get a paginated list of all quota plans"),
			router.WithTags("quota", "plans"),
			router.WithSchema(planSchema),
			router.WithFilterParamsFromSchema(planSchema),
			router.WithSuccessResponse(http.StatusOK, "List of quota plans", router.WithJSONContent(&dto.PlanListResponse{})),
		),
		e.newRoute(http.MethodPost, "/plans", e.handleCreatePlan,
			router.WithSummary("Create quota plan"),
			router.WithDescription("Create a new quota plan with specified limits"),
			router.WithTags("quota", "plans"),
			router.WithRequestBody(&dto.QuotaPlanRequest{}, "Quota plan configuration", true),
			router.WithSuccessResponse(http.StatusCreated, "Plan created successfully", router.WithJSONContent(&dto.QuotaPlanResponse{})),
		),
		e.newRoute(http.MethodGet, "/plans/:planID", e.handleGetPlan,
			router.WithSummary("Get quota plan"),
			router.WithDescription("Get details of a specific quota plan by ID"),
			router.WithTags("quota", "plans"),
			router.WithPathParam("planID", "Numeric ID of the quota plan", ""),
			router.WithSuccessResponse(http.StatusOK, "Quota plan details", router.WithJSONContent(&dto.QuotaPlanResponse{})),
		),
		e.newRoute(http.MethodPut, "/plans/:planID", e.handleUpdatePlan,
			router.WithSummary("Update quota plan"),
			router.WithDescription("Update an existing quota plan"),
			router.WithTags("quota", "plans"),
			router.WithPathParam("planID", "Numeric ID of the quota plan", ""),
			router.WithRequestBody(&dto.QuotaPlanRequest{}, "Quota plan configuration", true),
			router.WithSuccessResponse(http.StatusOK, "Plan updated successfully", router.WithJSONContent(&dto.QuotaPlanResponse{})),
		),
		e.newRoute(http.MethodDelete, "/plans/:planID", e.handleDeletePlan,
			router.WithSummary("Delete quota plan"),
			router.WithDescription("Delete a quota plan by ID"),
			router.WithTags("quota", "plans"),
			router.WithPathParam("planID", "Numeric ID of the quota plan", ""),
			router.WithoutDefaultSuccessResponse(),
			router.WithSuccessResponse(http.StatusNoContent, "Plan deleted successfully"),
		),
		e.newRoute(http.MethodPost, "/plans/:planID/default", e.handleSetDefaultPlan,
			router.WithSummary("Set default quota plan"),
			router.WithDescription("Set a quota plan as the default for new users"),
			router.WithTags("quota", "plans"),
			router.WithPathParam("planID", "Numeric ID of the quota plan", ""),
			router.WithoutDefaultSuccessResponse(),
			router.WithSuccessResponse(http.StatusNoContent, "Default plan set successfully"),
		),

		// User Quota Config Management
		e.newRoute(http.MethodGet, "/user-configs", e.handleListUserQuotaConfigs,
			router.WithSummary("List user quota configs"),
			router.WithDescription("Get a paginated list of user quota configurations with filtering and sorting"),
			router.WithTags("quota", "user-configs"),
			router.WithSchema(userConfigSchema),
			router.WithFilterParamsFromSchema(userConfigSchema),
			router.WithSuccessResponse(http.StatusOK, "List of user quota configs", router.WithJSONContent(&dto.UserQuotaConfigListResponse{})),
		),
		e.newRoute(http.MethodPut, "/user-configs/:userID", e.handleUpdateUserQuotaConfig,
			router.WithSummary("Update user quota config"),
			router.WithDescription("Update a user's quota configuration"),
			router.WithTags("quota", "user-configs"),
			router.WithPathParam("userID", "Numeric ID of the user", ""),
			router.WithRequestBody(&dto.UserQuotaConfigUpdateRequest{}, "User quota config update", true),
			router.WithSuccessResponse(http.StatusOK, "User quota config updated", router.WithJSONContent(&dto.UserQuotaConfigResponse{})),
		),
		e.newRoute(http.MethodDelete, "/user-configs/:userID/plan", e.handleResetUserQuotaPlan,
			router.WithSummary("Reset user quota plan"),
			router.WithDescription("Remove a user's assigned quota plan (sets to NULL)"),
			router.WithTags("quota", "user-configs"),
			router.WithPathParam("userID", "Numeric ID of the user", ""),
			router.WithoutDefaultSuccessResponse(),
			router.WithSuccessResponse(http.StatusNoContent, "User quota plan reset successfully"),
		),

		// Allowance Management
		e.newRoute(http.MethodGet, "/allowances", e.handleListAllowances,
			router.WithSummary("List allowance grants"),
			router.WithDescription("Get a paginated list of all allowance grants"),
			router.WithTags("quota", "grants"),
			router.WithSchema(grantSchema),
			router.WithFilterParamsFromSchema(grantSchema),
			router.WithSuccessResponse(http.StatusOK, "List of allowance grants", router.WithJSONContent(&dto.AllowanceListResponse{})),
		),
		e.newRoute(http.MethodPost, "/allowances", e.handleCreateGrant,
			router.WithSummary("Create allowance grant"),
			router.WithDescription("Create a new allowance grant for a user"),
			router.WithTags("quota", "grants"),
			router.WithRequestBody(&dto.AllowanceGrantRequest{}, "Allowance grant configuration", true),
			router.WithSuccessResponse(http.StatusCreated, "Grant created", router.WithJSONContent(&dto.AllowanceGrantResponse{})),
		),
		e.newRoute(http.MethodPut, "/allowances/:grantID", e.handleUpdateGrant,
			router.WithSummary("Update allowance grant"),
			router.WithDescription("Update an existing allowance grant"),
			router.WithTags("quota", "grants"),
			router.WithPathParam("grantID", "Numeric ID of the allowance grant", ""),
			router.WithRequestBody(&dto.AllowanceGrantRequest{}, "Allowance grant configuration", true),
			router.WithSuccessResponse(http.StatusOK, "Grant updated", router.WithJSONContent(&dto.AllowanceGrantResponse{})),
		),
		e.newRoute(http.MethodDelete, "/allowances/:grantID", e.handleDeleteGrant,
			router.WithSummary("Deactivate allowance grant"),
			router.WithDescription("Deactivate an allowance grant"),
			router.WithTags("quota", "grants"),
			router.WithPathParam("grantID", "Numeric ID of the allowance grant", ""),
			router.WithoutDefaultSuccessResponse(),
			router.WithSuccessResponse(http.StatusNoContent, "Grant deactivated"),
		),

		// System Management
		e.newRoute(http.MethodGet, "/system/stats", e.handleSystemStats,
			router.WithSummary("Get system statistics"),
			router.WithDescription("Get system-wide quota usage statistics"),
			router.WithTags("quota", "system"),
			router.WithSuccessResponse(http.StatusOK, "System statistics", router.WithJSONContent(&dto.SystemStatsResponse{})),
		),
		e.newRoute(http.MethodPost, "/system/reconcile", e.handleReconcile,
			router.WithSummary("Reconcile quota usage"),
			router.WithDescription("Reconcile user quota usage with actual storage/files"),
			router.WithTags("quota", "system"),
			router.WithRequestBody(&dto.ReconcileRequest{}, "Reconciliation options", true),
			router.WithSuccessResponse(http.StatusOK, "Reconciliation result", router.WithJSONContent(&dto.ReconcileResponse{})),
		),
		e.newRoute(http.MethodPost, "/system/cleanup", e.handleCleanup,
			router.WithSummary("Cleanup old records"),
			router.WithDescription("Delete old quota records based on retention policy"),
			router.WithTags("quota", "system"),
			router.WithRequestBody(&dto.CleanupRequest{}, "Cleanup options", true),
			router.WithSuccessResponse(http.StatusOK, "Cleanup result", router.WithJSONContent(&dto.CleanupResponse{})),
		),
	}
}

func (e *QuotaAdminExtension) buildMiddlewares() []echo.MiddlewareFunc {
	authMw := middleware.AuthMiddleware(e.Context(), middleware.WithAuthPurpose(jwt.PurposeLogin))
	accessMw := middleware.AccessMiddleware(e.Context())
	return []echo.MiddlewareFunc{authMw, accessMw}
}

func (e *QuotaAdminExtension) newRoute(method, path string, handler echo.HandlerFunc, opts ...router.SwaggerOption) router.Route {
	return router.NewRoute(method, path, handler, 
		router.WithAccess(core.ACCESS_ADMIN_ROLE),
		router.WithSwaggerOptions(opts...),
	)
}

// Quota Plan Management Handlers
func (e *QuotaAdminExtension) handleListPlans(c echo.Context) error {
	ctx := httputil.Context(c)

	return queryutilHttp.ProcessListRequest(
		c.Response(),
		c.Request(),
		"plans",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*models.QuotaPlan, int64, error) {
			return e.quotaService.ListQuotaPlans(ctx.Request().Context(), filters, sorts, pagination)
		},
		func(plan *models.QuotaPlan) dto.QuotaPlanResponse {
			// Build window type pointer
			var windowType *string
			if plan.WindowType != "" {
				wt := plan.WindowType.String()
				windowType = &wt
			}

			return dto.QuotaPlanResponse{
				ID:                plan.ID,
				Name:              plan.Name,
				Description:       plan.Description,
				WindowType:        windowType,
				WindowDuration:    plan.WindowDuration,
				WindowStartHour:   plan.WindowStartHour,
				WindowTimezone:    plan.WindowTimezone,
				StorageLimitBytes: &plan.StorageLimitBytes,
				UploadLimitBytes:  &plan.UploadLimitBytes,
				DownloadLimitBytes: &plan.DownloadLimitBytes,
				StorageThreshold:  plan.StorageThreshold,
				UploadThreshold:   plan.UploadThreshold,
				DownloadThreshold: plan.DownloadThreshold,
				IsDefault:         plan.IsDefault,
				IsActive:          plan.IsActive != nil && *plan.IsActive,
				CreatedAt:         plan.CreatedAt,
				UpdatedAt:         plan.UpdatedAt,
			}
		},
	)
}

func (e *QuotaAdminExtension) handleCreatePlan(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	var req dto.QuotaPlanRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.QuotaPlanRequest, *dto.QuotaPlanRequest](ctx, &req)
	if !ok {
		return nil
	}

	plan := &models.QuotaPlan{
		Name:              req.Name,
		Description:       req.Description,
		WindowDuration:    req.WindowDuration,
		WindowStartHour:   req.WindowStartHour,
		WindowTimezone:    req.WindowTimezone,
		StorageLimitBytes: uint64PtrValue(req.StorageLimitBytes),
		UploadLimitBytes:  uint64PtrValue(req.UploadLimitBytes),
		DownloadLimitBytes: uint64PtrValue(req.DownloadLimitBytes),
		StorageThreshold:  req.StorageThreshold,
		UploadThreshold:   req.UploadThreshold,
		DownloadThreshold: req.DownloadThreshold,
		IsDefault:         false,
		IsActive:          req.IsActive,
	}
	
	// Set WindowType if provided
	if req.WindowType != nil {
		plan.WindowType = models.WindowType(*req.WindowType)
	}

	if err := e.quotaService.CreateQuotaPlan(reqCtx, plan); err != nil {
		if errors.Is(err, models.ErrQuotaPlanNameExists) {
			e.Logger().Error("quota plan with this name already exists", zap.Error(err))
			apiErr := NewError(ErrKeyPlanNameExists, err)
			return ctx.Error(apiErr, apiErr.HttpStatus())
		}
		e.Logger().Error("failed to create quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyPlanCreateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	ctx.Response().Before(func() {
		ctx.Response().Status = http.StatusCreated
	})

	return httputil.EncodeResponse(ctx, plan, &dto.QuotaPlanResponse{})
}

func (e *QuotaAdminExtension) handleGetPlan(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	planID, err := parseUintParam(c, "planID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidPlanID, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	plan, err := e.quotaService.GetQuotaPlan(reqCtx, planID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiErr := NewError(ErrKeyPlanNotFound, err)
			return ctx.Error(apiErr, apiErr.HttpStatus())
		}
		e.Logger().Error("failed to get quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyConfigFetchFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return httputil.EncodeResponse(ctx, plan, &dto.QuotaPlanResponse{})
}

func (e *QuotaAdminExtension) handleUpdatePlan(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	planID, err := parseUintParam(c, "planID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidPlanID, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	var req dto.QuotaPlanRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.QuotaPlanRequest, *dto.QuotaPlanRequest](ctx, &req)
	if !ok {
		return nil
	}

	plan, err := e.quotaService.GetQuotaPlan(reqCtx, planID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiErr := NewError(ErrKeyPlanNotFound, err)
			return ctx.Error(apiErr, apiErr.HttpStatus())
		}
		e.Logger().Error("failed to get quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyConfigFetchFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	plan.Name = req.Name
	plan.Description = req.Description
	
	// Set window configuration fields
	if req.WindowType != nil {
		plan.WindowType = models.WindowType(*req.WindowType)
	}
	plan.WindowDuration = req.WindowDuration
	plan.WindowStartHour = req.WindowStartHour
	plan.WindowTimezone = req.WindowTimezone
	
	// Set byte limits
	plan.StorageLimitBytes = uint64PtrValue(req.StorageLimitBytes)
	plan.UploadLimitBytes = uint64PtrValue(req.UploadLimitBytes)
	plan.DownloadLimitBytes = uint64PtrValue(req.DownloadLimitBytes)
	
	// Set thresholds
	plan.StorageThreshold = req.StorageThreshold
	plan.UploadThreshold = req.UploadThreshold
	plan.DownloadThreshold = req.DownloadThreshold
	
	// Set active status
	plan.IsActive = req.IsActive

	if err := e.quotaService.UpdateQuotaPlan(reqCtx, planID, plan); err != nil {
		e.Logger().Error("failed to update quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyPlanUpdateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return httputil.EncodeResponse(ctx, plan, &dto.QuotaPlanResponse{})
}

func (e *QuotaAdminExtension) handleDeletePlan(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	planID, err := parseUintParam(c, "planID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidPlanID, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	if err := e.quotaService.DeleteQuotaPlan(reqCtx, planID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiErr := NewError(ErrKeyPlanNotFound, err)
			return ctx.Error(apiErr, apiErr.HttpStatus())
		}
		// Check if the error indicates the plan is in use
		if strings.Contains(err.Error(), "users assigned") {
			apiErr := NewError(ErrKeyPlanInUse, err)
			return ctx.Error(apiErr, apiErr.HttpStatus())
		}
		e.Logger().Error("failed to delete quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyPlanDeleteFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return c.NoContent(http.StatusNoContent)
}

func (e *QuotaAdminExtension) handleSetDefaultPlan(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	planID, err := parseUintParam(c, "planID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidPlanID, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	if err := e.quotaService.SetDefaultQuotaPlan(reqCtx, planID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			apiErr := NewError(ErrKeyPlanNotFound, err)
			return ctx.Error(apiErr, apiErr.HttpStatus())
		}
		e.Logger().Error("failed to set default quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyConfigUpdateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return c.NoContent(http.StatusNoContent)
}

// User Quota Config Management Handlers
func (e *QuotaAdminExtension) handleListUserQuotaConfigs(c echo.Context) error {
	ctx := httputil.Context(c)

	return queryutilHttp.ProcessListRequest(
		c.Response(),
		c.Request(),
		"user-configs",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*models.UserQuotaConfig, int64, error) {
			return e.quotaService.ListUserQuotaConfigs(ctx.Request().Context(), filters, sorts, pagination)
		},
		func(config *models.UserQuotaConfig) dto.UserQuotaConfigResponse {
			// Build window type pointer
			var windowType *string
			if config.WindowType != "" {
				wt := config.WindowType.String()
				windowType = &wt
			}

			return dto.UserQuotaConfigResponse{
				ID:                config.ID,
				UserID:            config.UserID,
				EnforcementPolicy: string(config.EnforcementPolicy),
				QuotaPlanID:       config.QuotaPlanID,
				WindowType:        windowType,
				WindowDuration:    config.WindowDuration,
				WindowStartHour:   config.WindowStartHour,
				WindowTimezone:    config.WindowTimezone,
				StorageLimitBytes: &config.StorageLimitBytes,
				UploadLimitBytes:  &config.UploadLimitBytes,
				DownloadLimitBytes: &config.DownloadLimitBytes,
				StorageThreshold:  config.StorageThreshold,
				UploadThreshold:   config.UploadThreshold,
				DownloadThreshold: config.DownloadThreshold,
				CreatedAt:         config.CreatedAt,
				UpdatedAt:         config.UpdatedAt,
			}
		},
	)
}

func (e *QuotaAdminExtension) handleUpdateUserQuotaConfig(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	userID, err := parseUintParam(c, "userID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidRequestParameters, fmt.Errorf("invalid user ID: %w", err))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	var req dto.UserQuotaConfigUpdateRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.UserQuotaConfigUpdateRequest, *dto.UserQuotaConfigUpdateRequest](ctx, &req)
	if !ok {
		return nil
	}

	// Convert request to core update struct
	update := &quotaCore.UserQuotaConfigUpdate{}

	update.EnforcementPolicy = req.EnforcementPolicy
	update.QuotaPlanID = req.QuotaPlanID
	update.WindowType = req.WindowType
	update.WindowDuration = req.WindowDuration
	update.WindowStartHour = req.WindowStartHour
	update.WindowTimezone = req.WindowTimezone
	update.StorageLimitBytes = req.StorageLimitBytes
	update.UploadLimitBytes = req.UploadLimitBytes
	update.DownloadLimitBytes = req.DownloadLimitBytes
	update.StorageThreshold = req.StorageThreshold
	update.UploadThreshold = req.UploadThreshold
	update.DownloadThreshold = req.DownloadThreshold

	config, err := e.quotaService.UpdateUserQuotaConfig(reqCtx, userID, update)
	if err != nil {
		e.Logger().Error("failed to update user quota config", zap.Error(err))
		apiErr := NewError(ErrKeyConfigUpdateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return httputil.EncodeResponse(ctx, config, &dto.UserQuotaConfigResponse{})
}

func (e *QuotaAdminExtension) handleResetUserQuotaPlan(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	userID, err := parseUintParam(c, "userID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidRequestParameters, fmt.Errorf("invalid user ID: %w", err))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	if err := e.quotaService.ResetUserQuotaPlan(reqCtx, userID); err != nil {
		e.Logger().Error("failed to reset user quota plan", zap.Error(err))
		apiErr := NewError(ErrKeyConfigUpdateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return c.NoContent(http.StatusNoContent)
}

// Allowance Management Handlers
func (e *QuotaAdminExtension) handleListAllowances(c echo.Context) error {
	ctx := httputil.Context(c)

	grantManager := e.quotaService.GetGrantManager()

	return queryutilHttp.ProcessListRequest(
		c.Response(),
		c.Request(),
		"allowances",
		func(filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*models.AllowanceGrant, int64, error) {
			return grantManager.ListGrants(ctx.Request().Context(), filters, sorts, pagination)
		},
		func(grant *models.AllowanceGrant) dto.AllowanceGrantResponse {
			return dto.AllowanceGrantResponse{
				ID:             grant.ID,
				UserID:         grant.UserID,
				Type:           grant.Type.String(),
				Source:         string(grant.Source),
				Bytes:          grant.Bytes,
				BytesUsed:      grant.BytesUsed,
				BytesRemaining: grant.BytesRemaining,
				ExpiryDate:     grant.ExpiryDate,
				IsActive:       grant.IsActive,
				CreatedAt:       grant.CreatedAt,
				UpdatedAt:       grant.UpdatedAt,
			}
		},
	)
}

func (e *QuotaAdminExtension) handleCreateGrant(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	var req dto.AllowanceGrantRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.AllowanceGrantRequest, *dto.AllowanceGrantRequest](ctx, &req)
	if !ok {
		return nil
	}

	grant := &models.AllowanceGrant{
		UserID:     req.UserID,
		Type:       req.Type,
		Source:     models.GrantSource(req.Source),
		Bytes:      req.Storage + req.Upload + req.Download,
		BytesUsed:  0,
		ExpiryDate: req.ExpiryDate,
		IsActive:   true,
	}

	grantManager := e.quotaService.GetGrantManager()
	if err := grantManager.CreateAllowanceGrant(reqCtx, req.UserID, grant); err != nil {
		e.Logger().Error("failed to create allowance grant", zap.Error(err))
		apiErr := NewError(ErrKeyGrantCreateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	ctx.Response().Before(func() {
		ctx.Response().Status = http.StatusCreated
	})

	return httputil.EncodeResponse(ctx, grant, &dto.AllowanceGrantResponse{})
}

func (e *QuotaAdminExtension) handleUpdateGrant(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	grantID, err := parseUintParam(c, "grantID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidGrantID, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	var req dto.AllowanceGrantRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.AllowanceGrantRequest, *dto.AllowanceGrantRequest](ctx, &req)
	if !ok {
		return nil
	}

	grantManager := e.quotaService.GetGrantManager()

	targetGrant, err := grantManager.GetGrantByID(reqCtx, grantID)
	if err != nil {
		e.Logger().Error("failed to get grant", zap.Error(err))
		apiErr := NewError(ErrKeyGrantFetchFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	if targetGrant == nil {
		apiErr := NewError(ErrKeyGrantNotFound, fmt.Errorf("grant not found"))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	totalBytes := req.Storage + req.Upload + req.Download
	if totalBytes < targetGrant.Bytes {
		apiErr := NewError(ErrKeyInvalidRequestParameters, fmt.Errorf("cannot decrease grant bytes"))
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	targetGrant.Bytes = totalBytes
	if req.ExpiryDate != nil {
		targetGrant.ExpiryDate = req.ExpiryDate
	}

	if err := grantManager.UpdateAllowanceGrant(reqCtx, targetGrant); err != nil {
		e.Logger().Error("failed to update grant", zap.Error(err))
		apiErr := NewError(ErrKeyUpdateFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return httputil.EncodeResponse(ctx, targetGrant, &dto.AllowanceGrantResponse{})
}

func (e *QuotaAdminExtension) handleDeleteGrant(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	grantID, err := parseUintParam(c, "grantID")
	if err != nil {
		apiErr := NewError(ErrKeyInvalidGrantID, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	grantManager := e.quotaService.GetGrantManager()
	if err := grantManager.DeactivateGrant(reqCtx, grantID); err != nil {
		e.Logger().Error("failed to deactivate grant", zap.Error(err))
		apiErr := NewError(ErrKeyGrantDeleteFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	return c.NoContent(http.StatusNoContent)
}

// System Management Handlers
func (e *QuotaAdminExtension) handleSystemStats(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	stats, err := e.quotaService.GetSystemStats(reqCtx)
	if err != nil {
		e.Logger().Error("failed to get system stats", zap.Error(err))
		apiErr := NewError(ErrKeyConfigFetchFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	response := dto.SystemStatsResponse{
		TotalUsers:   stats.TotalUsers,
		ActiveUsers:  stats.ActiveUsers,
		TotalPlans:   stats.TotalPlans,
		ActivePlans:  stats.ActivePlans,
		TotalGrants:  stats.TotalGrants,
		ActiveGrants: stats.ActiveGrants,
		CurrentUsage: dto.Usage{
			StorageBytes:  stats.CurrentUsage.BytesStored,
			UploadBytes:   stats.CurrentUsage.BytesUploaded,
			DownloadBytes: stats.CurrentUsage.BytesDownloaded,
		},
		TotalUsageBytes: stats.TotalUsageBytes,
	}

	return httputil.EncodeResponse(ctx, response, response)
}

func (e *QuotaAdminExtension) handleReconcile(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	var req dto.ReconcileRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.ReconcileRequest, *dto.ReconcileRequest](ctx, &req)
	if !ok {
		return nil
	}

	if err := e.quotaService.Reconcile(reqCtx); err != nil {
		e.Logger().Error("failed to reconcile quota", zap.Error(err))
		apiErr := NewError(ErrKeyReconcilationFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	response := dto.ReconcileResponse{
		UsersProcessed: 0,
		Message:        "Reconciliation completed successfully",
	}

	return httputil.EncodeResponse(ctx, response, response)
}

func (e *QuotaAdminExtension) handleCleanup(c echo.Context) error {
	ctx := httputil.Context(c)
	reqCtx := ctx.Context.Request().Context()

	var req dto.CleanupRequest
	_, ok := httputil.DecodeAndValidateRequest[*dto.CleanupRequest, *dto.CleanupRequest](ctx, &req)
	if !ok {
		return nil
	}

	if req.RetentionDays <= 0 {
		err := fmt.Errorf("retention_days must be positive")
		apiErr := NewError(ErrKeyRetentionDaysInvalid, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	deletedCount, err := e.quotaService.CleanupOldRecords(reqCtx, req.RetentionDays)
	if err != nil {
		e.Logger().Error("failed to cleanup old records", zap.Error(err))
		apiErr := NewError(ErrKeyCleanupFailed, err)
		return ctx.Error(apiErr, apiErr.HttpStatus())
	}

	response := dto.CleanupResponse{
		RecordsDeleted: deletedCount,
	}

	return httputil.EncodeResponse(ctx, response, response)
}

// Helper functions for DTO conversions

func int64PtrValue(ptr *int64) int64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func parseUintParam(c echo.Context, name string) (uint, error) {
	var val string
	if v := c.Param(name); v != "" {
		val = v
	} else {
		return 0, fmt.Errorf("missing parameter: %s", name)
	}

	planID, err := strconv.ParseUint(val, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: must be a number", name)
	}

	return uint(planID), nil
}

func parseGrantType(s string) (models.GrantType, error) {
	switch s {
	case "storage", "":
		return models.GrantTypeStorage, nil
	case "upload":
		return models.GrantTypeUpload, nil
	case "download":
		return models.GrantTypeDownload, nil
	default:
		return "", fmt.Errorf("invalid grant type: %s", s)
	}
}

// Helper functions for new DTO field conversions

func uint64PtrValue(ptr *uint64) uint64 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func uintPtrValue(ptr *uint) uint {
	if ptr == nil {
		return 0
	}
	return *ptr
}
