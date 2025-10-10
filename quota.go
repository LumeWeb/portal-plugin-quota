package main

import (
	"go.lumeweb.com/portal-plugin-quota/build"
	"go.lumeweb.com/portal-plugin-quota/internal"
	"go.lumeweb.com/portal-plugin-quota/internal/db/migrations"
	"go.lumeweb.com/portal-plugin-quota/internal/db/models"
	"go.lumeweb.com/portal/core"
)

func init() {
	core.RegisterPlugin(core.PluginInfo{
		ID:      internal.PLUGIN_NAME,
		Version: build.GetInfo(),
		Depends: []string{},
		Meta: func(ctx core.Context, builder core.PortalMetaBuilder) error {
			return nil
		},
		API: func() (core.API, []core.ContextBuilderOption, error) {
			return nil, nil, nil
		},
		Services: func() ([]core.ServiceInfo, error) {
			return []core.ServiceInfo{}, nil
		},
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
	})
}
