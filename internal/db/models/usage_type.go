package models

import (
	"errors"
)

// Predefined validation errors
var (
	ErrInvalidUsageType = errors.New("usage type is invalid")
)

// UsageType represents the type of usage
type UsageType string

const (
	UsageTypeUpload        UsageType = "UPLOAD"
	UsageTypeDownload      UsageType = "DOWNLOAD"
	UsageTypeStorageAdd    UsageType = "STORAGE_ADD"
	UsageTypeStorageRemove UsageType = "STORAGE_REMOVE"
)

// IsValid checks if the usage type is valid
func (u UsageType) IsValid() bool {
	switch u {
	case UsageTypeUpload, UsageTypeDownload, UsageTypeStorageAdd, UsageTypeStorageRemove:
		return true
	default:
		return false
	}
}

// String returns the string representation of the usage type
func (u UsageType) String() string {
	return string(u)
}

// TableName sets the table name for UsageType
func (UsageType) TableName() string {
	return "usage_types"
}
