package main

import (
	"os"

	"github.com/grafana/grafana-plugin-sdk-go/backend/app"
	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/grafana/grafana-chat-assistant/pkg/plugin"
)

func main() {
	if err := app.Manage("grafana-chat-assistant", plugin.NewApp, app.ManageOpts{}); err != nil {
		log.DefaultLogger.Error("Error starting plugin", "error", err.Error())
		os.Exit(1)
	}
}
