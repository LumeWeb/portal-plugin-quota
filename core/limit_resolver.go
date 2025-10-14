package core

import (
	"math"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
)

// LimitResolver provides unified limit resolution functionality across all quota policies
type LimitResolver interface {
	// ResolveEffectiveLimits resolves the effective limits for a user based on their configuration
	// This method consolidates the logic from getEffectiveLimits() and resolveEffectiveLimits()
	ResolveEffectiveLimits(config *models.UserQuotaConfig, policy models.EnforcementPolicy) (*EffectiveLimits, error)

	// ValidateThresholdVsLimit ensures threshold cannot exceed limit (for threshold policy)
	ValidateThresholdVsLimit(thresholdValue, limitValue int64, thresholdType string) error

	// ValidateThresholdValue validates that a threshold value is reasonable
	ValidateThresholdValue(thresholdValue int64, thresholdType string) error

	// ApplyLimit converts and validates database limit values to core limit values
	ApplyLimit(dest **uint64, source int64, limitName string, options ...LimitOption) error
}

// LimitOption provides configuration options for limit resolution
type LimitOption func(*LimitConfig)

// LimitConfig holds configuration for limit resolution
type LimitConfig struct {
	TreatZeroAsNil bool
}

// WithTreatZeroAsNil treats 0 values as nil (unlimited/disabled)
func WithTreatZeroAsNil() LimitOption {
	return func(c *LimitConfig) {
		c.TreatZeroAsNil = true
	}
}

// WithAllowUnlimited is a no-op since -1 values are already treated as unlimited by ApplyLimit
func WithAllowUnlimited() LimitOption {
	return func(c *LimitConfig) {
		// This option is a no-op as -1 values are already treated as unlimited
	}
}

// ThresholdCheckResult represents the result of a threshold evaluation
type ThresholdCheckResult struct {
	ShouldWarn     bool
	WithinLimit    bool
	CurrentUsage   uint64
	Threshold      *uint64
	Limit          *uint64
	DecisionReason models.QuotaCheckReason
}

// EvaluateThreshold evaluates threshold logic in a simplified way
// This consolidates the complex wouldHitThreshold/wouldCrossThreshold logic
func EvaluateThreshold(currentUsage, requestedBytes, threshold, limit uint64) ThresholdCheckResult {
	if threshold == 0 {
		// Threshold is 0, which means always warn
		// Check for overflow before calculating new usage
		withinLimit := true
		if currentUsage > math.MaxUint64-requestedBytes {
			withinLimit = false
		} else {
			withinLimit = currentUsage+requestedBytes <= limit
		}

		return ThresholdCheckResult{
			ShouldWarn:     true,
			WithinLimit:    withinLimit,
			CurrentUsage:   currentUsage,
			Threshold:      &threshold,
			Limit:          &limit,
			DecisionReason: models.QuotaCheckReasonWarningThreshold,
		}
	}

	newUsage := currentUsage + requestedBytes
	wouldExceedThreshold := newUsage > threshold
	wouldCrossThreshold := currentUsage < threshold && newUsage >= threshold
	withinLimit := newUsage <= limit

	shouldWarn := (wouldExceedThreshold || wouldCrossThreshold) && withinLimit

	var reason models.QuotaCheckReason
	if shouldWarn {
		reason = models.QuotaCheckReasonWarningThreshold
	} else {
		reason = models.QuotaCheckReasonOK
	}

	return ThresholdCheckResult{
		ShouldWarn:     shouldWarn,
		WithinLimit:    withinLimit,
		CurrentUsage:   currentUsage,
		Threshold:      &threshold,
		Limit:          &limit,
		DecisionReason: reason,
	}
}
