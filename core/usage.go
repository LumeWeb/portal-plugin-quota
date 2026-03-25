package core

import (
	"context"
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"gorm.io/gorm"
)

// UsageManager defines the interface for usage recording and management
type UsageManager interface {
	// RecordUpload records upload usage for a user
	RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error

	// RecordDownload records download usage for a user
	RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error

	// RecordStorageChange records storage usage changes for a user
	// bytes represents the change in storage:
	//   positive values indicate storage added (file uploaded)
	//   negative values indicate storage removed (file deleted)
	RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error

	// RecordUserUsageDetail records a detailed usage record
	// If tx is provided, it will be used instead of creating a new transaction
	RecordUserUsageDetail(ctx context.Context, detail *UserUsageDetail, tx *gorm.DB) error

	// UpdateDailyUsage updates the daily aggregated usage for a user
	UpdateDailyUsage(ctx context.Context, userID uint, usageType UsageType, bytes int64) error

	// GetCurrentUsage returns the current usage for a user
	GetCurrentUsage(ctx context.Context, userID uint) (*Usage, error)

	// GetUsageHistory returns usage history for a user
	// period is in days (24-hour periods from now)
	GetUsageHistory(ctx context.Context, userID uint, period int, usageType UsageType) ([]*UsagePoint, error)

	// GetUsageHistoryDateRange returns usage history for a user within a specific date range
	GetUsageHistoryDateRange(ctx context.Context, userID uint, usageType UsageType, startTime, endTime time.Time) ([]*UsagePoint, error)

	// GetDetailedUsage returns detailed usage records for a user within a time range
	GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*UserUsageDetail, error)

	// GetTotalBytesByType returns the total bytes consumed for a specific usage type across all time
	GetTotalBytesByType(ctx context.Context, userID uint, usageType UsageType) (uint64, error)

	// GetUserQuotaConfig returns the quota configuration for a user
	GetUserQuotaConfig(ctx context.Context, userID uint) (*models.UserQuotaConfig, error)

	// RecordUsageAndConsume records a usage detail and consumes from grants in a single transaction
	// This is used by allowance policy enforcers to atomically record usage and consume allowance
	RecordUsageAndConsume(ctx context.Context, detail *models.UserUsageDetail, grantType models.GrantType, bytes uint64) error

	// GetAggregatedUsageByType returns the aggregated usage for a specific user and usage type
	GetAggregatedUsageByType(ctx context.Context, userID uint, usageType UsageType) (uint64, error)
}

// UsageAggregator defines the interface for aggregating usage data
type UsageAggregator interface {
	GetAggregatedUsageByType(ctx context.Context, userID uint, usageType UsageType) (uint64, error)
}
