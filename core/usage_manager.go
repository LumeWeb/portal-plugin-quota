package core

// UsageManager defines the interface for usage recording and management
type UsageManager interface {
	// RecordUpload records upload usage for a user
	RecordUpload(userID, uploadID uint, bytes uint64, ip string) error
	
	// RecordDownload records download usage for a user
	RecordDownload(userID, uploadID uint, bytes uint64, ip string) error
	
	// RecordStorageChange records storage usage changes for a user
	RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error
}
