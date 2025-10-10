package models

import (
	"errors"
)

// Predefined validation errors
var (
	ErrInvalidUserID              = errors.New("user_id must be greater than 0")
	ErrInvalidDate                = errors.New("date must not be zero")
	ErrInvalidUploadID            = errors.New("upload_id must be greater than 0")
	ErrInvalidBytes               = errors.New("bytes must be greater than 0")
	ErrInvalidIP                  = errors.New("ip must not be empty")
	ErrInvalidTimestamp           = errors.New("timestamp must not be zero")
	ErrInvalidSharedWith          = errors.New("shared_with must be between 0 and 1000")
	ErrInvalidGrantID             = errors.New("grant_id must be greater than 0")
	ErrInvalidUsageDetailID       = errors.New("usage_detail_id must be greater than 0")
	ErrInvalidBytesConsumed       = errors.New("bytes_consumed must be greater than 0")
	ErrInvalidConsumptionDate     = errors.New("consumption_date must not be zero")
	ErrInvalidPlanName            = errors.New("name must not be empty")
	ErrInvalidStorageThreshold    = errors.New("storage_threshold cannot be greater than storage_limit")
	ErrInvalidUploadThreshold     = errors.New("upload_threshold cannot be greater than upload_daily_limit")
	ErrInvalidDownloadThreshold   = errors.New("download_threshold cannot be greater than download_daily_limit")
	ErrCannotDeleteDefaultPlan    = errors.New("cannot delete default quota plan")
	ErrCannotDeleteReferencedPlan = errors.New("cannot delete quota plan that is referenced by user configurations")
	ErrInvalidBytesUsed           = errors.New("bytes_used cannot be greater than bytes")
	ErrInvalidBytesRemaining      = errors.New("bytes_remaining must equal bytes - bytes_used")
	ErrInvalidExpiryDateOnCreate  = errors.New("expiry_date must be in the future")
	ErrInvalidStorageLimit        = errors.New("storage_limit must be greater than or equal to 0")
	ErrInvalidUploadDailyLimit    = errors.New("upload_daily_limit must be greater than or equal to 0")
	ErrInvalidDownloadDailyLimit  = errors.New("download_daily_limit must be greater than or equal to 0")
	ErrInvalidUploadTotalLimit    = errors.New("upload_total_limit must be greater than or equal to 0")
	ErrInvalidDownloadTotalLimit  = errors.New("download_total_limit must be greater than or equal to 0")
)
