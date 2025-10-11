package core

import (
	"time"
)

// PolicyEnforcer interface - each enforcement policy implements this
type PolicyEnforcer interface {
	// Check if operation is allowed under this policy
	CheckUploadQuota(config *UserQuotaConfig, requestedBytes uint64) (QuotaCheckResult, error)
	CheckDownloadQuota(config *UserQuotaConfig, requestedBytes uint64) (QuotaCheckResult, error)
	CheckStorageQuota(config *UserQuotaConfig, requestedBytes uint64) (QuotaCheckResult, error)

	// Record usage (implementation varies by policy)
	RecordUpload(userID, uploadID uint, bytes uint64, ip string) error
	RecordDownload(userID, uploadID uint, bytes uint64, ip string) error
	RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error

	// Policy-specific operations
	GetDetailedUsage(userID uint, start, end time.Time) ([]*UserUsageDetail, error)
	GetCurrentUsage(userID uint) (*Usage, error)
	GetUsageHistory(userID uint, period int, usageType UsageType) ([]*UsagePoint, error)
}
