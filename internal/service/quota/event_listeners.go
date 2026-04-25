package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal/core"
	portalModels "go.lumeweb.com/portal/db/models"
	portalEvent "go.lumeweb.com/portal/event"
	"go.uber.org/zap"
)

var (
	errServiceUnavailable = errors.New("service unavailable")
	errUploadSizeOverflow = errors.New("upload size exceeds maximum int64 value")
)

// usageRecordingFunc is a function type for recording usage within a reservation
type usageRecordingFunc func(ctx context.Context, userID uint, uploadID uint, bytes uint64, ip string) error

// handleReservationWithUsage handles reservation release and usage recording atomically.
// It acquires a lock, releases the reservation if it exists, and records usage if successful.
// Returns true if a reservation was found and handled.
func (s *QuotaServiceDefault) handleReservationWithUsage(
	ctx context.Context,
	userID uint,
	reservationUUID string,
	bytes uint64,
	successful bool,
	recordUsage usageRecordingFunc,
	uploadID uint,
	ip string,
	operation string,
) (bool, error) {
	lock, err := s.lockManager.AcquireLock(ctx, userID)
	if err != nil {
		s.Logger().Error("Failed to acquire user lock",
			zap.Uint("userID", userID),
			zap.String("operation", operation),
			zap.String("reservation_uuid", reservationUUID),
			zap.Error(err))
		return true, err
	}
	defer lock.Release()

	reservation := s.reservationManager.GetReservation(reservationUUID)
	if reservation == nil {
		return false, nil
	}

	s.Logger().Debug("Releasing reservation",
		zap.Uint("userID", userID),
		zap.String("operation", operation),
		zap.String("reservation_uuid", reservationUUID),
		zap.Uint64("bytes", bytes),
		zap.Bool("successful", successful))

	reservation.Release()

	if successful {
		s.Logger().Debug("Recording usage atomically",
			zap.Uint("userID", userID),
			zap.String("operation", operation),
			zap.Uint("uploadID", uploadID),
			zap.Uint64("bytes", bytes))

		if err := recordUsage(ctx, userID, uploadID, bytes, ip); err != nil {
			s.Logger().Error("Failed to record usage",
				zap.Uint("userID", userID),
				zap.String("operation", operation),
				zap.Uint("uploadID", uploadID),
				zap.Uint64("bytes", bytes),
				zap.Error(err))
		}
	}

	return true, nil
}

// registerEventListeners registers all portal event listeners for the quota service
func (s *QuotaServiceDefault) registerEventListeners() {
	// Register upload completed handler
	portalEvent.OnUploadCompleted(s.Context(), s.handleUploadCompleted)

	// Register download completed handler
	portalEvent.OnDownloadCompleted(s.Context(), s.handleDownloadCompleted)

	// Register storage pin handler
	portalEvent.OnStorageObjectPinned(s.Context(), s.handleStorageObjectPinned)

	// Register storage unpin handler
	portalEvent.OnStorageObjectUnpinned(s.Context(), s.handleStorageObjectUnpinned)
}

// getUploadSize retrieves the upload size for storage change recording.
// It returns an error if the upload size exceeds math.MaxInt64, as this would cause overflow
// when converting to int64 for RecordStorageChange. In such cases, the size is capped at
// math.MaxInt64 to prevent overflow and the error is logged.
func (s *QuotaServiceDefault) getUploadSize(ctx context.Context, pin *portalModels.Pin) (uint64, error) {
	upload, err := s.getUploadForPin(ctx, pin)
	if err != nil {
		return 0, err
	}

	if upload.Size > math.MaxInt64 {
		s.Logger().Warn("Upload size exceeds maximum int64 value, capping for storage change",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Uint64("uploadSize", upload.Size),
			zap.Int64("cappedSize", math.MaxInt64))
		return math.MaxInt64, fmt.Errorf("%w: actual size %d", errUploadSizeOverflow, upload.Size)
	}

	return upload.Size, nil
}

// getUploadForPin retrieves the upload for a given pin.
func (s *QuotaServiceDefault) getUploadForPin(ctx context.Context, pin *portalModels.Pin) (*portalModels.Upload, error) {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.getUploadForPin")
	defer span.End()

	uploadService := core.GetService[core.UploadService](s.Context(), core.UPLOAD_SERVICE)
	if uploadService == nil {
		s.Logger().Error("Upload service not available",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID))
		return nil, errServiceUnavailable
	}

	upload, err := uploadService.GetUploadByID(ctx, pin.UploadID)
	if err != nil {
		s.Logger().Error("Failed to get upload for pin",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Error(err))
		return nil, err
	}

	return upload, nil
}

// getStorageSizeToRecord calculates the effective storage size to record.
// It checks for redundancy metadata in the upload's Metadata field and scales
// the size accordingly. If no metadata exists, it returns the original size.
func (s *QuotaServiceDefault) getStorageSizeToRecord(upload *portalModels.Upload, defaultSize uint64) uint64 {
	// Check if upload has metadata
	var metadataMap map[string]interface{}
	if err := json.Unmarshal([]byte(upload.Metadata), &metadataMap); err != nil {
		s.Logger().Debug("Upload metadata not parseable for redundancy scaling",
			zap.Uint("uploadID", upload.ID),
			zap.Error(err))
		return defaultSize
	}

	// Parse redundancy metadata
	slabMeta, err := pluginCore.ParseRedundancyMetadata(metadataMap)
	if err != nil {
		s.Logger().Debug("No redundancy metadata found for upload",
			zap.Uint("uploadID", upload.ID),
			zap.Error(err))
		return defaultSize
	}

	// Calculate scaled size using DataSize (original data) and redundancy factors
	scaled := pluginCore.CalculateScaledSize(upload.Size, slabMeta.MinShards, slabMeta.TotalSectors)

	s.Logger().Debug("Scaled storage size based on redundancy metadata",
		zap.Uint("uploadID", upload.ID),
		zap.Uint64("originalSize", upload.Size),
		zap.Uint64("scaledSize", scaled),
		zap.Uint64("minShards", slabMeta.MinShards),
		zap.Uint64("totalSectors", slabMeta.TotalSectors))

	return scaled
}

// handleUploadCompleted handles the upload completed event
func (s *QuotaServiceDefault) handleUploadCompleted(ctx context.Context, uploadID uint, bytes uint64, ip string, userID *uint, reservationUUID *string, successful bool) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.handleUploadCompleted")
	defer span.End()

	if userID == nil {
		s.Logger().Debug("Skipping anonymous upload for quota tracking",
			zap.Uint("uploadID", uploadID),
			zap.Uint64("bytes", bytes))
		return nil
	}

	if reservationUUID != nil {
		_, err := s.handleReservationWithUsage(
			ctx,
			*userID,
			*reservationUUID,
			bytes,
			successful,
			s.RecordUpload,
			uploadID,
			ip,
			"upload",
		)
		return err
	}

	s.Logger().Debug("Recording upload usage from event",
		zap.Uint("userID", *userID),
		zap.Uint("uploadID", uploadID),
		zap.Uint64("bytes", bytes))

	return s.RecordUpload(ctx, *userID, uploadID, bytes, ip)
}

// handleDownloadCompleted handles the download completed event
func (s *QuotaServiceDefault) handleDownloadCompleted(ctx context.Context, uploadID uint, bytes uint64, ip string, userID *uint, reservationUUID *string, successful bool) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.handleDownloadCompleted")
	defer span.End()

	var effectiveUserID uint
	if userID == nil {
		effectiveUserID = 0 // Anonymous download - will use shared usage
	} else {
		effectiveUserID = *userID
	}

	if reservationUUID != nil {
		handled, err := s.handleReservationWithUsage(
			ctx,
			effectiveUserID,
			*reservationUUID,
			bytes,
			successful,
			s.RecordDownload,
			uploadID,
			ip,
			"download",
		)
		if handled && err == nil && !successful {
			// Track failed downloads in metrics
			DownloadFailed.WithLabelValues().Inc()
		}
		return err
	}

	return s.RecordDownload(ctx, effectiveUserID, uploadID, bytes, ip)
}

// handleStorageObjectPinned handles the storage object pinned event
func (s *QuotaServiceDefault) handleStorageObjectPinned(ctx context.Context, pin *portalModels.Pin, ip string, reservationUUID *string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.handleStorageObjectPinned")
	defer span.End()

	upload, err := s.getUploadForPin(ctx, pin)
	if err != nil {
		return err
	}

	// Get the base size and apply redundancy scaling
	size := upload.Size
	if size > math.MaxInt64 {
		s.Logger().Warn("Upload size exceeds maximum int64 value, capping for storage change",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Uint64("uploadSize", size),
			zap.Int64("cappedSize", math.MaxInt64))
		size = math.MaxInt64
	}

	// Apply redundancy scaling
	scaledSize := s.getStorageSizeToRecord(upload, size)

	// Create adapter for RecordStorageChange to match usageRecordingFunc signature
	recordStorageFunc := func(ctx context.Context, userID uint, uploadID uint, bytes uint64, ip string) error {
		return s.RecordStorageChange(ctx, userID, uploadID, int64(bytes), ip)
	}

	if reservationUUID != nil {
		_, err := s.handleReservationWithUsage(
			ctx,
			pin.UserID,
			*reservationUUID,
			scaledSize,
			true, // Storage pin is always successful
			recordStorageFunc,
			pin.UploadID,
			ip,
			"storage_pin",
		)
		return err
	}

	s.Logger().Debug("Recording storage usage from pin event",
		zap.Uint("userID", pin.UserID),
		zap.Uint("uploadID", pin.UploadID),
		zap.Uint64("originalSize", size),
		zap.Uint64("scaledSize", scaledSize))

	return s.RecordStorageChange(ctx, pin.UserID, pin.UploadID, int64(scaledSize), ip)
}

// handleStorageObjectUnpinned handles the storage object unpinned event
func (s *QuotaServiceDefault) handleStorageObjectUnpinned(ctx context.Context, pin *portalModels.Pin, ip string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.handleStorageObjectUnpinned")
	defer span.End()

	upload, err := s.getUploadForPin(ctx, pin)
	if err != nil {
		return err
	}

	// Get the base size and apply redundancy scaling
	size := upload.Size
	if size > math.MaxInt64 {
		s.Logger().Warn("Upload size exceeds maximum int64 value, capping for storage change",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Uint64("uploadSize", size),
			zap.Int64("cappedSize", math.MaxInt64))
		size = math.MaxInt64
	}

	// Apply redundancy scaling
	scaledSize := s.getStorageSizeToRecord(upload, size)

	s.Logger().Debug("Recording storage usage from unpin event",
		zap.Uint("userID", pin.UserID),
		zap.Uint("uploadID", pin.UploadID),
		zap.Uint64("originalSize", size),
		zap.Uint64("scaledSize", scaledSize))

	// Storage removal uses negative bytes
	return s.RecordStorageChange(ctx, pin.UserID, pin.UploadID, -int64(scaledSize), ip)
}
