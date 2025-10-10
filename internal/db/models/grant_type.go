package models

import (
	"errors"
)

// Predefined validation errors
var (
	ErrInvalidGrantType = errors.New("grant type is invalid")
)

// GrantType - Type of resource being granted
type GrantType string

const (
	GrantTypeStorage  GrantType = "STORAGE"
	GrantTypeUpload   GrantType = "UPLOAD"
	GrantTypeDownload GrantType = "DOWNLOAD"
)

// IsValid checks if the grant type is valid
func (g GrantType) IsValid() bool {
	switch g {
	case GrantTypeStorage, GrantTypeUpload, GrantTypeDownload:
		return true
	default:
		return false
	}
}

// String returns the string representation of the grant type
func (g GrantType) String() string {
	return string(g)
}
