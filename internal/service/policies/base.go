package policies

import (
	"fmt"

	"github.com/docker/go-units"
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
	"gorm.io/gorm"
)

// BasePolicyEnforcer provides common functionality for all policy enforcers
type BasePolicyEnforcer struct {
	ctx          core.Context
	db           *gorm.DB
	logger       *core.Logger
	usageManager pluginCore.UsageManager
}

// NewBasePolicyEnforcer creates a new base policy enforcer
func NewBasePolicyEnforcer(ctx core.Context, usageManager pluginCore.UsageManager) *BasePolicyEnforcer {
	return &BasePolicyEnforcer{
		ctx:          ctx,
		db:           ctx.DB(),
		logger:       ctx.NamedLogger("quota.BasePolicyEnforcer"),
		usageManager: usageManager,
	}
}

// Common validation methods

// validateUserID validates that a user ID is valid
func (b *BasePolicyEnforcer) validateUserID(userID uint) error {
	if userID == 0 {
		return models.ErrInvalidUserID
	}
	return nil
}

// validateRequestedBytes validates that requested bytes is valid
func (b *BasePolicyEnforcer) validateRequestedBytes(requestedBytes uint64) error {
	if requestedBytes == 0 {
		return models.ErrInvalidBytes
	}
	return nil
}

// validateRecordParams validates common parameters for upload/download recording
func (b *BasePolicyEnforcer) validateRecordParams(userID, uploadID uint, bytes uint64) error {
	if err := b.validateUserID(userID); err != nil {
		return err
	}
	if uploadID == 0 {
		return models.ErrInvalidUploadID
	}
	return b.validateRequestedBytes(bytes)
}

// validateStorageRecordParams validates common parameters for storage recording
func (b *BasePolicyEnforcer) validateStorageRecordParams(userID, uploadID uint, bytes int64) error {
	if err := b.validateUserID(userID); err != nil {
		return err
	}
	if uploadID == 0 {
		return models.ErrInvalidUploadID
	}
	if bytes == 0 {
		return models.ErrZeroBytes
	}
	return nil
}

// delegateRecordUpload delegates to usageManager.RecordUpload after validation
func (b *BasePolicyEnforcer) delegateRecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := b.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}
	return b.usageManager.RecordUpload(userID, uploadID, bytes, ip)
}

// delegateRecordDownload delegates to usageManager.RecordDownload after validation
func (b *BasePolicyEnforcer) delegateRecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := b.validateRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}
	return b.usageManager.RecordDownload(userID, uploadID, bytes, ip)
}

// delegateRecordStorageChange delegates to usageManager.RecordStorageChange after validation
func (b *BasePolicyEnforcer) delegateRecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := b.validateStorageRecordParams(userID, uploadID, bytes); err != nil {
		return err
	}
	return b.usageManager.RecordStorageChange(userID, uploadID, bytes, ip)
}

// QuotaResultBuilder helps build quota check results with a fluent interface
type QuotaResultBuilder struct {
	allowed       bool
	reason        pluginCore.QuotaCheckReason
	policy        models.EnforcementPolicy
	currentUsage  uint64
	limit         *uint64
	threshold     *uint64
	allowance     *uint64
	allowanceUsed *uint64
}

// NewResultBuilder creates a new QuotaResultBuilder
func (b *BasePolicyEnforcer) NewResultBuilder() *QuotaResultBuilder {
	return &QuotaResultBuilder{}
}

// Allowed sets whether the operation is allowed
func (b *QuotaResultBuilder) Allowed(allowed bool) *QuotaResultBuilder {
	b.allowed = allowed
	return b
}

// Reason sets the quota check reason
func (b *QuotaResultBuilder) Reason(reason pluginCore.QuotaCheckReason) *QuotaResultBuilder {
	b.reason = reason
	return b
}

// Policy sets the enforcement policy
func (b *QuotaResultBuilder) Policy(policy models.EnforcementPolicy) *QuotaResultBuilder {
	b.policy = policy
	return b
}

// CurrentUsage sets the current usage value
func (b *QuotaResultBuilder) CurrentUsage(usage uint64) *QuotaResultBuilder {
	b.currentUsage = usage
	return b
}

// Limit sets the limit value
func (b *QuotaResultBuilder) Limit(limit uint64) *QuotaResultBuilder {
	b.limit = &limit
	return b
}

// Threshold sets the threshold value
func (b *QuotaResultBuilder) Threshold(threshold uint64) *QuotaResultBuilder {
	b.threshold = &threshold
	return b
}

// Allowance sets the allowance value
func (b *QuotaResultBuilder) Allowance(allowance uint64) *QuotaResultBuilder {
	b.allowance = &allowance
	return b
}

// AllowanceUsed sets the used allowance value
func (b *QuotaResultBuilder) AllowanceUsed(used uint64) *QuotaResultBuilder {
	b.allowanceUsed = &used
	return b
}

// Build constructs the final QuotaCheckResult
func (b *QuotaResultBuilder) Build() pluginCore.QuotaCheckResult {
	details := pluginCore.QuotaCheckDetails{
		Policy:        b.policy,
		CurrentUsage:  b.currentUsage,
		Limit:         b.limit,
		Threshold:     b.threshold,
		Allowance:     b.allowance,
		AllowanceUsed: b.allowanceUsed,
	}

	return pluginCore.QuotaCheckResult{
		Allowed: b.allowed,
		Reason:  b.reason,
		Details: details,
	}
}

// createQuotaCheckResult creates a standard quota check result
func (b *BasePolicyEnforcer) createQuotaCheckResult(allowed bool, reason pluginCore.QuotaCheckReason, policy models.EnforcementPolicy, details pluginCore.QuotaCheckDetails) pluginCore.QuotaCheckResult {
	return pluginCore.QuotaCheckResult{
		Allowed: allowed,
		Reason:  reason,
		Details: details,
	}
}

// createSuccessResult creates a success quota check result
func (b *BasePolicyEnforcer) createSuccessResult(policy models.EnforcementPolicy) pluginCore.QuotaCheckResult {
	return b.NewResultBuilder().
		Allowed(true).
		Reason(models.QuotaCheckReasonOK).
		Policy(policy).
		Build()
}

// createFailureResult creates a failure quota check result
func (b *BasePolicyEnforcer) createFailureResult(reason pluginCore.QuotaCheckReason, policy models.EnforcementPolicy, details pluginCore.QuotaCheckDetails) pluginCore.QuotaCheckResult {
	return b.createQuotaCheckResult(false, reason, policy, details)
}

// createWarningResult creates a warning quota check result (allowed but with warning)
func (b *BasePolicyEnforcer) createWarningResult(policy models.EnforcementPolicy, currentUsage uint64, threshold, limit uint64) pluginCore.QuotaCheckResult {
	return b.NewResultBuilder().
		Allowed(true).
		Reason(models.QuotaCheckReasonWarningThreshold).
		Policy(policy).
		CurrentUsage(currentUsage).
		Threshold(threshold).
		Limit(limit).
		Build()
}

// applyLimit sets a limit field if it passes validation
func (b *BasePolicyEnforcer) applyLimit(dest **uint64, source int64, limitName string) error {
	return b.applyLimitWithOptions(dest, source, limitName, false)
}

// applyLimitWithOptions sets a limit field if it passes validation with options
func (b *BasePolicyEnforcer) applyLimitWithOptions(dest **uint64, source int64, limitName string, treatZeroAsNil bool) error {
	// Allow -1 (unlimited), 0 (disabled), and positive values
	if source < -1 {
		return fmt.Errorf("invalid %s: %d (must be -1, 0, or positive)", limitName, source)
	}

	// Check if the value is unreasonably large (1 PiB should be enough for most use cases)
	if source > 0 && source > int64(units.PiB) {
		return fmt.Errorf("limit value %d is unreasonably large", source)
	}

	var convertedValue *uint64
	if source == -1 || (treatZeroAsNil && source == 0) {
		convertedValue = nil // unlimited or disabled (treated as nil)
	} else {
		converted := uint64(source)
		convertedValue = &converted
	}

	*dest = convertedValue
	return nil
}
