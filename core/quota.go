package core

import (
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

const QUOTA_SERVICE = "quota"

// QuotaService is the interface for quota management functionality
type QuotaService interface {
	core.Service
	// Usage Recording
	RecordUpload(userID, uploadID uint, bytes uint64, ip string) error
	RecordDownload(userID, uploadID uint, bytes uint64, ip string) error
	RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error

	// Quota Checking (returns allowed + reason)
	CheckUploadQuota(userID uint, requestedBytes uint64) (QuotaCheckResult, error)
	CheckDownloadQuota(userID uint, requestedBytes uint64) (QuotaCheckResult, error)
	CheckStorageQuota(userID uint, requestedBytes uint64) (QuotaCheckResult, error)

	// Usage Analytics
	GetCurrentUsage(userID uint) (*Usage, error)
	GetUsageHistory(userID uint, period int, usageType UsageType) ([]*UsagePoint, error)
	GetDetailedUsage(userID uint, start, end time.Time) ([]*UserUsageDetail, error)
	GetTodayUsage(userID uint) (*Usage, error)

	// Configuration Management
	SetQuotaConfig(userID uint, config *UserQuotaConfig) error
	GetQuotaConfig(userID uint) (*UserQuotaConfig, error)

	// Quota Plan Management
	CreateQuotaPlan(plan *models.QuotaPlan) error
	UpdateQuotaPlan(planID uint, plan *models.QuotaPlan) error
	DeleteQuotaPlan(planID uint) error
	GetQuotaPlan(planID uint) (*models.QuotaPlan, error)
	ListQuotaPlans() ([]*models.QuotaPlan, error)
	SetDefaultQuotaPlan(planID uint) error
	GetDefaultQuotaPlan() (*models.QuotaPlan, error)
	AssignUserToPlan(userID uint, planID uint) error

	// Allowance Management (for ALLOWANCE policy)
	AddAllowance(userID uint, storage, upload, download uint64) error
	DeductAllowance(userID uint, storage, upload, download uint64) error
	GetAllowanceBalance(userID uint) (*AllowanceBalance, error)
	ResetAllowance(userID uint) error

	// System Management
	Reconcile() error
	CleanupOldRecords(retentionDays int) error

	// Usage Manager getter
	GetUsageManager() UsageManager

	// Grant Manager getter
	GetGrantManager() GrantManager

	// Usage Aggregator getter
	GetUsageAggregator() UsageAggregator

	// Quota Plan Manager getter
	GetQuotaPlanManager() QuotaPlanManager

	// Config Manager getter
	GetConfigManager() ConfigManager
}

// QuotaPlanManager abstracts database operations related to quota plans
type QuotaPlanManager interface {
	GetQuotaPlanByID(id uint64) (*models.QuotaPlan, error)
	GetDefaultQuotaPlan() (*models.QuotaPlan, error)
}
