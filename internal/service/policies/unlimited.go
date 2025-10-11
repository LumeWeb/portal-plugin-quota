package policies

import (
	"time"

	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

// UnlimitedPolicyEnforcer implements PolicyEnforcer for the UNLIMITED policy
type UnlimitedPolicyEnforcer struct {
	*BasePolicyEnforcer
}

// NewUnlimitedPolicyEnforcer creates a new unlimited policy enforcer
func NewUnlimitedPolicyEnforcer(ctx core.Context) *UnlimitedPolicyEnforcer {
	return &UnlimitedPolicyEnforcer{
		BasePolicyEnforcer: NewBasePolicyEnforcer(ctx),
	}
}

// CheckUploadQuota always allows uploads since there are no limits
func (u *UnlimitedPolicyEnforcer) CheckUploadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := u.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := u.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return u.createSuccessResult(models.EnforcementPolicyUnlimited), nil
}

// CheckDownloadQuota always allows downloads since there are no limits
func (u *UnlimitedPolicyEnforcer) CheckDownloadQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := u.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := u.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return u.createSuccessResult(models.EnforcementPolicyUnlimited), nil
}

// CheckStorageQuota always allows storage since there are no limits
func (u *UnlimitedPolicyEnforcer) CheckStorageQuota(config *models.UserQuotaConfig, requestedBytes uint64) (pluginCore.QuotaCheckResult, error) {
	if err := u.validateRequestedBytes(requestedBytes); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}
	if err := u.validateUserID(config.UserID); err != nil {
		return pluginCore.QuotaCheckResult{}, err
	}

	return u.createSuccessResult(models.EnforcementPolicyUnlimited), nil
}

// RecordUpload simply records usage without any limit checking
func (u *UnlimitedPolicyEnforcer) RecordUpload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := u.validateUserID(userID); err != nil {
		return err
	}

	if err := u.validateBytes(bytes); err != nil {
		return err
	}

	// Record detailed usage
	detail := &models.UserUsageDetail{
		UserID:     userID,
		UploadID:   uploadID,
		Type:       models.UsageTypeUpload,
		Bytes:      bytes,
		IP:         ip,
		Timestamp:  time.Now(),
		SharedWith: 1, // Uploads are not shared
	}

	if err := u.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily aggregated usage
	return u.updateDailyUsage(userID, models.UsageTypeUpload, int64(bytes))
}

// RecordDownload simply records usage without any limit checking
func (u *UnlimitedPolicyEnforcer) RecordDownload(userID, uploadID uint, bytes uint64, ip string) error {
	if err := u.validateUserID(userID); err != nil {
		return err
	}

	if err := u.validateBytes(bytes); err != nil {
		return err
	}

	// Record detailed usage
	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      models.UsageTypeDownload,
		Bytes:     bytes,
		IP:        ip,
		Timestamp: time.Now(),
	}

	if err := u.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily aggregated usage
	return u.updateDailyUsage(userID, models.UsageTypeDownload, int64(bytes))
}

// RecordStorageChange simply records usage without any limit checking
func (u *UnlimitedPolicyEnforcer) RecordStorageChange(userID, uploadID uint, bytes int64, ip string) error {
	if err := u.validateUserID(userID); err != nil {
		return err
	}

	if bytes == 0 {
		return models.ErrInvalidBytes
	}

	// Record detailed usage
	var usageType models.UsageType
	var recordBytes uint64
	if bytes > 0 {
		usageType = models.UsageTypeStorageAdd
		recordBytes = uint64(bytes)
	} else {
		usageType = models.UsageTypeStorageRemove
		recordBytes = uint64(-bytes)
	}

	detail := &models.UserUsageDetail{
		UserID:    userID,
		UploadID:  uploadID,
		Type:      usageType,
		Bytes:     recordBytes,
		IP:        ip,
		Timestamp: time.Now(),
	}

	if err := u.recordUserUsageDetail(detail); err != nil {
		return err
	}

	// Update daily aggregated usage
	return u.updateDailyUsage(userID, usageType, bytes)
}

// GetDetailedUsage delegates to the base enforcer
func (u *UnlimitedPolicyEnforcer) GetDetailedUsage(userID uint, start, end time.Time) ([]*models.UserUsageDetail, error) {
	return u.getDetailedUsage(userID, start, end)
}

// GetCurrentUsage delegates to the base enforcer
func (u *UnlimitedPolicyEnforcer) GetCurrentUsage(userID uint) (*pluginCore.Usage, error) {
	return u.getCurrentUsage(userID)
}

// GetUsageHistory delegates to the base enforcer
func (u *UnlimitedPolicyEnforcer) GetUsageHistory(userID uint, period int, usageType pluginCore.UsageType) ([]*pluginCore.UsagePoint, error) {
	return u.getUsageHistory(userID, period, models.UsageType(usageType))
}
