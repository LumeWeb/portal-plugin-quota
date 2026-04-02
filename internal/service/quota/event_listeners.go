package quota

import (
	"context"
	"errors"
	"fmt"
	"math"

	"go.lumeweb.com/portal/core"
	portalModels "go.lumeweb.com/portal/db/models"
	portalEvent "go.lumeweb.com/portal/event"
	"go.uber.org/zap"
)

var (
	errServiceUnavailable = errors.New("service unavailable")
	errUploadSizeOverflow = errors.New("upload size exceeds maximum int64 value")
)

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
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.getUploadSize")
	defer span.End()

	uploadService := core.GetService[core.UploadService](s.Context(), core.UPLOAD_SERVICE)
	if uploadService == nil {
		s.Logger().Error("Upload service not available",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID))
		return 0, errServiceUnavailable
	}

	upload, err := uploadService.GetUploadByID(ctx, pin.UploadID)
	if err != nil {
		s.Logger().Error("Failed to get upload for pin",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Error(err))
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

	// If reservation exists, handle it
	if reservationUUID != nil {
		reservation := s.reservationManager.GetReservation(*reservationUUID)
		if reservation != nil {
			s.Logger().Debug("Releasing reservation for upload",
				zap.Uint("userID", *userID),
				zap.String("reservation_uuid", *reservationUUID),
				zap.Uint64("bytes", bytes),
				zap.Bool("successful", successful))

			// Release reservation (called regardless of success/failure)
			reservation.Release()
		}
		return nil
	}

	// Fall back to recording usage directly (only when no reservation exists)
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

	// If reservation exists, handle it
	if reservationUUID != nil {
		reservation := s.reservationManager.GetReservation(*reservationUUID)
		if reservation != nil {
			s.Logger().Debug("Releasing reservation for download",
				zap.Uint("effectiveUserID", effectiveUserID),
				zap.String("reservation_uuid", *reservationUUID),
				zap.Uint64("bytes", bytes),
				zap.Bool("successful", successful))

			// Release reservation (called regardless of success/failure)
			reservation.Release()
		}

		// Track failed downloads in metrics
		if !successful {
			DownloadFailed.WithLabelValues().Inc()
			s.Logger().Debug("Download failed - reservation released",
				zap.Uint("effectiveUserID", effectiveUserID),
				zap.String("reservation_uuid", *reservationUUID),
				zap.Uint64("bytes", bytes))
		}

		return nil
	}

	// Fall back to recording usage directly (only when no reservation exists)
	return s.RecordDownload(ctx, effectiveUserID, uploadID, bytes, ip)
}

// handleStorageObjectPinned handles the storage object pinned event
func (s *QuotaServiceDefault) handleStorageObjectPinned(ctx context.Context, pin *portalModels.Pin, ip string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.handleStorageObjectPinned")
	defer span.End()

	size, err := s.getUploadSize(ctx, pin)
	if err != nil && !errors.Is(err, errUploadSizeOverflow) {
		return err
	}

	s.Logger().Debug("Recording storage usage from pin event",
		zap.Uint("userID", pin.UserID),
		zap.Uint("uploadID", pin.UploadID),
		zap.Uint64("bytes", size))

	return s.RecordStorageChange(ctx, pin.UserID, pin.UploadID, int64(size), ip)
}

// handleStorageObjectUnpinned handles the storage object unpinned event
func (s *QuotaServiceDefault) handleStorageObjectUnpinned(ctx context.Context, pin *portalModels.Pin, ip string) error {
	ctx, span := core.TraceMethod(ctx, "QuotaServiceDefault.handleStorageObjectUnpinned")
	defer span.End()

	size, err := s.getUploadSize(ctx, pin)
	if err != nil && !errors.Is(err, errUploadSizeOverflow) {
		return err
	}

	s.Logger().Debug("Recording storage usage from unpin event",
		zap.Uint("userID", pin.UserID),
		zap.Uint("uploadID", pin.UploadID),
		zap.Uint64("bytes", size))

	// Storage removal uses negative bytes
	return s.RecordStorageChange(ctx, pin.UserID, pin.UploadID, -int64(size), ip)
}
