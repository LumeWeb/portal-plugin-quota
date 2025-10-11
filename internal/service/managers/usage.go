package managers

import (
	"errors"
	"fmt"
	"math"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UsageManager handles centralized usage recording and shared usage calculations
// Implements core.UsageManager interface
type UsageManager struct {
	ctx           core.Context
	db            *gorm.DB
	logger        *core.Logger
	config        *config.QuotaConfig
	pinService    core.PinService
	uploadService core.UploadService
}

// NewUsageManager creates a new usage manager
func NewUsageManager(ctx core.Context) *UsageManager {
	quotaConfig := core.GetServiceConfig[*config.QuotaConfig](ctx, pluginCore.QUOTA_SERVICE)

	return &UsageManager{
		ctx:           ctx,
		db:            ctx.DB(),
		logger:        ctx.NamedLogger("quota.UsageManager"),
		config:        quotaConfig,
		pinService:    core.GetService[core.PinService](ctx, core.PIN_SERVICE),
		uploadService: core.GetService[core.UploadService](ctx, core.UPLOAD_SERVICE),
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
		Timestamp:  time.Now(),
	}

	if err := um.recordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record upload usage detail: %w", err)
	}

	// Update daily usage
	if err := um.updateDailyUsage(userID, pluginModels.UsageTypeUpload, int64(bytes)); err != nil {
		return fmt.Errorf("failed to update daily upload usage: %w", err)
	}

	return nil
}

// RecordDownload records download usage for a user
// Implements core.UsageManager.RecordDownload
func (um *UsageManager) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := um.validateUserID(userID); err != nil {
		return err
	}
	if err := um.validateBytes(bytes); err != nil {
		return err
	}

	// For downloads, usage may be shared if multiple users have pinned the same object
	sharedWith := uint(1)
	sharedBytes := bytes

	if um.config != nil && um.config.EnableSharedUsage {
		// Calculate shared usage
		calculatedSharedWith, calculatedSharedBytes, err := um.calculateSharedUsage(uploadID, bytes)
		if err != nil {
			um.logger.Warn("Failed to calculate shared usage, falling back to individual usage", zap.Uint("uploadID", uploadID), zap.Uint("userID", userID), zap.Error(err))
		} else {
			sharedWith = calculatedSharedWith
			sharedBytes = calculatedSharedBytes
			// Ensure shared bytes is at least 1 to satisfy model validation
			if sharedBytes == 0 && bytes > 0 {
				sharedBytes = 1
			}
		}
	}

	detail := &pluginModels.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       pluginModels.UsageTypeDownload,
		Bytes:      sharedBytes,
		IP:         ip,
		SharedWith: sharedWith,
		Timestamp:  time.Now(),
	}

	if err := um.recordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record download usage detail: %w", err)
	}

	// Update daily usage with the actual bytes consumed by this user
	if err := um.updateDailyUsage(userID, pluginModels.UsageTypeDownload, int64(sharedBytes)); err != nil {
		return fmt.Errorf("failed to update daily download usage: %w", err)
	}

	return nil
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
		recordBytes = uint64(-bytes)
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
		Timestamp:  time.Now(),
	}

	if err := um.recordUserUsageDetail(detail); err != nil {
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

	if err := um.updateDailyUsage(userID, usageType, dailyUsageBytes); err != nil {
		return fmt.Errorf("failed to update daily storage usage: %w", err)
	}

	return nil
}

// calculateSharedUsage calculates how many users are sharing an object and the bytes per user
// This method is only used for download operations
func (um *UsageManager) calculateSharedUsage(uploadID uint, totalBytes uint64) (uint, uint64, error) {
	// Check if pin service is available
	if um.pinService == nil {
		return 1, totalBytes, fmt.Errorf("pin service not available")
	}

	// Get all pins for this upload using the PinService
	pins, err := um.pinService.GetPinsByUploadID(um.ctx, uploadID)
	if err != nil {
		return 1, totalBytes, fmt.Errorf("failed to get pins for upload: %w", err)
	}

	// Count unique users who have pinned this object
	userCount := uint(0)
	seenUsers := make(map[uint]bool)

	for _, pin := range pins {
		if !seenUsers[pin.UserID] {
			seenUsers[pin.UserID] = true
			userCount++
		}
	}

	// If no users found, default to 1 (shouldn't happen but be safe)
	if userCount == 0 {
		userCount = 1
	}

	// Calculate shared bytes per user using configured precision
	precision := 0
	if um.config != nil {
		precision = um.config.SharedUsagePrecision
	}
	sharedBytes := um.calculateSharedBytes(totalBytes, userCount, precision)

	return userCount, sharedBytes, nil
}

// calculateSharedBytes calculates the bytes per user with configurable precision
func (um *UsageManager) calculateSharedBytes(totalBytes uint64, userCount uint, precision int) uint64 {
	if userCount == 0 {
		return 0
	}

	// Ensure precision is within reasonable bounds
	if precision < 0 {
		precision = 0
	}
	if precision > 10 {
		precision = 10
	}

	// Calculate base shared bytes
	baseFloat := float64(totalBytes) / float64(userCount)
	
	// For precision 0, do exact division
	if precision == 0 {
		return uint64(baseFloat)
	}
	
	// Apply precision scaling and round up
	multiplier := math.Pow10(precision)
	scaledFloat := baseFloat * multiplier
	roundedBytes := uint64(math.Ceil(scaledFloat))
	
	// Ensure minimum of 1 byte when totalBytes > 0 and userCount > 0
	if roundedBytes == 0 && totalBytes > 0 && userCount > 0 {
		roundedBytes = 1
	}

	return roundedBytes
}

// recordUserUsageDetail records a detailed usage record
func (um *UsageManager) recordUserUsageDetail(detail *pluginModels.UserUsageDetail) error {
	return um.db.Create(detail).Error
}

// updateDailyUsage updates the daily aggregated usage for a user
func (um *UsageManager) updateDailyUsage(userID uint, usageType pluginModels.UsageType, bytes int64) error {
	if err := um.validateUserID(userID); err != nil {
		return err
	}

	today := time.Now().Truncate(24 * time.Hour)

	// Create the daily quota record with initial values
	dailyQuota := pluginModels.UserQuota{
		UserID: userID,
		Date:   today,
	}

	// Set the appropriate field based on usage type
	switch usageType {
	case pluginModels.UsageTypeUpload:
		if bytes > 0 {
			dailyQuota.BytesUploaded = uint64(bytes)
		}
	case pluginModels.UsageTypeDownload:
		if bytes > 0 {
			dailyQuota.BytesDownloaded = uint64(bytes)
		}
	case pluginModels.UsageTypeStorageAdd:
		if bytes > 0 {
			dailyQuota.BytesStored = uint64(bytes)
		}
	case pluginModels.UsageTypeStorageRemove:
		// For storage removal, we start with 0 and apply negative delta
		if bytes < 0 {
			dailyQuota.BytesStored = 0
		}
	}

	// Use upsert to handle concurrent access atomically
	return um.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "date"}},
		DoUpdates: clause.Assignments(um.getUpdateAssignments(usageType, bytes)),
	}).Create(&dailyQuota).Error
}

// getUpdateAssignments returns the assignments for updating quota values atomically
func (um *UsageManager) getUpdateAssignments(usageType pluginModels.UsageType, bytes int64) map[string]interface{} {
	assignments := make(map[string]interface{})

	switch usageType {
	case pluginModels.UsageTypeUpload:
		if bytes > 0 {
			assignments["bytes_uploaded"] = gorm.Expr("bytes_uploaded + ?", bytes)
		}
	case pluginModels.UsageTypeDownload:
		if bytes > 0 {
			assignments["bytes_downloaded"] = gorm.Expr("bytes_downloaded + ?", bytes)
		}
	case pluginModels.UsageTypeStorageAdd:
		if bytes > 0 {
			assignments["bytes_stored"] = gorm.Expr("bytes_stored + ?", bytes)
		}
	case pluginModels.UsageTypeStorageRemove:
		if bytes < 0 {
			// Apply signed delta and clamp to 0 minimum
			assignments["bytes_stored"] = gorm.Expr("CASE WHEN bytes_stored + ? < 0 THEN 0 ELSE bytes_stored + ? END", bytes, bytes)
		}
	}

	return assignments
}

// Validation methods

// validateUserID validates that a user ID is valid
func (um *UsageManager) validateUserID(userID uint) error {
	if userID == 0 {
		return pluginModels.ErrInvalidUserID
	}
	return nil
}

// GetCurrentUsage retrieves the current daily usage for a user
func (um *UsageManager) GetCurrentUsage(userID uint) (*pluginModels.UserQuota, error) {
	if err := um.validateUserID(userID); err != nil {
		return nil, err
	}

	today := time.Now().Truncate(24 * time.Hour)
	var dailyQuota pluginModels.UserQuota

	err := um.db.Where("user_id = ? AND date = ?", userID, today).First(&dailyQuota).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return empty quota record if none found for today
			dailyQuota = pluginModels.UserQuota{
				UserID: userID,
				Date:   today,
			}
			return &dailyQuota, nil
		}
		return nil, err
	}

	return &dailyQuota, nil
}

// validateBytes validates that bytes value is valid (not zero)
func (um *UsageManager) validateBytes(bytes uint64) error {
	if bytes == 0 {
		return pluginModels.ErrInvalidBytes
	}
	return nil
}
