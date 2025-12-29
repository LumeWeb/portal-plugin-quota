package models

import (
	"errors"
)

// Predefined validation errors
var (
	ErrInvalidQuotaCheckReason = errors.New("quota check reason is invalid")
)

// QuotaCheckReason represents the reason for a quota check result
type QuotaCheckReason string

const (
	QuotaCheckReasonOK                QuotaCheckReason = "OK"
	QuotaCheckReasonLimitExceeded     QuotaCheckReason = "LIMIT_EXCEEDED"
	QuotaCheckReasonAllowanceDepleted QuotaCheckReason = "ALLOWANCE_DEPLETED"
	QuotaCheckReasonWarningThreshold  QuotaCheckReason = "WARNING_THRESHOLD"
	QuotaCheckReasonThresholdExceeded QuotaCheckReason = "THRESHOLD_EXCEEDED"
)

// IsValid checks if the quota check reason is valid
func (r QuotaCheckReason) IsValid() bool {
	switch r {
	case QuotaCheckReasonOK, QuotaCheckReasonLimitExceeded, QuotaCheckReasonAllowanceDepleted, QuotaCheckReasonWarningThreshold, QuotaCheckReasonThresholdExceeded:
		return true
	default:
		return false
	}
}

// String returns the string representation of the quota check reason
func (r QuotaCheckReason) String() string {
	return string(r)
}

// TableName sets the table name for QuotaCheckReason
func (QuotaCheckReason) TableName() string {
	return "quota_check_reasons"
}
