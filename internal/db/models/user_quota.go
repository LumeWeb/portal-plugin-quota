package models

import (
	"time"

	"gorm.io/gorm"
)

// UserQuota - Aggregated daily quota usage
type UserQuota struct {
	gorm.Model
	UserID          uint      `gorm:"uniqueIndex:user_date"`
	Date            time.Time `gorm:"index;uniqueIndex:user_date"`
	BytesUploaded   uint64
	BytesDownloaded uint64
	BytesStored     uint64
}

// TableName sets the table name for UserQuota
func (UserQuota) TableName() string {
	return "user_quotas"
}

// BeforeCreate validates the UserQuota model before creation
func (u *UserQuota) BeforeCreate(_ *gorm.DB) error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if u.Date.IsZero() {
		return ErrInvalidDate
	}

	return nil
}

// BeforeUpdate validates the UserQuota model before update
func (u *UserQuota) BeforeUpdate(_ *gorm.DB) error {
	if u.UserID <= 0 {
		return ErrInvalidUserID
	}

	if u.Date.IsZero() {
		return ErrInvalidDate
	}

	return nil
}

// UpsertDailyUsage updates or creates daily usage records atomically
func UpsertDailyUsage(db *gorm.DB, userID uint, usageType UsageType, bytes int64) error {
	today := time.Now().UTC().Truncate(24 * time.Hour)

	// Create a new record with default values for upsert
	quota := UserQuota{
		UserID: userID,
		Date:   today,
	}

	// Use database-specific upsert mechanisms to handle concurrent access
	switch db.Dialector.Name() {
	case "mysql":
		return upsertMySQL(db, &quota, usageType, bytes)
	case "sqlite":
		return upsertSQLite(db, &quota, usageType, bytes)
	default:
		// Fallback to generic approach
		return upsertGeneric(db, &quota, usageType, bytes)
	}
}

// upsertMySQL handles MySQL-specific upsert
func upsertMySQL(db *gorm.DB, quota *UserQuota, usageType UsageType, bytes int64) error {
	query := `
		INSERT INTO user_quotas (user_id, date, bytes_uploaded, bytes_downloaded, bytes_stored, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
	`

	var updates string
	switch usageType {
	case UsageTypeUpload:
		updates = "bytes_uploaded = bytes_uploaded + ?"
	case UsageTypeDownload:
		updates = "bytes_downloaded = bytes_downloaded + ?"
	case UsageTypeStorageAdd:
		updates = "bytes_stored = bytes_stored + ?"
	case UsageTypeStorageRemove:
		updates = "bytes_stored = CASE WHEN bytes_stored + ? < 0 THEN 0 ELSE bytes_stored + ? END"
	default:
		return nil
	}

	query += updates

	now := time.Now().UTC()
	args := []interface{}{
		quota.UserID, quota.Date, quota.BytesUploaded, quota.BytesDownloaded, quota.BytesStored, now, now,
	}

	if usageType == UsageTypeStorageRemove {
		args = append(args, bytes, bytes)
	} else {
		args = append(args, bytes)
	}

	return db.Exec(query, args...).Error
}

// upsertSQLite handles SQLite-specific upsert
func upsertSQLite(db *gorm.DB, quota *UserQuota, usageType UsageType, bytes int64) error {
	query := `
		INSERT OR IGNORE INTO user_quotas (user_id, date, bytes_uploaded, bytes_downloaded, bytes_stored, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().UTC()
	args := []interface{}{
		quota.UserID, quota.Date, quota.BytesUploaded, quota.BytesDownloaded, quota.BytesStored, now, now,
	}

	if err := db.Exec(query, args...).Error; err != nil {
		return err
	}

	// Now update the record
	updates := ""
	switch usageType {
	case UsageTypeUpload:
		updates = "bytes_uploaded = bytes_uploaded + ?"
	case UsageTypeDownload:
		updates = "bytes_downloaded = bytes_downloaded + ?"
	case UsageTypeStorageAdd:
		updates = "bytes_stored = bytes_stored + ?"
	case UsageTypeStorageRemove:
		updates = "bytes_stored = CASE WHEN bytes_stored + ? < 0 THEN 0 ELSE bytes_stored + ? END"
	default:
		return nil
	}

	updateQuery := `UPDATE user_quotas SET ` + updates + `, updated_at = ? WHERE user_id = ? AND date = ?`
	updateArgs := []interface{}{bytes, now, quota.UserID, quota.Date}

	if usageType == UsageTypeStorageRemove {
		updateArgs = []interface{}{bytes, bytes, now, quota.UserID, quota.Date}
	}

	return db.Exec(updateQuery, updateArgs...).Error
}

// upsertGeneric handles upsert for databases without specific upsert syntax
func upsertGeneric(db *gorm.DB, quota *UserQuota, usageType UsageType, bytes int64) error {
	// Try to create the record first
	result := db.Where("user_id = ? AND date = ?", quota.UserID, quota.Date).FirstOrCreate(quota)
	if result.Error != nil {
		return result.Error
	}

	// Update the appropriate field based on usage type
	assignments := getUpdateAssignments(usageType, bytes)
	return db.Model(quota).Updates(assignments).Error
}

// getUpdateAssignments returns the assignments for updating quota values atomically
func getUpdateAssignments(usageType UsageType, bytes int64) map[string]interface{} {
	assignments := make(map[string]interface{})

	switch usageType {
	case UsageTypeUpload:
		assignments["bytes_uploaded"] = gorm.Expr("bytes_uploaded + ?", bytes)
	case UsageTypeDownload:
		assignments["bytes_downloaded"] = gorm.Expr("bytes_downloaded + ?", bytes)
	case UsageTypeStorageAdd:
		assignments["bytes_stored"] = gorm.Expr("bytes_stored + ?", bytes)
	case UsageTypeStorageRemove:
		// Apply signed delta and clamp to 0 minimum
		assignments["bytes_stored"] = gorm.Expr("CASE WHEN bytes_stored + ? < 0 THEN 0 ELSE bytes_stored + ? END", bytes, bytes)
	}

	return assignments
}
