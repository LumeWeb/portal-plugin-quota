package managers

import (
	"fmt"
	"math"
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	pluginModels "go.lumeweb.com/portal-plugin-quota/internal/db/models"
	portalCore "go.lumeweb.com/portal/core"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// UsageAggregator aggregates usage data across time periods
type UsageAggregator interface {
	GetAggregatedUsageByType(userID uint, usageType pluginModels.UsageType) (uint64, error)
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
	config := pluginModels.UserQuotaConfig{
		UserID:            userID,
		EnforcementPolicy: pluginModels.EnforcementPolicyHardLimits,
	}

	result := um.db.Where("user_id = ?", userID).FirstOrCreate(&config)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get or create user quota config: %w", result.Error)
	}

	return &config, nil
}

// GetAggregatedUsageByType returns the aggregated usage for a specific user and usage type
func (um *UsageManager) GetAggregatedUsageByType(userID uint, usageType pluginModels.UsageType) (uint64, error) {
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

	endDate := time.Now()
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
		Timestamp:  time.Now().UTC(),
	}

	if err := um.RecordUserUsageDetail(detail); err != nil {
		return fmt.Errorf("failed to record download usage detail: %w", err)
	}

	// Update daily usage with the actual bytes consumed by this user
	if err := um.UpdateDailyUsage(userID, pluginModels.UsageTypeDownload, int64(sharedBytes)); err != nil {
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

	today := time.Now().UTC().Truncate(24 * time.Hour)

	// Create a new record with initial values
	dailyQuota := pluginModels.UserQuota{
		UserID: userID,
		Date:   today,
	}

	// Set initial values based on usage type
	switch usageType {
	case pluginModels.UsageTypeUpload:
		dailyQuota.BytesUploaded = uint64(bytes)
	case pluginModels.UsageTypeDownload:
		dailyQuota.BytesDownloaded = uint64(bytes)
	case pluginModels.UsageTypeStorageAdd:
		dailyQuota.BytesStored = uint64(bytes)
	case pluginModels.UsageTypeStorageRemove:
		if bytes < 0 {
			dailyQuota.BytesStored = 0
		} else {
			dailyQuota.BytesStored = uint64(bytes)
		}
	}

	// Determine the update assignments based on usage type
	assignments := um.getUpdateAssignments(usageType, bytes)

	// Use GORM upsert with OnConflict
	return um.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "date"},
		},
		DoUpdates: clause.Assignments(assignments),
	}).Create(&dailyQuota).Error
}

// getUpdateAssignments returns the assignments for updating quota values atomically
func (um *UsageManager) getUpdateAssignments(usageType pluginModels.UsageType, bytes int64) map[string]interface{} {
	assignments := make(map[string]interface{})

	switch usageType {
	case pluginModels.UsageTypeUpload:
		assignments["bytes_uploaded"] = gorm.Expr("bytes_uploaded + ?", bytes)
	case pluginModels.UsageTypeDownload:
		assignments["bytes_downloaded"] = gorm.Expr("bytes_downloaded + ?", bytes)
	case pluginModels.UsageTypeStorageAdd:
		assignments["bytes_stored"] = gorm.Expr("bytes_stored + ?", bytes)
	case pluginModels.UsageTypeStorageRemove:
		// Apply signed delta and clamp to 0 minimum
		assignments["bytes_stored"] = gorm.Expr("CASE WHEN bytes_stored + ? < 0 THEN 0 ELSE bytes_stored + ? END", bytes, bytes)
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
	for _, usageType := range []pluginModels.UsageType{
		pluginModels.UsageTypeUpload,
		pluginModels.UsageTypeDownload,
		pluginModels.UsageTypeStorageAdd,
		pluginModels.UsageTypeStorageRemove,
	} {
		bytes, err := um.GetAggregatedUsageByType(userID, usageType)
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
