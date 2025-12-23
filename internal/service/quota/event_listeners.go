package quota

import (
	"errors"

	portalCore "go.lumeweb.com/portal/core"
	portalModels "go.lumeweb.com/portal/db/models"
	portalEvent "go.lumeweb.com/portal/event"
	"go.uber.org/zap"
)

var errServiceUnavailable = errors.New("service unavailable")

// registerEventListeners registers all portal event listeners for the quota service
func (s *QuotaServiceDefault) registerEventListeners() {
	if s.ctx == nil {
		return
	}

	// Register upload completed handler
	portalEvent.OnUploadCompleted(s.ctx, s.handleUploadCompleted)

	// Register download completed handler
	portalEvent.OnDownloadCompleted(s.ctx, s.handleDownloadCompleted)

	// Register storage pin handler
	portalEvent.OnStorageObjectPinned(s.ctx, s.handleStorageObjectPinned)

	// Register storage unpin handler
	portalEvent.OnStorageObjectUnpinned(s.ctx, s.handleStorageObjectUnpinned)
}

// handleUploadCompleted handles the upload completed event
func (s *QuotaServiceDefault) handleUploadCompleted(uploadID uint, bytes uint64, ip string, userID *uint) error {
	// Anonymous uploads are not tracked for quota
	if userID == nil {
		s.logger.Debug("Skipping anonymous upload for quota tracking",
			zap.Uint("uploadID", uploadID),
			zap.Uint64("bytes", bytes))
		return nil
	}

	s.logger.Debug("Recording upload usage from event",
		zap.Uint("userID", *userID),
		zap.Uint("uploadID", uploadID),
		zap.Uint64("bytes", bytes))

	return s.RecordUpload(*userID, uploadID, bytes, ip)
}

// handleDownloadCompleted handles the download completed event
func (s *QuotaServiceDefault) handleDownloadCompleted(uploadID uint, bytes uint64, ip string, userID *uint) error {
	var effectiveUserID uint
	if userID == nil {
		effectiveUserID = 0 // Anonymous download - will use shared usage
	} else {
		effectiveUserID = *userID
	}

	return s.RecordDownload(effectiveUserID, uploadID, bytes, ip)
}

// handleStorageObjectPinned handles the storage object pinned event
func (s *QuotaServiceDefault) handleStorageObjectPinned(pin *portalModels.Pin, ip string) error {
	// Get upload service to retrieve upload size
	uploadService := portalCore.GetService[portalCore.UploadService](s.ctx, portalCore.UPLOAD_SERVICE)
	if uploadService == nil {
		s.logger.Error("Upload service not available",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID))
		return errServiceUnavailable
	}

	upload, err := uploadService.GetUploadByID(s.ctx, pin.UploadID)
	if err != nil {
		s.logger.Error("Failed to get upload for pin",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Error(err))
		return err
	}

	s.logger.Debug("Recording storage usage from pin event",
		zap.Uint("userID", pin.UserID),
		zap.Uint("uploadID", pin.UploadID),
		zap.Uint64("bytes", upload.Size))

	return s.RecordStorageChange(pin.UserID, pin.UploadID, int64(upload.Size), ip)
}

// handleStorageObjectUnpinned handles the storage object unpinned event
func (s *QuotaServiceDefault) handleStorageObjectUnpinned(pin *portalModels.Pin, ip string) error {
	// Get upload service to retrieve upload size
	uploadService := portalCore.GetService[portalCore.UploadService](s.ctx, portalCore.UPLOAD_SERVICE)
	if uploadService == nil {
		s.logger.Error("Upload service not available",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID))
		return errServiceUnavailable
	}

	upload, err := uploadService.GetUploadByID(s.ctx, pin.UploadID)
	if err != nil {
		s.logger.Error("Failed to get upload for unpin",
			zap.Uint("pinID", pin.ID),
			zap.Uint("uploadID", pin.UploadID),
			zap.Error(err))
		return err
	}

	s.logger.Debug("Recording storage usage from unpin event",
		zap.Uint("userID", pin.UserID),
		zap.Uint("uploadID", pin.UploadID),
		zap.Uint64("bytes", upload.Size))

	// Storage removal uses negative bytes
	return s.RecordStorageChange(pin.UserID, pin.UploadID, -int64(upload.Size), ip)
}
