package config

import (
	z "github.com/Oudwins/zog"
	"go.lumeweb.com/portal/config"
)

var _ config.ServiceConfig = (*ServiceConfig)(nil)

type ServiceConfig struct {
	Enabled                   bool   `config:"enabled"`
	DefaultEnforcementPolicy   string `config:"default_enforcement_policy"`
	ReconciliationHour        int    `config:"reconciliation_hour"`
	HistoryRetentionDays      int    `config:"history_retention_days"`
	DetailedRetentionDays     int    `config:"detailed_retention_days"`
	EnableSharedUsage         bool   `config:"enable_shared_usage"`
	SharedUsagePrecision      int    `config:"shared_usage_precision"`
	DefaultQuotaPlanName      string `config:"default_quota_plan_name"`
}

func (s ServiceConfig) Schema() z.ZogSchema {
	return z.Struct(z.Shape{
		"Enabled":                   z.Bool(),
		"DefaultEnforcementPolicy":  z.String(),
		"ReconciliationHour":        z.Int(),
		"HistoryRetentionDays":      z.Int(),
		"DetailedRetentionDays":     z.Int(),
		"EnableSharedUsage":         z.Bool(),
		"SharedUsagePrecision":      z.Int(),
		"DefaultQuotaPlanName":      z.String(),
	})
}

func (s ServiceConfig) Defaults() map[string]any {
	return map[string]any{
		"Enabled":                   true,
		"DefaultEnforcementPolicy":  "HARD_LIMITS",
		"ReconciliationHour":        2,
		"HistoryRetentionDays":      90,
		"DetailedRetentionDays":     30,
		"EnableSharedUsage":         true,
		"SharedUsagePrecision":      2,
		"DefaultQuotaPlanName":      "default",
	}
}
