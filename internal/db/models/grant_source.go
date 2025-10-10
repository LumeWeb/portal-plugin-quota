package models

import (
	"errors"
)

// Predefined validation errors
var (
	ErrInvalidGrantSource = errors.New("grant source is invalid")
)

// GrantSource - Origin of the grant
type GrantSource string

const (
	GrantSourceSubscription GrantSource = "SUBSCRIPTION"
	GrantSourcePAYGAddon    GrantSource = "PAYG_ADDON"
	GrantSourceBonus        GrantSource = "BONUS"
	GrantSourcePromo        GrantSource = "PROMO"
)

// IsValid checks if the grant source is valid
func (g GrantSource) IsValid() bool {
	switch g {
	case GrantSourceSubscription, GrantSourcePAYGAddon, GrantSourceBonus, GrantSourcePromo:
		return true
	default:
		return false
	}
}

// String returns the string representation of the grant source
func (g GrantSource) String() string {
	return string(g)
}

// GetGrantPriority returns the priority level of a grant source
func (g GrantSource) GetGrantPriority() int {
	switch g {
	case GrantSourcePAYGAddon:
		return 4
	case GrantSourcePromo:
		return 3
	case GrantSourceBonus:
		return 2
	case GrantSourceSubscription:
		return 1
	default:
		return 0
	}
}
