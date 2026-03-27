package quota

import (
	pluginplugin "go.lumeweb.com/portal-plugin-quota/internal/plugin"
	"go.lumeweb.com/portal/core"
)

func init() {
	core.RegisterPlugin(pluginplugin.GetPluginInfo())
}
