package core

import (
	"context"
	"fmt"
	"time"

	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// Re-export WindowType from models for API consumers
type WindowType = models.WindowType

const (
	WindowTypeRolling      WindowType = models.WindowTypeRolling
	WindowTypeCalendarDay  WindowType = models.WindowTypeCalendarDay
	WindowTypeCalendarWeek WindowType = models.WindowTypeCalendarWeek
	WindowTypeCalendarMonth WindowType = models.WindowTypeCalendarMonth
	WindowTypeCalendarYear WindowType = models.WindowTypeCalendarYear
	WindowTypeLifetime     = models.WindowTypeLifetime
)

// QuotaCheckResult represents the result of a quota check
type QuotaCheckResult struct {
	Allowed     bool
	Reason      QuotaCheckReason   // "OK", "LIMIT_EXCEEDED", "ALLOWANCE_DEPLETED", "WARNING_THRESHOLD", etc.
	Details     QuotaCheckDetails
	Reservation Reservation         // Reservation if quota was reserved (optional)
}

// ReleaseReservation releases the quota reservation if one exists.
// This is a convenience method for callers to clean up failed operations.
func (q *QuotaCheckResult) ReleaseReservation() {
	if q.Reservation != nil {
		q.Reservation.Release()
	}
}

// QuotaCheckDetails provides detailed information about quota status
type QuotaCheckDetails struct {
	CurrentUsage  uint64            `json:"current_usage"`
	Limit         *uint64           `json:"limit,omitempty"`          // For HARD_LIMITS/THRESHOLD
	Allowance     *uint64           `json:"allowance,omitempty"`      // For ALLOWANCE
	AllowanceUsed *uint64           `json:"allowance_used,omitempty"` // For ALLOWANCE
	Threshold     *uint64           `json:"threshold,omitempty"`      // For THRESHOLD
	Policy        EnforcementPolicy `json:"policy"`
}

// Usage represents current usage statistics for a user
type Usage struct {
	UserID          uint                 `json:"user_id"`
	BytesUploaded   uint64               `json:"bytes_uploaded"`
	BytesDownloaded uint64               `json:"bytes_downloaded"`
	BytesStored     uint64               `json:"bytes_stored"`
	LastUpdated     time.Time            `json:"last_updated"`
	UsageByType     map[UsageType]uint64 `json:"usage_by_type"`
}

// UsagePoint represents a single data point in usage history
type UsagePoint struct {
	Date   time.Time `json:"date"`
	Bytes  uint64    `json:"bytes"`
	Type   UsageType `json:"type"`
	UserID uint      `json:"user_id"`
}

// EffectiveLimits represents the resolved limits for a user
type EffectiveLimits struct {
	UserID            uint              `json:"user_id"`
	EnforcementPolicy EnforcementPolicy `json:"enforcement_policy"`
	
	// Window-based limits
	StorageLimitConfig   *Limit `json:"storage_limit_config,omitempty"`
	UploadLimitConfig    *Limit `json:"upload_limit_config,omitempty"`
	DownloadLimitConfig  *Limit `json:"download_limit_config,omitempty"`
	
	// Thresholds (for THRESHOLD policy)
	StorageThreshold   *uint64 `json:"storage_threshold,omitempty"`
	UploadThreshold    *uint64 `json:"upload_threshold,omitempty"`
	DownloadThreshold  *uint64 `json:"download_threshold,omitempty"`
	
	QuotaPlanID *uint64 `json:"quota_plan_id,omitempty"`

	// Track whether limits were explicitly configured
	HasStorageLimitConfig   bool `json:"has_storage_limit_config"`
	HasUploadLimitConfig    bool `json:"has_upload_limit_config"`
	HasDownloadLimitConfig  bool `json:"has_download_limit_config"`
	HasStorageThresholdConfig   bool `json:"has_storage_threshold_config"`
	HasUploadThresholdConfig    bool `json:"has_upload_threshold_config"`
	HasDownloadThresholdConfig  bool `json:"has_download_threshold_config"`
}

// HasAnyLimits returns true if any limits are configured for this user
func (e EffectiveLimits) HasAnyLimits() bool {
	return e.HasStorageLimitConfig ||
		e.HasUploadLimitConfig ||
		e.HasDownloadLimitConfig ||
		e.HasStorageThresholdConfig ||
		e.HasUploadThresholdConfig ||
		e.HasDownloadThresholdConfig
}

// HasWindowLimits returns true if any window-based limits are configured
func (e EffectiveLimits) HasWindowLimits() bool {
	return e.HasStorageLimitConfig ||
		e.HasUploadLimitConfig ||
		e.HasDownloadLimitConfig
}

// GetWindowBytesRemaining returns the remaining bytes for a given limit configuration
func (e *EffectiveLimits) GetWindowBytesRemaining(ctx context.Context, usageManager UsageManager, userID uint, limitConfig *Limit, usageType UsageType) (uint64, error) {
	if limitConfig == nil || limitConfig.Window.IsNil() {
		// No effective limit - return unlimited (max uint64)
		return ^uint64(0), nil
	}

	// Get current usage for the window
	currentUsage, _, _, err := usageManager.GetUsageForWindow(
		ctx, userID, usageType, limitConfig.Window)
	if err != nil {
		return 0, fmt.Errorf("failed to get usage for window: %w", err)
	}

	// Calculate remaining bytes
	if limitConfig.Bytes <= currentUsage {
		return 0, nil
	}
	return limitConfig.Bytes - currentUsage, nil
}

// AllowanceBalance represents the current allowance balance for a user
type AllowanceBalance struct {
	StorageAllowance  uint64 `json:"storage_allowance"`
	StorageUsed       uint64 `json:"storage_used"`
	StorageRemaining  uint64 `json:"storage_remaining"`
	UploadAllowance   uint64 `json:"upload_allowance"`
	UploadUsed        uint64 `json:"upload_used"`
	UploadRemaining   uint64 `json:"upload_remaining"`
	DownloadAllowance uint64 `json:"download_allowance"`
	DownloadUsed      uint64 `json:"download_used"`
	DownloadRemaining uint64 `json:"download_remaining"`
}

// GrantConsumption represents the consumption of bytes from a specific grant
type GrantConsumption struct {
	GrantID         uint      `json:"grant_id"`
	BytesConsumed   uint64    `json:"bytes_consumed"`
	ConsumptionDate time.Time `json:"consumption_date"`
}

// Import types from models package to avoid circular imports
// These types are defined in the models package and used here

type (
	// UserQuotaConfig represents a user's quota configuration
	UserQuotaConfig = models.UserQuotaConfig

	// AllowanceGrant represents an individual allowance grant
	AllowanceGrant = models.AllowanceGrant

	// UserUsageDetail represents detailed usage records
	UserUsageDetail = models.UserUsageDetail

	// AllowanceConsumption represents allowance consumption records
	AllowanceConsumption = models.AllowanceConsumption

	// EnforcementPolicy represents the quota enforcement policy
	EnforcementPolicy = models.EnforcementPolicy

	// GrantType represents the type of resource being granted
	GrantType = models.GrantType

	// UsageType represents the type of usage
	UsageType = models.UsageType

	// QuotaCheckReason represents the reason for a quota check result
	QuotaCheckReason = models.QuotaCheckReason
)

const (
	GrantTypeStorage  = models.GrantTypeStorage
	GrantTypeUpload   = models.GrantTypeUpload
	GrantTypeDownload = models.GrantTypeDownload
)
const (
	UsageTypeUpload        = models.UsageTypeUpload
	UsageTypeDownload      = models.UsageTypeDownload
	UsageTypeStorageAdd    = models.UsageTypeStorageAdd
	UsageTypeStorageRemove = models.UsageTypeStorageRemove
)

// WindowType defines the type of time window for limit enforcement


// LimitWindow defines a time window for limit enforcement
type LimitWindow struct {
	Type      WindowType `json:"type"`                         // The window type
	Duration  *int64     `json:"duration,omitempty"`           // Duration in seconds (for ROLLING windows)
	StartDay  *int       `json:"start_day,omitempty"`          // For WEEK: Sunday=0, Monday=1, etc.
	StartHour *int       `json:"start_hour,omitempty"`         // When window starts (UTC hour, default 0)
	Timezone  *string    `json:"timezone,omitempty"`           // Timezone for calendar windows (optional, default UTC)
}

// IsNil returns true if this window configuration is effectively nil (no meaningful settings)
func (w *LimitWindow) IsNil() bool {
	return w == nil || w.Type == ""
}

// Validate returns an error if the window configuration is invalid
func (w *LimitWindow) Validate() error {
	if w == nil {
		return nil
	}

	// Validate window type
	validTypes := []WindowType{
		WindowTypeRolling,
		WindowTypeCalendarDay,
		WindowTypeCalendarWeek,
		WindowTypeCalendarMonth,
		WindowTypeCalendarYear,
		WindowTypeLifetime,
	}
	
	isValid := false
	for _, validType := range validTypes {
		if w.Type == validType {
			isValid = true
			break
		}
	}
	if !isValid {
		return fmt.Errorf("invalid window type: %s", w.Type)
	}

	// Validate window-specific requirements
	switch w.Type {
	case WindowTypeRolling:
		if w.Duration == nil || *w.Duration <= 0 {
			return fmt.Errorf("ROLLING window requires positive duration")
		}
	case WindowTypeCalendarWeek:
		if w.StartDay != nil && (*w.StartDay < 0 || *w.StartDay > 6) {
			return fmt.Errorf("WEEK window start_day must be 0-6 (Sunday=0)")
		}
		if w.StartHour != nil && (*w.StartHour < 0 || *w.StartHour > 23) {
			return fmt.Errorf("start_hour must be 0-23")
		}
	case WindowTypeCalendarDay, WindowTypeCalendarMonth, WindowTypeCalendarYear:
		if w.StartHour != nil && (*w.StartHour < 0 || *w.StartHour > 23) {
			return fmt.Errorf("start_hour must be 0-23")
		}
	}

	// Validate timezone
	if w.Timezone != nil {
		_, err := time.LoadLocation(*w.Timezone)
		if err != nil {
			return fmt.Errorf("invalid timezone: %w", err)
		}
	}

	return nil
}

// GetWindowBounds calculates the start and end time bounds for this window
// Returns (windowStart, windowEnd, error)
func (w *LimitWindow) GetWindowBounds(now time.Time) (time.Time, time.Time, error) {
	if w == nil {
		return time.Time{}, time.Time{}, fmt.Errorf("window is nil")
	}

	// Ensure we're working in UTC (or configured timezone)
	tz := time.UTC
	if w.Timezone != nil {
		loc, err := time.LoadLocation(*w.Timezone)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid timezone: %w", err)
		}
		now = now.In(loc)
		tz = loc
	}

	switch w.Type {
	case WindowTypeRolling:
		if w.Duration == nil {
			return time.Time{}, time.Time{}, fmt.Errorf("ROLLING window requires duration")
		}
		duration := time.Duration(*w.Duration) * time.Second
		startTime := now.Add(-duration)
		return startTime, now, nil

	case WindowTypeCalendarDay:
		startHour := 0
		if w.StartHour != nil {
			startHour = *w.StartHour
		}
		
		// Start of day (today at startHour)
		startTime := time.Date(now.Year(), now.Month(), now.Day(), startHour, 0, 0, 0, tz)
		
		// If we're before startHour today, use yesterday
		if now.Hour() < startHour {
			startTime = startTime.AddDate(0, 0, -1)
		}
		
		// End is startHour next day
		endTime := startTime.AddDate(0, 0, 1)
		return startTime, endTime, nil

	case WindowTypeCalendarWeek:
		startDay := time.Sunday
		if w.StartDay != nil {
			startDay = time.Weekday(*w.StartDay)
		}
		
		// Find most recent startDay
		weekday := now.Weekday()
		daysToStart := int(weekday) - int(startDay)
		if daysToStart < 0 {
			daysToStart += 7
		}
		
		// Add startHour support
		startHour := 0
		if w.StartHour != nil {
			startHour = *w.StartHour
		}
		
		startOfDay := now.Truncate(24 * time.Hour).AddDate(0, 0, -daysToStart)
		startTime := time.Date(startOfDay.Year(), startOfDay.Month(), startOfDay.Day(), startHour, 0, 0, 0, tz)
		endTime := startTime.AddDate(0, 0, 7)
		return startTime, endTime, nil

	case WindowTypeCalendarMonth:
		startHour := 0
		if w.StartHour != nil {
			startHour = *w.StartHour
		}
		
		year, month, _ := now.Date()
		startTime := time.Date(year, month, 1, startHour, 0, 0, 0, tz)
		endTime := time.Date(year, month+1, 1, startHour, 0, 0, 0, tz)
		return startTime, endTime, nil

	case WindowTypeCalendarYear:
		startHour := 0
		if w.StartHour != nil {
			startHour = *w.StartHour
		}
		
		year, _, _ := now.Date()
		startTime := time.Date(year, 1, 1, startHour, 0, 0, 0, tz)
		endTime := time.Date(year+1, 1, 1, startHour, 0, 0, 0, tz)
		return startTime, endTime, nil

	case WindowTypeLifetime:
		startTime := time.Time{}  // Zero time = beginning
		endTime := now
		return startTime, endTime, nil

	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported window type: %s", w.Type)
	}
}

// Limit represents a byte limit with an associated time window
type Limit struct {
	Bytes         uint64     `json:"bytes"`            // Number of bytes allowed in this window
	Window        LimitWindow `json:"window"`          // Time window for this limit
	Priority      int        `json:"priority"`         // Priority (higher checked first)
}

type QuotaPlan = models.QuotaPlan
type QuotaConfig = config.QuotaConfig

// ThresholdCheckResult represents the result of a threshold evaluation
type ThresholdCheckResult struct {
	ShouldWarn     bool
	WithinLimit    bool
	CurrentUsage   uint64
	Threshold      *uint64
	Limit          *uint64
	DecisionReason models.QuotaCheckReason
}

// EvaluateThreshold evaluates threshold logic in a simplified way
func EvaluateThreshold(currentUsage, requestedBytes, threshold, limit uint64) ThresholdCheckResult {
	if threshold == 0 {
		// Threshold is 0, which means always warn
		// Check if within limit using overflow-safe subtraction
		withinLimit := currentUsage <= limit && requestedBytes <= limit-currentUsage

		return ThresholdCheckResult{
			ShouldWarn:     true,
			WithinLimit:    withinLimit,
			CurrentUsage:   currentUsage,
			Threshold:      &threshold,
			Limit:          &limit,
			DecisionReason: models.QuotaCheckReasonWarningThreshold,
		}
	}

	// Check if within limit using overflow-safe subtraction
	withinLimit := currentUsage <= limit && requestedBytes <= limit-currentUsage

	// Check if would exceed threshold using overflow-safe logic
	wouldExceedThreshold := false
	if threshold >= currentUsage {
		wouldExceedThreshold = requestedBytes > threshold-currentUsage
	} else {
		// If current usage already exceeds threshold, then any additional usage would exceed it
		wouldExceedThreshold = true
	}

	// Check if would cross threshold using overflow-safe logic
	wouldCrossThreshold := false
	if threshold > currentUsage {
		wouldCrossThreshold = requestedBytes >= threshold-currentUsage
	}
	// If threshold <= currentUsage, we're already at or past the threshold, so no crossing occurs

	shouldWarn := (wouldExceedThreshold || wouldCrossThreshold) && withinLimit

	var reason models.QuotaCheckReason
	if shouldWarn {
		reason = models.QuotaCheckReasonWarningThreshold
	} else {
		reason = models.QuotaCheckReasonOK
	}

	// Only calculate newUsage if we know it won't overflow (when withinLimit is true)
	var newUsage uint64
	if withinLimit {
		newUsage = currentUsage + requestedBytes
	} else {
		newUsage = currentUsage // Keep current usage when overflow would occur
	}

	return ThresholdCheckResult{
		ShouldWarn:     shouldWarn,
		WithinLimit:    withinLimit,
		CurrentUsage:   newUsage,
		Threshold:      &threshold,
		Limit:          &limit,
		DecisionReason: reason,
	}
}

// SystemStats represents system-wide quota statistics
type SystemStats struct {
	TotalUsers      int64  `json:"total_users"`
	ActiveUsers     int64  `json:"active_users"`
	TotalPlans      int64  `json:"total_plans"`
	ActivePlans     int64  `json:"active_plans"`
	TotalGrants     int64  `json:"total_grants"`
	ActiveGrants    int64  `json:"active_grants"`
	CurrentUsage    Usage  `json:"current_usage"`
	TotalUsageBytes uint64 `json:"total_usage_bytes"`
}

// UserQuotaConfigUpdate represents the fields that can be updated for a user's quota config
type UserQuotaConfigUpdate struct {
	EnforcementPolicy  *EnforcementPolicy `json:"enforcement_policy,omitempty"`
	QuotaPlanID        *uint64            `json:"quota_plan_id,omitempty"`
	// Window configuration
	WindowType      *string  `json:"window_type,omitempty"`
	WindowDuration  *int64   `json:"window_duration,omitempty"`
	WindowStartHour *int     `json:"window_start_hour,omitempty"`
	WindowTimezone  *string  `json:"window_timezone,omitempty"`
	// Byte limits
	StorageLimitBytes   *uint64 `json:"storage_limit_bytes,omitempty"`
	UploadLimitBytes    *uint64 `json:"upload_limit_bytes,omitempty"`
	DownloadLimitBytes  *uint64 `json:"download_limit_bytes,omitempty"`
	StorageThreshold    *int64  `json:"storage_threshold,omitempty"`
	UploadThreshold     *int64  `json:"upload_threshold,omitempty"`
	DownloadThreshold   *int64  `json:"download_threshold,omitempty"`
}

