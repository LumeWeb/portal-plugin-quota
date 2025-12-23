package managers

import (
	"fmt"
	"math"
	"time"

	"github.com/samber/lo"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	portalCore "go.lumeweb.com/portal/core"
	portalModels "go.lumeweb.com/portal/db/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UsageManager handles centralized usage recording and shared usage calculations
// Implements core.UsageManager interface
type UsageManager struct {
	ctx           portalCore.Context
	db            *gorm.DB
	logger        *portalCore.Logger
	config        *config.QuotaConfig
	pinService    portalCore.PinService
	uploadService portalCore.UploadService
}

// NewUsageManager creates a new usage manager
func NewUsageManager(ctx portalCore.Context) *UsageManager {
	quotaConfig := portalCore.GetServiceConfig[*config.QuotaConfig](ctx, pluginCore.QUOTA_SERVICE)

	return &UsageManager{
		ctx:           ctx,
		db:            ctx.DB(),
		logger:        ctx.NamedLogger("quota.UsageManager"),
		config:        quotaConfig,
		pinService:    portalCore.GetService[portalCore.PinService](ctx, portalCore.PIN_SERVICE),
		uploadService: portalCore.GetService[portalCore.UploadService](ctx, portalCore.UPLOAD_SERVICE),
	}
}

// RecordUpload records upload usage for a user
// Implements core.UsageManager.RecordUpload
func (um *UsageManager) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := um.validateUserID(userID); err != nil {
		return err
	}
	if err := um.validateBytes(bytes); err != nil {
		return err
	}

	// For uploads, usage is not shared - only the uploading user is charged
	detail := &pluginModels.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       pluginModels.UsageTypeUpload,
		Bytes:      bytes,
		IP:         ip,
		SharedWith: 1, // Only the uploader
		Timestamp:  time.Now().UTC(),
	}

	if err := um.RecordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record upload usage detail: %w", err)
	}

	// Update daily usage
	if err := um.UpdateDailyUsage(userID, pluginModels.UsageTypeUpload, int64(bytes)); err != nil {
		return fmt.Errorf("failed to update daily upload usage: %w", err)
	}

	return nil
}

// GetUserQuotaConfig returns the quota configuration for a user
func (um *UsageManager) GetUserQuotaConfig(userID uint) (*pluginModels.UserQuotaConfig, error) {
	if err := um.validateUserID(userID); err != nil {
		return nil, err
	}

	// Use FirstOrCreate to prevent race conditions when multiple goroutines
	// try to create the same user config simultaneously
	cfg := pluginModels.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
	}

	result := um.db.Where("user_id = ?", userID).FirstOrCreate(&cfg)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get or create user quota config: %w", result.Error)
	}

	return &cfg, nil
}

// GetAggregatedUsageByType returns the aggregated usage for a specific user and usage type
func (um *UsageManager) GetAggregatedUsageByType(userID uint, usageType pluginCore.UsageType) (uint64, error) {
	if err := um.validateUserID(userID); err != nil {
		return 0, err
	}

	var total uint64
	err := um.db.Model(&pluginModels.UserUsageDetail{}).
		Where("user_id = ? AND type = ?", userID, usageType).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&total).Error

	if err != nil {
		return 0, fmt.Errorf("failed to aggregate usage by type: %w", err)
	}

	return total, nil
}

// GetUsageHistory returns usage history for a user
func (um *UsageManager) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	if err := um.validateUserID(userID); err != nil {
		return nil, err
	}

	if period <= 0 {
		return nil, fmt.Errorf("period must be positive")
	}

	endDate := time.Now().UTC()
	startDate := endDate.AddDate(0, 0, -period)

	var usageDetails []pluginModels.UserUsageDetail
	err := um.db.Where("user_id = ? AND type = ? AND timestamp BETWEEN ? AND ?",
		userID, pluginModels.UsageType(usageType), startDate, endDate).
		Order("timestamp ASC").
		Find(&usageDetails).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get usage history: %w", err)
	}

	var usagePoints []*pluginCore.UsagePoint
	for _, detail := range usageDetails {
		usagePoints = append(usagePoints, &pluginCore.UsagePoint{
			Date:   detail.Timestamp,
			Bytes:  detail.Bytes,
			Type:   pluginCore.UsageType(usageType),
			UserID: userID,
		})
	}

	return usagePoints, nil
}

// GetDetailedUsage returns detailed usage records for a user within a time range
func (um *UsageManager) GetDetailedUsage(userID uint, start, end time.Time) ([]*pluginCore.UserUsageDetail, error) {
	if err := um.validateUserID(userID); err != nil {
		return nil, err
	}

	if start.After(end) {
		return nil, fmt.Errorf("start time must be before end time")
	}

	var usageDetails []pluginModels.UserUsageDetail
	err := um.db.Where("user_id = ? AND timestamp BETWEEN ? AND ?", userID, start, end).
		Order("timestamp ASC").
		Find(&usageDetails).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get detailed usage: %w", err)
	}

	var coreUsageDetails []*pluginCore.UserUsageDetail
	for _, detail := range usageDetails {
		coreUsageDetails = append(coreUsageDetails, &pluginCore.UserUsageDetail{
			Model: gorm.Model{
				ID:        detail.ID,
				CreatedAt: detail.CreatedAt,
				UpdatedAt: detail.UpdatedAt,
				DeletedAt: detail.DeletedAt,
			},
			UserID:     detail.UserID,
			UploadID:   detail.UploadID,
			Type:       pluginCore.UsageType(detail.Type),
			Bytes:      detail.Bytes,
			IP:         detail.IP,
			SharedWith: detail.SharedWith,
			Timestamp:  detail.Timestamp,
		})
	}

	return coreUsageDetails, nil
}

// GetTotalBytesByType retrieves the total bytes consumed for a specific usage type across all time
func (um *UsageManager) GetTotalBytesByType(userID uint, usageType pluginCore.UsageType) (uint64, error) {
	if err := um.validateUserID(userID); err != nil {
		return 0, err
	}

	var totalBytes uint64
	err := um.db.Model(&pluginModels.UserUsageDetail{}).
		Where("user_id = ? AND type = ?", userID, pluginModels.UsageType(usageType)).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&totalBytes).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get total bytes for type %s: %w", usageType, err)
	}

	return totalBytes, nil
}

// RecordDownload records download usage for a user
// Implements core.UsageManager.RecordDownload
//
// If userID == 0, the download is treated as anonymous and bytes are
// distributed among all users who have pinned the upload (shared usage).
// If userID > 0, the user pays the full download bytes.
func (um *UsageManager) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := um.validateBytes(bytes); err != nil {
		return err
	}

	// Anonymous download (userID == 0) - distribute among pinners
	if userID == 0 {
		return um.recordAnonymousDownload(uploadID, bytes, ip)
	}

	if err := um.validateUserID(userID); err != nil {
		return err
	}

	// Authenticated user pays full bytes - their download, their responsibility
	detail := &pluginModels.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       pluginModels.UsageTypeDownload,
		Bytes:      bytes,
		IP:         ip,
		SharedWith: 1, // Not shared - user pays in full
		Timestamp:  time.Now().UTC(),
	}

	if err := um.RecordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record download usage detail: %w", err)
	}

	// Update daily usage
	if err := um.UpdateDailyUsage(userID, pluginModels.UsageTypeDownload, int64(bytes)); err != nil {
		return fmt.Errorf("failed to update daily download usage: %w", err)
	}

	return nil
}

// recordAnonymousDownload handles anonymous downloads by recording shared usage
// for all users who have pinned the upload.
//
// Shared usage is required for anonymous downloads because:
// 1. Anonymous users don't exist in the quota system (no quota limits)
// 2. Users who pin content make it accessible and share the responsibility
// 3. Fair attribution distributes the cost among those who made content available
//
// When EnableSharedUsage = false, anonymous downloads are skipped (no quota tracking).
func (um *UsageManager) recordAnonymousDownload(uploadID uint, bytes uint64, ip string) error {
	// If shared usage is disabled, skip recording (no user to charge)
	// Anonymous downloads become effectively free from a quota perspective
	if um.config == nil || !um.config.EnableSharedUsage {
		um.logger.Debug("Anonymous download not recorded (shared usage disabled)",
			zap.Uint("uploadID", uploadID), zap.Uint64("bytes", bytes))
		return nil
	}

	// Get all users who have pinned this upload
	pinnedUsers, err := um.getPinnedUsersForUpload(uploadID)
	if err != nil {
		return fmt.Errorf("failed to get pinned users: %w", err)
	}

	if len(pinnedUsers) == 0 {
		um.logger.Debug("No pinned users for upload, skipping anonymous download recording",
			zap.Uint("uploadID", uploadID))
		return nil
	}

	// Calculate shared bytes per user
	userCount := uint(len(pinnedUsers))
	sharedBytes := um.calculateSharedBytes(bytes, userCount, um.config.SharedUsagePrecision)

	// Record usage for each pinned user
	for _, userID := range pinnedUsers {
		detail := &pluginModels.UserUsageDetail{
			UserID:     userID,
			UploadID:   uploadID,
			Type:       pluginModels.UsageTypeDownload,
			Bytes:      sharedBytes,
			IP:         ip,
			SharedWith: userCount,
			Timestamp:  time.Now().UTC(),
		}

		if err := um.RecordUserUsageDetail(detail); err != nil {
			um.logger.Warn("Failed to record anonymous download usage for user",
				zap.Uint("userID", userID),
				zap.Uint("uploadID", uploadID),
				zap.Error(err))
			continue
		}

		// Update daily usage
		if err := um.UpdateDailyUsage(userID, pluginModels.UsageTypeDownload, int64(sharedBytes)); err != nil {
			um.logger.Warn("Failed to update daily download usage for user",
				zap.Uint("userID", userID),
				zap.Uint("uploadID", uploadID),
				zap.Error(err))
		}
	}

	return nil
}

// getPinnedUsersForUpload returns all user IDs that have pinned the given upload
func (um *UsageManager) getPinnedUsersForUpload(uploadID uint) ([]uint, error) {
	if um.pinService == nil {
		return nil, fmt.Errorf("pin service not available")
	}

	pins, err := um.pinService.GetPinsByUploadID(um.ctx, uploadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pins for upload: %w", err)
	}

	return lo.Uniq(lo.Map(pins, func(pin *portalModels.Pin, _ int) uint {
		return pin.UserID
	})), nil
}

// RecordStorageChange records storage usage changes for a user
// Implements core.UsageManager.RecordStorageChange
func (um *UsageManager) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := um.validateUserID(userID); err != nil {
		return err
	}
	if bytes == 0 {
		return pluginModels.ErrInvalidBytes
	}

	// Determine usage type and byte value for recording
	var usageType pluginModels.UsageType
	var recordBytes uint64

	if bytes < 0 {
		usageType = pluginModels.UsageTypeStorageRemove
		// Handle math.MinInt64 case to prevent overflow when converting to uint64
		if bytes == math.MinInt64 {
			recordBytes = math.MaxInt64
		} else {
			recordBytes = uint64(-bytes)
		}
	} else {
		usageType = pluginModels.UsageTypeStorageAdd
		recordBytes = uint64(bytes)
	}

	// For storage changes, usage is never shared - each user is charged the full amount
	sharedWith := uint(1)
	sharedBytes := recordBytes

	detail := &pluginModels.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       usageType,
		Bytes:      sharedBytes,
		IP:         ip,
		SharedWith: sharedWith,
		Timestamp:  time.Now().UTC(),
	}

	if err := um.RecordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record storage usage detail: %w", err)
	}

	// Update daily usage with the correct usage type and byte value
	// For storage operations, we need to handle the signed bytes properly
	var dailyUsageBytes int64
	if usageType == pluginModels.UsageTypeStorageRemove {
		dailyUsageBytes = -int64(sharedBytes)
	} else {
		dailyUsageBytes = int64(sharedBytes)
	}

	if err := um.UpdateDailyUsage(userID, usageType, dailyUsageBytes); err != nil {
		return fmt.Errorf("failed to update daily storage usage: %w", err)
	}

	return nil
}

// calculateSharedBytes calculates the bytes per user with configurable precision
func (um *UsageManager) calculateSharedBytes(totalBytes uint64, userCount uint, precision int) uint64 {
	if userCount == 0 {
		return 0
	}

	// Calculate base shared bytes
	baseFloat := float64(totalBytes) / float64(userCount)

	// Precision semantics:
	//   0  -> floor to whole byte
	//   >0 -> ceil to whole byte
	var roundedBytes uint64
	if precision > 0 {
		roundedBytes = uint64(math.Ceil(baseFloat))
	} else {
		roundedBytes = uint64(baseFloat) // floor
	}

	// Ensure minimum of 1 byte when using precision and totalBytes > 0 and userCount > 0
	if precision > 0 && roundedBytes == 0 && totalBytes > 0 && userCount > 0 {
		roundedBytes = 1
	}

	return roundedBytes
}

// RecordUserUsageDetail records a detailed usage record
func (um *UsageManager) RecordUserUsageDetail(detail *pluginModels.UserUsageDetail) error {
	return um.db.Create(detail).Error
}

// UpdateDailyUsage updates the daily aggregated usage for a user
func (um *UsageManager) UpdateDailyUsage(userID uint, usageType pluginModels.UsageType, bytes int64) error {
	if err := um.validateUserID(userID); err != nil {
		return err
	}

	// Use the new upsert method that properly handles concurrent access
	if err := pluginModels.UpsertDailyUsage(um.db, userID, usageType, bytes); err != nil {
		return fmt.Errorf("failed to update daily usage: %w", err)
	}

	return nil
}

// Validation methods

// validateUserID validates that a user ID is valid
func (um *UsageManager) validateUserID(userID uint) error {
	if userID == 0 {
		return pluginModels.ErrInvalidUserID
	}
	return nil
}

// isUniqueConstraintViolation checks if the error is a unique constraint violation
func (um *UsageManager) isUniqueConstraintViolation(err error) bool {
	if err == nil {
		return false
	}

	// Check for SQLite unique constraint error
	if sqliteErr, ok := err.(interface{ SQLiteError() error }); ok {
		if sqliteErr.SQLiteError() != nil {
			// SQLite unique constraint error code is 2067
			// This is a simplified check - in practice you might need to check the specific error code
			return true
		}
	}

	// Check for generic database unique constraint errors
	// This varies by database driver but common patterns include:
	errStr := err.Error()
	isDBConstraint := (errStr == "UNIQUE constraint failed") ||
		(errStr == "duplicate key value violates unique constraint") ||
		(errStr == "PRIMARY KEY must be unique")

	// Check for GORM model validation errors
	isModelValidation := (errStr == "user_id must be greater than 0") ||
		(errStr == "date must be greater than 0")

	return isDBConstraint || isModelValidation
}

// GetCurrentUsage retrieves the current daily usage for a user
func (um *UsageManager) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	if err := um.validateUserID(userID); err != nil {
		return nil, err
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	tomorrow := today.Add(24 * time.Hour)

	// Get uploaded bytes for today
	var bytesUploaded uint64
	err := um.db.Model(&pluginModels.UserUsageDetail{}).
		Where("user_id = ? AND type = ? AND timestamp >= ? AND timestamp < ?", userID, pluginModels.UsageTypeUpload, today, tomorrow).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&bytesUploaded).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get uploaded bytes: %w", err)
	}

	// Get downloaded bytes for today
	var bytesDownloaded uint64
	err = um.db.Model(&pluginModels.UserUsageDetail{}).
		Where("user_id = ? AND type = ? AND timestamp >= ? AND timestamp < ?", userID, pluginModels.UsageTypeDownload, today, tomorrow).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&bytesDownloaded).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get downloaded bytes: %w", err)
	}

	// Get storage add bytes for today
	var bytesStoredAdd uint64
	err = um.db.Model(&pluginModels.UserUsageDetail{}).
		Where("user_id = ? AND type = ? AND timestamp >= ? AND timestamp < ?", userID, pluginModels.UsageTypeStorageAdd, today, tomorrow).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&bytesStoredAdd).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get storage add bytes: %w", err)
	}

	// Get storage remove bytes for today
	var bytesStoredRemove uint64
	err = um.db.Model(&pluginModels.UserUsageDetail{}).
		Where("user_id = ? AND type = ? AND timestamp >= ? AND timestamp < ?", userID, pluginModels.UsageTypeStorageRemove, today, tomorrow).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&bytesStoredRemove).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get storage remove bytes: %w", err)
	}

	// Calculate net stored bytes
	var bytesStored uint64
	if bytesStoredAdd > bytesStoredRemove {
		bytesStored = bytesStoredAdd - bytesStoredRemove
	}

	// Get aggregated usage by type
	usageByType := make(map[pluginCore.UsageType]uint64)
	usageTypes := []pluginModels.UsageType{
		pluginModels.UsageTypeUpload,
		pluginModels.UsageTypeDownload,
		pluginModels.UsageTypeStorageAdd,
		pluginModels.UsageTypeStorageRemove,
	}

	for _, usageType := range usageTypes {
		bytes, err := um.GetAggregatedUsageByType(userID, pluginCore.UsageType(usageType))
		if err != nil {
			return nil, fmt.Errorf("failed to get aggregated usage: %w", err)
		}
		usageByType[pluginCore.UsageType(usageType)] = bytes
	}

	usage := &pluginCore.Usage{
		UserID:          userID,
		BytesUploaded:   bytesUploaded,
		BytesDownloaded: bytesDownloaded,
		BytesStored:     bytesStored,
		LastUpdated:     time.Now().UTC(),
		UsageByType:     usageByType,
	}

	return usage, nil
}

// validateBytes validates that bytes value is valid (not zero)
func (um *UsageManager) validateBytes(bytes uint64) error {
	if bytes == 0 {
		return pluginModels.ErrInvalidBytes
	}
	return nil
}
