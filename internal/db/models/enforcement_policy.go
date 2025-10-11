package models

import (
	"errors"
)

// Predefined validation errors
var (
	ErrInvalidEnforcementPolicy = errors.New("enforcement policy is invalid")
)

// EnforcementPolicy represents the quota enforcement policy
type EnforcementPolicy string

const (
	EnforcementPolicyHardLimits EnforcementPolicy = "HARD_LIMITS"
	EnforcementPolicyUnlimited  EnforcementPolicy = "UNLIMITED"
	EnforcementPolicyAllowance  EnforcementPolicy = "ALLOWANCE"
	EnforcementPolicyThreshold  EnforcementPolicy = "THRESHOLD"
)

// IsValid checks if the enforcement policy is valid
func (e EnforcementPolicy) IsValid() bool {
	switch e {
	case EnforcementPolicyHardLimits, EnforcementPolicyUnlimited, EnforcementPolicyAllowance, EnforcementPolicyThreshold:
		return true
	default:
		return false
	}
}

// String returns the string representation of the enforcement policy
func (e EnforcementPolicy) String() string {
	return string(e)
}

// TableName sets the table name for EnforcementPolicy
func (EnforcementPolicy) TableName() string {
	return "enforcement_policies"
}
