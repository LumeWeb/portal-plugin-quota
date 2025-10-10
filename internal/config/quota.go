package config

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/portal/config"
)

var _ config.ServiceConfig = (*QuotaConfig)(nil)

// QuotaConfig defines the configuration for the quota management service.
type QuotaConfig struct {
	// Enabled controls whether the quota service is active.
	Enabled bool `config:"enabled"`
	// DefaultEnforcementPolicy specifies how quota limits are enforced.
	// Valid values: HARD_LIMITS, UNLIMITED, ALLOWANCE, THRESHOLD.
	DefaultEnforcementPolicy string `config:"default_enforcement_policy"`
	// ReconciliationHour sets the hour (0-23) when quota reconciliation runs.
	ReconciliationHour int `config:"reconciliation_hour"`
	// HistoryRetentionDays specifies how long to retain quota history records.
	HistoryRetentionDays int `config:"history_retention_days"`
	// DetailedRetentionDays specifies how long to retain detailed usage records.
	DetailedRetentionDays int `config:"detailed_retention_days"`
	// EnableSharedUsage controls whether shared usage tracking is enabled.
	EnableSharedUsage bool `config:"enable_shared_usage"`
	// SharedUsagePrecision sets the decimal precision for shared usage calculations (0-10).
	SharedUsagePrecision int `config:"shared_usage_precision"`
	// DefaultQuotaPlanName specifies the quota plan assigned to new users.
	DefaultQuotaPlanName string `config:"default_quota_plan_name"`
}

func (s QuotaConfig) Schema() z.ZogSchema {
	return z.Struct(z.Shape{
		"Enabled":                  z.Bool(),
		"DefaultEnforcementPolicy": z.String().OneOf([]string{"HARD_LIMITS", "UNLIMITED", "ALLOWANCE", "THRESHOLD"}),
		"ReconciliationHour":       z.Int().GT(0).LTE(23),
		"HistoryRetentionDays":     z.Int().GT(0),
		"DetailedRetentionDays":    z.Int().GT(0),
		"EnableSharedUsage":        z.Bool(),
		"SharedUsagePrecision":     z.Int().GT(0).LTE(10),
		"DefaultQuotaPlanName":     z.String(),
	})
}

func (s QuotaConfig) Defaults() map[string]any {
	return map[string]any{
		"Enabled":                  true,
		"DefaultEnforcementPolicy": "HARD_LIMITS",
		"ReconciliationHour":       2,
		"HistoryRetentionDays":     90,
		"DetailedRetentionDays":    30,
		"EnableSharedUsage":        true,
		"SharedUsagePrecision":     2,
		"DefaultQuotaPlanName":     "default",
	}
}
