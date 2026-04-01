package managers

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// ReservationManagerDefault implements CRUD operations for quota reservations
type ReservationManagerDefault struct {
	db        *gorm.DB
	timeout   time.Duration
	logger    *core.Logger
}

// NewReservationManager creates a new reservation manager
func NewReservationManager(ctx core.Context) *ReservationManagerDefault {
	return &ReservationManagerDefault{
		db:      ctx.DB(),
		timeout: 1 * time.Hour,
		logger:  ctx.NamedLogger("quota.ReservationManager"),
	}
}

// SetTimeout sets the reservation timeout
func (rm *ReservationManagerDefault) SetTimeout(timeout time.Duration) {
	rm.timeout = timeout
}

// CreateReservation creates a new quota reservation
func (rm *ReservationManagerDefault) CreateReservation(ctx context.Context, userID uint, usageType models.UsageType, bytes uint64, ip string) (*models.QuotaReservation, error) {
	rm.logger.Debug("Creating reservation",
		zap.Uint("user_id", userID),
		zap.String("usage_type", usageType.String()),
		zap.Uint64("bytes", bytes),
		zap.String("ip", ip),
	)

	reservation := &models.QuotaReservation{
		UserID:   userID,
		Type:     usageType,
		Bytes:    bytes,
		Status:   models.ReservationStatusPending,
		SourceIP: ip,
	}

	if err := rm.db.WithContext(ctx).Create(reservation).Error; err != nil {
		rm.logger.Error("Failed to create reservation",
			zap.Uint("user_id", userID),
			zap.String("usage_type", usageType.String()),
			zap.Uint64("bytes", bytes),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create reservation: %w", err)
	}

	rm.logger.Debug("Reservation created successfully",
		zap.Uint("reservation_id", reservation.ID),
		zap.Uint("user_id", userID),
		zap.Uint64("bytes", bytes),
	)

	return reservation, nil
}

// CommitReservation commits a reservation to a usage record
func (rm *ReservationManagerDefault) CommitReservation(ctx context.Context, reservationID uint, uploadID uint) error {
	rm.logger.Debug("Committing reservation",
		zap.Uint("reservation_id", reservationID),
		zap.Uint("upload_id", uploadID),
	)

	return rm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation models.QuotaReservation

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&reservation, reservationID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				rm.logger.Debug("Reservation not found for commit",
					zap.Uint("reservation_id", reservationID),
				)
				return nil
			}
			rm.logger.Error("Failed to lock reservation for commit",
				zap.Uint("reservation_id", reservationID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to lock reservation: %w", err)
		}

		if reservation.Status != models.ReservationStatusPending {
			rm.logger.Debug("Reservation not pending, skipping commit",
				zap.Uint("reservation_id", reservationID),
				zap.String("status", string(reservation.Status)),
			)
			return nil
		}

		reservation.Status = models.ReservationStatusCommitted
		reservation.UpdatedAt = time.Now()
		if uploadID > 0 {
			reservation.UploadID = &uploadID
		}

		if err := tx.Save(&reservation).Error; err != nil {
			rm.logger.Error("Failed to update reservation status",
				zap.Uint("reservation_id", reservationID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to update reservation: %w", err)
		}

		detail := &models.UserUsageDetail{
			UserID:    reservation.UserID,
			Type:      reservation.Type,
			Bytes:     reservation.Bytes,
			IP:        models.IPAddr(reservation.SourceIP),
			Timestamp: time.Now().UTC(),
		}

		if uploadID > 0 {
			detail.UploadID = uploadID
		}

		if err := tx.Create(detail).Error; err != nil {
			rm.logger.Error("Failed to create usage detail",
				zap.Uint("user_id", reservation.UserID),
				zap.Uint("upload_id", uploadID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to create usage detail: %w", err)
		}

		rm.logger.Debug("Reservation committed successfully",
			zap.Uint("reservation_id", reservationID),
			zap.Uint("user_id", reservation.UserID),
			zap.Uint64("bytes", reservation.Bytes),
		)

		return nil
	})
}

// ReleaseReservation releases a reservation
func (rm *ReservationManagerDefault) ReleaseReservation(ctx context.Context, reservationID uint) error {
	rm.logger.Debug("Releasing reservation",
		zap.Uint("reservation_id", reservationID),
	)

	return rm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var reservation models.QuotaReservation

		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&reservation, reservationID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				rm.logger.Debug("Reservation not found for release",
					zap.Uint("reservation_id", reservationID),
				)
				return nil
			}
			rm.logger.Error("Failed to lock reservation for release",
				zap.Uint("reservation_id", reservationID),
				zap.Error(err),
			)
			return fmt.Errorf("failed to lock reservation: %w", err)
		}

		if reservation.Status != models.ReservationStatusPending {
			rm.logger.Debug("Reservation not pending, skipping release",
				zap.Uint("reservation_id", reservationID),
				zap.String("status", string(reservation.Status)),
			)
			return nil
		}

		reservation.Status = models.ReservationStatusRolledBack
		reservation.UpdatedAt = time.Now()

		if err := tx.Save(&reservation).Error; err != nil {
			rm.logger.Error("Failed to update reservation status for release",
				zap.Uint("reservation_id", reservationID),
				zap.Error(err),
			)
			return err
		}

		rm.logger.Debug("Reservation released successfully",
			zap.Uint("reservation_id", reservationID),
			zap.Uint("user_id", reservation.UserID),
			zap.Uint64("bytes", reservation.Bytes),
		)

		return nil
	})
}

// GetReservationByID retrieves a reservation
func (rm *ReservationManagerDefault) GetReservationByID(ctx context.Context, reservationID uint) (*models.QuotaReservation, error) {
	var reservation models.QuotaReservation
	if err := rm.db.WithContext(ctx).First(&reservation, reservationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			rm.logger.Debug("Reservation not found",
				zap.Uint("reservation_id", reservationID),
			)
			return nil, nil
		}
		rm.logger.Error("Failed to get reservation",
			zap.Uint("reservation_id", reservationID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get reservation: %w", err)
	}
	return &reservation, nil
}

// GetPendingReservationsForUser gets all pending reservations for a user
func (rm *ReservationManagerDefault) GetPendingReservationsForUser(ctx context.Context, userID uint) ([]*models.QuotaReservation, error) {
	var reservations []*models.QuotaReservation
	err := rm.db.WithContext(ctx).
		Where("user_id = ? AND status = ?", userID, models.ReservationStatusPending).
		Find(&reservations).Error
	if err != nil {
		rm.logger.Error("Failed to get pending reservations for user",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get pending reservations: %w", err)
	}
	rm.logger.Debug("Retrieved pending reservations",
		zap.Uint("user_id", userID),
		zap.Int("count", len(reservations)),
	)
	return reservations, nil
}

// SumPendingBytesForUser sums pending reservation bytes for a user and type
func (rm *ReservationManagerDefault) SumPendingBytesForUser(ctx context.Context, userID uint, usageType models.UsageType) (uint64, error) {
	var sum int64
	err := rm.db.WithContext(ctx).
		Model(&models.QuotaReservation{}).
		Where("user_id = ? AND type = ? AND status = ?", userID, usageType, models.ReservationStatusPending).
		Select("COALESCE(SUM(bytes), 0)").
		Scan(&sum).Error
	if err != nil {
		rm.logger.Error("Failed to sum pending bytes for user",
			zap.Uint("user_id", userID),
			zap.String("usage_type", usageType.String()),
			zap.Error(err),
		)
		return 0, fmt.Errorf("failed to sum pending bytes: %w", err)
	}
	rm.logger.Debug("Summed pending reservation bytes",
		zap.Uint("user_id", userID),
		zap.String("usage_type", usageType.String()),
		zap.Uint64("sum", uint64(sum)),
	)
	return uint64(sum), nil
}

// CleanupStaleReservations releases pending reservations older than configured timeout for ALL users
func (rm *ReservationManagerDefault) CleanupStaleReservations(ctx context.Context) (int64, error) {
	threshold := time.Now().Add(-rm.timeout)

	rm.logger.Debug("Cleaning up stale reservations for all users",
		zap.Time("threshold", threshold),
	)

	result := rm.db.WithContext(ctx).
		Where("status = ? AND created_at < ?", models.ReservationStatusPending, threshold).
		Delete(&models.QuotaReservation{})

	if result.Error != nil {
		rm.logger.Error("Failed to cleanup stale reservations",
			zap.Error(result.Error),
		)
		return 0, fmt.Errorf("failed to cleanup stale reservations: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		rm.logger.Info("Cleaned up stale reservations",
			zap.Int64("count", result.RowsAffected),
			zap.Time("threshold", threshold),
		)
	}

	return result.RowsAffected, nil
}

// CleanupStaleReservationsForUser releases pending reservations for a specific user older than timeout
// This is optimized for the hot path during quota checks
func (rm *ReservationManagerDefault) CleanupStaleReservationsForUser(ctx context.Context, userID uint) (int64, error) {
	threshold := time.Now().Add(-rm.timeout)

	result := rm.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND created_at < ?", userID, models.ReservationStatusPending, threshold).
		Delete(&models.QuotaReservation{})

	if result.Error != nil {
		rm.logger.Error("Failed to cleanup stale reservations for user",
			zap.Uint("user_id", userID),
			zap.Error(result.Error),
		)
		return 0, fmt.Errorf("failed to cleanup stale reservations: %w", result.Error)
	}

	if result.RowsAffected > 0 {
		rm.logger.Debug("Cleaned up stale reservations for user",
			zap.Uint("user_id", userID),
			zap.Int64("count", result.RowsAffected),
		)
	}

	return result.RowsAffected, nil
}
