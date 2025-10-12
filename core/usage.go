package core

import (
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// UsageManager defines the interface for usage recording and management
type UsageManager interface {
	// RecordUpload records upload usage for a user
	RecordUpload(userID, uploadID uint, bytes uint64, ip string) error

	// RecordDownload records download usage for a user
	RecordDownload(userID, uploadID uint, bytes uint64, ip string) error

	// RecordStorageChange records storage usage changes for a user
	RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error

	// RecordUserUsageDetail records a detailed usage record
	RecordUserUsageDetail(detail *UserUsageDetail) error

	// UpdateDailyUsage updates the daily aggregated usage for a user
	UpdateDailyUsage(userID uint, usageType UsageType, bytes int64) error

	// GetCurrentUsage returns the current usage for a user
	GetCurrentUsage(userID uint) (*Usage, error)

	// GetUsageHistory returns usage history for a user
	GetUsageHistory(userID uint, period int, usageType UsageType) ([]*UsagePoint, error)

	// GetDetailedUsage returns detailed usage records for a user within a time range
	GetDetailedUsage(userID uint, start, end time.Time) ([]*UserUsageDetail, error)

	// GetTotalBytesByType returns the total bytes consumed for a specific usage type across all time
	GetTotalBytesByType(userID uint, usageType UsageType) (uint64, error)

	// GetUserQuotaConfig returns the quota configuration for a user
	GetUserQuotaConfig(userID uint) (*models.UserQuotaConfig, error)
}

// UsageAggregator defines the interface for aggregating usage data
type UsageAggregator interface {
	GetAggregatedUsageByType(userID uint, usageType UsageType) (uint64, error)
}
