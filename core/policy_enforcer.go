package core

import (
	"context"
	"time"
)

// PolicyEnforcer interface - each enforcement policy implements this
type PolicyEnforcer interface {
	// Check if operation is allowed under this policy
	CheckUploadQuota(ctx context.Context, config *UserQuotaConfig, requestedBytes uint64) (QuotaCheckResult, error)
	CheckDownloadQuota(ctx context.Context, config *UserQuotaConfig, requestedBytes uint64) (QuotaCheckResult, error)
	CheckStorageQuota(ctx context.Context, config *UserQuotaConfig, requestedBytes uint64) (QuotaCheckResult, error)

	// Record usage (implementation varies by policy)
	RecordUpload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error
	RecordDownload(ctx context.Context, userID, uploadID uint, bytes uint64, ip string) error
	RecordStorageChange(ctx context.Context, userID, uploadID uint, bytes int64, ip string) error

	// Policy-specific operations
	GetDetailedUsage(ctx context.Context, userID uint, start, end time.Time) ([]*UserUsageDetail, error)
	GetCurrentUsage(ctx context.Context, userID uint) (*Usage, error)
	GetUsageHistory(ctx context.Context, userID uint, period int, usageType UsageType) ([]*UsagePoint, error)
}
