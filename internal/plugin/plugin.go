package plugin

import (
	"github.com/prometheus/client_golang/prometheus"

	"go.lumeweb.com/portal-plugin-quota/build"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.lumeweb.com/portal-plugin-quota/internal/api"
	"go.lumeweb.com/portal-plugin-quota/internal/db/migrations"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal-plugin-quota/internal/service/policies"
	quota_service "go.lumeweb.com/portal-plugin-quota/internal/service/quota"
	"go.lumeweb.com/portal/core"
)

func GetPluginMetrics() []prometheus.Collector {
	return append(
		quota_service.GetCollectors(),
		policies.GetCollectors()...,
	)
}

func GetPluginInfo() core.PluginInfo {
	return core.PluginInfo{
		ID:      internal.PluginName,
		Version: build.GetInfo(),
		Depends: []string{"dashboard"},
		Meta: func(ctx core.Context, builder core.PortalMetaBuilder) error {
			return nil
		},
		API: func() (core.API, []core.ContextBuilderOption, error) {
			return nil, nil, nil
		},
		Services: func() ([]core.ServiceInfo, error) {
			return []core.ServiceInfo{
				{ID: internal.PluginName, Factory: quota_service.NewQuotaService},
			}, nil
		},
		APIExtensions: func(core.Context) ([]core.APIExtensionFactory, error) {
			return []core.APIExtensionFactory{
				api.NewQuotaAdminExtension(),
				api.NewQuotaExtension(),
			}, nil
		},
		Metrics: GetPluginMetrics(),
		Models: []any{
			&models.UserQuota{},
			&models.UserUsageDetail{},
			&models.QuotaPlan{},
			&models.UserQuotaConfig{},
			&models.AllowanceGrant{},
			&models.AllowanceConsumption{},
		},

		Migrations: core.DBMigration{
			core.DB_TYPE_MYSQL:  migrations.GetMySQL(),
			core.DB_TYPE_SQLITE: migrations.GetSQLite(),
		},
	}
}
