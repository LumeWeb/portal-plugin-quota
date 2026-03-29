package dto

import (
	"time"

	"go.lumeweb.com/portal/config"
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/httputil"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

var (
	_ httputil.DTOValidator = (*QuotaPlanRequest)(nil)
	_ httputil.DTORequest[*QuotaPlanRequest] = (*QuotaPlanRequest)(nil)
	_ httputil.DTOValidator = (*AllowanceGrantRequest)(nil)
	_ httputil.DTORequest[*AllowanceGrantRequest] = (*AllowanceGrantRequest)(nil)
	_ httputil.DTOValidator = (*AllowanceListRequest)(nil)
	_ httputil.DTORequest[*AllowanceListRequest] = (*AllowanceListRequest)(nil)
	_ httputil.DTOValidator = (*ReconcileRequest)(nil)
	_ httputil.DTORequest[*ReconcileRequest] = (*ReconcileRequest)(nil)
	_ httputil.DTOValidator = (*CleanupRequest)(nil)
	_ httputil.DTORequest[*CleanupRequest] = (*CleanupRequest)(nil)
	_ httputil.DTOValidator = (*UserQuotaConfigUpdateRequest)(nil)
	_ httputil.DTORequest[*UserQuotaConfigUpdateRequest] = (*UserQuotaConfigUpdateRequest)(nil)
	_ httputil.DTOValidator = (*UserQuotaConfigListRequest)(nil)
	_ httputil.DTORequest[*UserQuotaConfigListRequest] = (*UserQuotaConfigListRequest)(nil)
	_ httputil.DTOResponse[*models.UserQuotaConfig] = (*UserQuotaConfigResponse)(nil)
)

// QuotaPlanRequest describes a quota plan for creation/update
type QuotaPlanRequest struct {
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	StorageLimit       *int64  `json:"storage_limit"`
	UploadDailyLimit   *int64  `json:"upload_daily_limit"`
	DownloadDailyLimit *int64  `json:"download_daily_limit"`
	UploadTotalLimit   *int64  `json:"upload_total_limit"`
	DownloadTotalLimit *int64  `json:"download_total_limit"`
	StorageThreshold   *int64  `json:"storage_threshold"`
	UploadThreshold    *int64  `json:"upload_threshold"`
	DownloadThreshold  *int64  `json:"download_threshold"`
	IsActive           *bool   `json:"is_active"`
}

func (r *QuotaPlanRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"Name":               z.String().Required().Min(1),
		"Description":        z.String().Optional(),
		"StorageLimit":       z.Ptr(z.Int64().GTE(0)),
		"UploadDailyLimit":   z.Ptr(z.Int64().GTE(0)),
		"DownloadDailyLimit": z.Ptr(z.Int64().GTE(0)),
		"UploadTotalLimit":   z.Ptr(z.Int64().GTE(0)),
		"DownloadTotalLimit": z.Ptr(z.Int64().GTE(0)),
		"StorageThreshold":   z.Ptr(z.Int64().GTE(0)),
		"UploadThreshold":    z.Ptr(z.Int64().GTE(0)),
		"DownloadThreshold":  z.Ptr(z.Int64().GTE(0)),
		"IsActive":           z.Ptr(z.Bool()),
	})
}

func (r *QuotaPlanRequest) ToModel() (*QuotaPlanRequest, error) {
	return r, nil
}

// QuotaPlanResponse represents a quota plan
type QuotaPlanResponse struct {
	ID                 uint      `json:"id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	StorageLimit       *int64    `json:"storage_limit"`
	UploadDailyLimit   *int64    `json:"upload_daily_limit"`
	DownloadDailyLimit *int64    `json:"download_daily_limit"`
	UploadTotalLimit   *int64    `json:"upload_total_limit"`
	DownloadTotalLimit *int64    `json:"download_total_limit"`
	StorageThreshold   *int64    `json:"storage_threshold"`
	UploadThreshold    *int64    `json:"upload_threshold"`
	DownloadThreshold  *int64    `json:"download_threshold"`
	IsDefault          bool      `json:"is_default"`
	IsActive           bool      `json:"is_active"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (r *QuotaPlanResponse) FromModel(model *models.QuotaPlan) error {
	if model == nil {
		return nil
	}
	r.ID = model.ID
	r.Name = model.Name
	r.Description = model.Description
	r.StorageLimit = &model.StorageLimit
	r.UploadDailyLimit = &model.UploadDailyLimit
	r.DownloadDailyLimit = &model.DownloadDailyLimit
	r.UploadTotalLimit = &model.UploadTotalLimit
	r.DownloadTotalLimit = &model.DownloadTotalLimit
	r.StorageThreshold = model.StorageThreshold
	r.UploadThreshold = model.UploadThreshold
	r.DownloadThreshold = model.DownloadThreshold
	r.IsDefault = model.IsDefault
	r.IsActive = model.IsActive != nil && *model.IsActive
	r.CreatedAt = model.CreatedAt
	r.UpdatedAt = model.UpdatedAt
	return nil
}

// PlanListResponse is a swagger-only DTO that represents the paginated response for quota plans.
// It mirrors queryutil.Response[*dto.QuotaPlanResponse] for OpenAPI documentation.
//
// This struct exists due to a swagger documentation generation bug where queryutil.Response generics
// are not getting detected properly as an array type. By providing a concrete struct, we ensure the
// swagger docs correctly show the Plans field (mapped to "data" in JSON) as an array of QuotaPlanResponse items.
//
// Note: This struct is only used for swagger documentation, not for actual encoding.
type PlanListResponse struct {
	Plans []QuotaPlanResponse `json:"data"`
	Total int                 `json:"total"`
}

// AllowanceGrantRequest represents a request to create/update a grant
type AllowanceGrantRequest struct {
	UserID     uint               `json:"user_id"`
	Type       models.GrantType `json:"type"`
	Source     models.GrantSource  `json:"source"`
	Storage    uint64             `json:"storage"`
	Upload     uint64             `json:"upload"`
	Download   uint64             `json:"download"`
	ExpiryDate *time.Time         `json:"expiry_date"`
}

func (r *AllowanceGrantRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"UserID": z.UintLike[uint]().Required(),
		"Type": config.ZogStringLike[models.GrantType]().OneOf([]models.GrantType{
			models.GrantTypeStorage,
			models.GrantTypeUpload,
			models.GrantTypeDownload,
		}).Required(),
		"Source": config.ZogStringLike[models.GrantSource]().OneOf([]models.GrantSource{
			models.GrantSourceSubscription,
			models.GrantSourcePAYGAddon,
			models.GrantSourceBonus,
			models.GrantSourcePromo,
		}).Required(),
		"Storage":   z.UintLike[uint64](),
		"Upload":    z.UintLike[uint64](),
		"Download":  z.UintLike[uint64](),
		"ExpiryDate": z.Ptr(z.Time()),
	})
}

func (r *AllowanceGrantRequest) ToModel() (*AllowanceGrantRequest, error) {
	return r, nil
}

// AllowanceListRequest represents query parameters for listing allowance grants
type AllowanceListRequest struct {
	UserID *uint            `query:"user_id" json:"user_id"`
	Type   models.GrantType `query:"type" json:"type"`
}

func (r *AllowanceListRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"UserID": z.Ptr(z.UintLike[uint]()),
		"Type": config.ZogStringLike[models.GrantType]().OneOf([]models.GrantType{
			models.GrantTypeStorage,
			models.GrantTypeUpload,
			models.GrantTypeDownload,
		}),
	})
}

func (r *AllowanceListRequest) ToModel() (*AllowanceListRequest, error) {
	return r, nil
}

// AllowanceGrantResponse represents an allowance grant
type AllowanceGrantResponse struct {
	ID             uint       `json:"id"`
	UserID         uint       `json:"user_id"`
	Type           string     `json:"type"`
	Source         string     `json:"source"`
	Bytes          uint64     `json:"bytes"`
	BytesUsed      uint64     `json:"bytes_used"`
	BytesRemaining uint64     `json:"bytes_remaining"`
	ExpiryDate     *time.Time `json:"expiry_date"`
	IsActive       bool       `json:"is_active"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (r *AllowanceGrantResponse) FromModel(model *models.AllowanceGrant) error {
	if model == nil {
		return nil
	}
	r.ID = model.ID
	r.UserID = model.UserID
	r.Type = model.Type.String()
	r.Source = string(model.Source)
	r.Bytes = model.Bytes
	r.BytesUsed = model.BytesUsed
	r.BytesRemaining = model.BytesRemaining
	r.ExpiryDate = model.ExpiryDate
	r.IsActive = model.IsActive
	r.CreatedAt = model.CreatedAt
	r.UpdatedAt = model.UpdatedAt
	return nil
}

// AllowanceListResponse is a swagger-only DTO that represents the paginated response for allowance grants.
// It mirrors queryutil.Response[*dto.AllowanceGrantResponse] for OpenAPI documentation.
//
// This struct exists due to a swagger documentation generation bug where queryutil.Response generics
// are not getting detected properly as an array type. By providing a concrete struct, we ensure the
// swagger docs correctly show the Grants field (mapped to "data" in JSON) as an array of AllowanceGrantResponse items.
//
// Note: This struct is only used for swagger documentation, not for actual encoding.
type AllowanceListResponse struct {
	Grants []AllowanceGrantResponse `json:"data"`
	Total  int                      `json:"total"`
}

// CleanupRequest represents a request for cleanup operation
type CleanupRequest struct {
	RetentionDays int `json:"retention_days"`
}

func (r *CleanupRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"RetentionDays": z.IntLike[int]().GTE(1).LTE(36500),
	})
}

func (r *CleanupRequest) ToModel() (*CleanupRequest, error) {
	return r, nil
}

// CleanupResponse represents the result of a cleanup operation
type CleanupResponse struct {
	RecordsDeleted int64 `json:"records_deleted"`
}

func (r CleanupResponse) FromModel(_ CleanupResponse) error {
	return nil
}

// ReconcileRequest represents a request for reconciliation (optional user_id)
type ReconcileRequest struct {
	UserID *uint `json:"user_id,omitempty"`
}

func (r *ReconcileRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"UserID": z.Ptr(z.UintLike[uint]()),
	})
}

func (r *ReconcileRequest) ToModel() (*ReconcileRequest, error) {
	return r, nil
}

// ReconcileResponse represents the result of a reconciliation operation
type ReconcileResponse struct {
	UsersProcessed int    `json:"users_processed"`
	Message        string `json:"message"`
}

func (r ReconcileResponse) FromModel(_ ReconcileResponse) error {
	return nil
}

// UserQuotaConfigResponse represents a user quota configuration
// Note: Uses pointer for UserID to avoid gorm.Model embedding issues
type UserQuotaConfigResponse struct {
	ID                 uint      `json:"id"`
	UserID             uint      `json:"user_id"`
	EnforcementPolicy  string    `json:"enforcement_policy"`
	QuotaPlanID        *uint64   `json:"quota_plan_id,omitempty"`
	StorageLimit       *int64    `json:"storage_limit,omitempty"`
	UploadDailyLimit   *int64    `json:"upload_daily_limit,omitempty"`
	DownloadDailyLimit *int64    `json:"download_daily_limit,omitempty"`
	UploadTotalLimit   *int64    `json:"upload_total_limit,omitempty"`
	DownloadTotalLimit *int64    `json:"download_total_limit,omitempty"`
	StorageThreshold   *int64    `json:"storage_threshold,omitempty"`
	UploadThreshold    *int64    `json:"upload_threshold,omitempty"`
	DownloadThreshold  *int64    `json:"download_threshold,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (r *UserQuotaConfigResponse) FromModel(model *models.UserQuotaConfig) error {
	if model == nil {
		return nil
	}
	r.ID = model.ID
	r.UserID = model.UserID
	r.EnforcementPolicy = string(model.EnforcementPolicy)
	r.QuotaPlanID = model.QuotaPlanID
	r.StorageLimit = model.StorageLimit
	r.UploadDailyLimit = model.UploadDailyLimit
	r.DownloadDailyLimit = model.DownloadDailyLimit
	r.UploadTotalLimit = model.UploadTotalLimit
	r.DownloadTotalLimit = model.DownloadTotalLimit
	r.StorageThreshold = model.StorageThreshold
	r.UploadThreshold = model.UploadThreshold
	r.DownloadThreshold = model.DownloadThreshold
	r.CreatedAt = model.CreatedAt
	r.UpdatedAt = model.UpdatedAt
	return nil
}

// UserQuotaConfigListResponse is a swagger-only DTO that represents the paginated response for user quota configs.
// It mirrors queryutil.Response[*dto.UserQuotaConfigResponse] for OpenAPI documentation.
//
// This struct exists due to a swagger documentation generation bug where queryutil.Response generics
// are not getting detected properly as an array type. By providing a concrete struct, we ensure the
// swagger docs correctly show the Configs field (mapped to "data" in JSON) as an array of UserQuotaConfigResponse items.
//
// Note: This struct is only used for swagger documentation, not for actual encoding.
type UserQuotaConfigListResponse struct {
	Configs []UserQuotaConfigResponse `json:"data"`
	Total   int                       `json:"total"`
}

// UserQuotaConfigUpdateRequest represents a request to update a user's quota config
type UserQuotaConfigUpdateRequest struct {
	EnforcementPolicy  *models.EnforcementPolicy `json:"enforcement_policy,omitempty"`
	QuotaPlanID        *uint64                   `json:"quota_plan_id,omitempty"`
	StorageLimit       *int64                    `json:"storage_limit,omitempty"`
	UploadDailyLimit   *int64                    `json:"upload_daily_limit,omitempty"`
	DownloadDailyLimit *int64                    `json:"download_daily_limit,omitempty"`
	UploadTotalLimit   *int64                    `json:"upload_total_limit,omitempty"`
	DownloadTotalLimit *int64                    `json:"download_total_limit,omitempty"`
	StorageThreshold   *int64                    `json:"storage_threshold,omitempty"`
	UploadThreshold    *int64                    `json:"upload_threshold,omitempty"`
	DownloadThreshold  *int64                    `json:"download_threshold,omitempty"`
}

func (r *UserQuotaConfigUpdateRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"EnforcementPolicy":  config.ZogStringLike[models.EnforcementPolicy]().Optional(),
		"QuotaPlanID":        z.Ptr(z.UintLike[uint64]()),
		"StorageLimit":       z.Ptr(z.Int64().GTE(0)),
		"UploadDailyLimit":   z.Ptr(z.Int64().GTE(0)),
		"DownloadDailyLimit": z.Ptr(z.Int64().GTE(0)),
		"UploadTotalLimit":   z.Ptr(z.Int64().GTE(0)),
		"DownloadTotalLimit": z.Ptr(z.Int64().GTE(0)),
		"StorageThreshold":   z.Ptr(z.Int64().GTE(0)),
		"UploadThreshold":    z.Ptr(z.Int64().GTE(0)),
		"DownloadThreshold":  z.Ptr(z.Int64().GTE(0)),
	})
}

func (r *UserQuotaConfigUpdateRequest) ToModel() (*UserQuotaConfigUpdateRequest, error) {
	return r, nil
}

// UserQuotaConfigListRequest represents query parameters for listing user quota configs
type UserQuotaConfigListRequest struct {
	PlanID *uint `query:"plan_id" json:"plan_id,omitempty"`
}

func (r *UserQuotaConfigListRequest) Schema() *z.StructSchema {
	return z.Struct(z.Shape{
		"PlanID": z.Ptr(z.UintLike[uint]()),
	})
}

func (r *UserQuotaConfigListRequest) ToModel() (*UserQuotaConfigListRequest, error) {
	return r, nil
}

// SystemStatsResponse represents system-wide quota statistics
type SystemStatsResponse struct {
	TotalUsers      int64 `json:"total_users"`
	ActiveUsers     int64 `json:"active_users"`
	TotalPlans      int64 `json:"total_plans"`
	ActivePlans     int64 `json:"total_active_plans"`
	TotalGrants     int64 `json:"total_grants"`
	ActiveGrants    int64 `json:"total_active_grants"`
	CurrentUsage    Usage `json:"current_usage"`
	TotalUsageBytes uint64 `json:"total_usage_bytes"`
}

func (r SystemStatsResponse) FromModel(_ SystemStatsResponse) error {
	return nil
}

// Usage represents aggregated usage statistics
type Usage struct {
	StorageBytes  uint64 `json:"storage_bytes"`
	UploadBytes   uint64 `json:"upload_bytes"`
	DownloadBytes uint64 `json:"download_bytes"`
}