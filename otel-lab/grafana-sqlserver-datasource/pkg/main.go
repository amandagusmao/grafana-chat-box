package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/datasource"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-sqlserver-datasource/pkg/plugin"
)

func main() {
	if err := datasource.Manage("grafana-sqlserver-datasource", plugin.NewDatasource, datasource.ManageOpts{}); err != nil {
		log.DefaultLogger.Error("Error starting plugin", "error", err.Error())
		os.Exit(1)
	}
}
