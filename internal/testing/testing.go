package testing

import (
	pluginCore "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/migrations"
	"go.lumeweb.com/portal/core"
	coreTesting "go.lumeweb.com/portal/core/testing"
)

func TestOptions() coreTesting.TestContextBuilderOption {
	// Default configuration for backward compatibility
	return TestOptionsWithConfig(&config.QuotaConfig{
		SharedUsagePrecision: 2,
	})
}

func TestOptionsWithConfig(config *config.QuotaConfig) coreTesting.TestContextBuilderOption {
	return coreTesting.CombineOptions(coreTesting.NewMockPluginBuilder(internal.PLUGIN_NAME).
		WithMigrations(core.DBMigration{core.DB_TYPE_SQLITE: migrations.GetSQLite()}).
		WithMockServiceFactory(pluginCore.QUOTA_SERVICE, pluginCore.NewMockQuotaService).BuilderOption(),
		coreTesting.WithServiceConfig(internal.PLUGIN_NAME, pluginCore.QUOTA_SERVICE, config),
	)
}
