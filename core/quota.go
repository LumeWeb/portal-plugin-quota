package core

import (
	"context"
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.lumeweb.com/queryutil"
)

const QUOTA_SERVICE = "quota"

// QuotaService is the interface for quota management functionality
type QuotaService interface {
	core.Service
	core.Configurable
	// Usage Recording
	RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error
	RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error
	RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error

	// Quota Checking (returns allowed + reason)
	// Options can be provided to configure reservation behavior
	CheckUploadQuota(ctx context.Context, userID uint, requestedBytes uint64, opts ...CheckOption) (QuotaCheckResult, error)
	CheckDownloadQuota(ctx context.Context, userID uint, requestedBytes uint64, opts ...CheckOption) (QuotaCheckResult, error)
	CheckStorageQuota(ctx context.Context, userID uint, requestedBytes uint64, opts ...CheckOption) (QuotaCheckResult, error)

	// Usage Analytics
	GetCurrentUsage(ctx context.Context, userID uint) (*Usage, error)
	GetUsageHistory(ctx context.Context, userID uint, period int, usageType UsageType) ([]*UsagePoint, error)
	GetUsageHistoryDateRange(ctx context.Context, userID uint, usageType UsageType, startTime, endTime time.Time) ([]*UsagePoint, error)
	GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*UserUsageDetail, error)
	GetTodayUsage(ctx context.Context, userID uint) (*Usage, error)

	// Configuration Management
	SetQuotaConfig(ctx context.Context, userID uint, config *UserQuotaConfig) error
	GetQuotaConfig(ctx context.Context, userID uint) (*UserQuotaConfig, error)

	// Quota Plan Management
	CreateQuotaPlan(ctx context.Context, plan *models.QuotaPlan) error
	UpdateQuotaPlan(ctx context.Context, planID uint, plan *models.QuotaPlan) error
	DeleteQuotaPlan(ctx context.Context, planID uint) error
	GetQuotaPlan(ctx context.Context, planID uint) (*models.QuotaPlan, error)
	ListQuotaPlans(ctx context.Context, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*models.QuotaPlan, int64, error)
	SetDefaultQuotaPlan(ctx context.Context, planID uint) error
	GetDefaultQuotaPlan(ctx context.Context) (*models.QuotaPlan, error)
	AssignUserToPlan(ctx context.Context, userID uint, planID uint) error
	RemoveUserFromPlan(ctx context.Context, userID uint) error

	// User Quota Config Management
	ListUserQuotaConfigs(ctx context.Context, filters []queryutil.CrudFilter, sorts []queryutil.Sort, pagination queryutil.Pagination) ([]*UserQuotaConfig, int64, error)
	UpdateUserQuotaConfig(ctx context.Context, userID uint, update *UserQuotaConfigUpdate) (*UserQuotaConfig, error)
	ResetUserQuotaPlan(ctx context.Context, userID uint) error

	// Allowance Management (for ALLOWANCE policy)
	AddAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error
	AddBonusAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error
	AddPromoAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error
	AddSubscriptionAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error
	AddPAYGAddonAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error
	DeductAllowance(ctx context.Context, userID uint, storage, upload, download uint64) error
	GetAllowanceBalance(ctx context.Context, userID uint) (*AllowanceBalance, error)
	ResetAllowance(ctx context.Context, userID uint) error

	// System Management
	GetSystemStats(ctx context.Context) (*SystemStats, error)
	Reconcile(ctx context.Context) error
	CleanupOldRecords(ctx context.Context, retentionDays int) (int64, error)

	// Usage Manager getter
	GetUsageManager() UsageManager

	// Grant Manager getter
	GetGrantManager() GrantManager

	// Reservation Manager getter
	GetReservationManager() ReservationManager

	// Quota Plan Manager getter
	GetQuotaPlanManager() QuotaPlanManager

	// Config Manager getter
	GetConfigManager() ConfigManager

	// CID-based quota availability check
	// Checks if there are any users with sufficient quota to handle an operation on content identified by CID
	CheckCIDGroupQuotaAvailability(ctx context.Context, cid core.StorageHash, requiredBytes uint64, usageType UsageType) (bool, error)
}

// QuotaPlanManager abstracts database operations related to quota plans
type QuotaPlanManager interface {
	GetQuotaPlanByID(ctx context.Context, id uint64) (*models.QuotaPlan, error)
	GetQuotaPlanByName(ctx context.Context, name string) (*models.QuotaPlan, error)
	GetDefaultQuotaPlan(ctx context.Context) (*models.QuotaPlan, error)
}
