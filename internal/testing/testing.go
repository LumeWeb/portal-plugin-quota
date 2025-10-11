package testing

import (
	core2 "go.lumeweb.com/portal-plugin-quota/core"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.lumeweb.com/portal-plugin-quota/internal/config"
	"go.lumeweb.com/portal-plugin-quota/internal/db/migrations"
	"go.lumeweb.com/portal/core"
	testing2 "go.lumeweb.com/portal/core/testing"
)

// MockQuotaService is a minimal mock service for testing
type MockQuotaService struct {
	ctx core.Context
}

func (m *MockQuotaService) ID() string {
	return "quota"
}

func (m *MockQuotaService) Name() string {
	return "Quota Service"
}

func TestOptions() testing2.TestContextBuilderOption {
	// Default configuration for backward compatibility
	return TestOptionsWithConfig(&config.QuotaConfig{
		SharedUsagePrecision: 2,
	})
}

func TestOptionsWithConfig(config *config.QuotaConfig) testing2.TestContextBuilderOption {
	return testing2.CombineOptions(testing2.NewMockPluginBuilder(internal.PLUGIN_NAME).
		WithMigrations(core.DBMigration{core.DB_TYPE_SQLITE: migrations.GetSQLite()}).
		WithService(core2.QUOTA_SERVICE, func() (core.Service, []core.ContextBuilderOption, error) {
			return &MockQuotaService{}, nil, nil
		}).Build().BuilderOption(),
		testing2.WithServiceConfig(internal.PLUGIN_NAME, core2.QUOTA_SERVICE, config),
	)
}
