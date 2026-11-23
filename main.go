package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/divkix/Alita_Robot/alita/app"
	"github.com/divkix/Alita_Robot/alita/config"
)

//go:embed locales
var Locales embed.FS

func main() {
	// Health check mode for Docker healthcheck (distroless images have no curl/wget)
	if len(os.Args) > 1 && (os.Args[1] == "--health" || os.Args[1] == "-health") {
		app.HealthCheck()
	}

	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-version" || os.Args[1] == "-v") {
		version := config.AppConfig.BotVersion
		if version == "" {
			version = "v2.30.1"
		}
		fmt.Println(version)
		os.Exit(0)
	}

	mode := "polling"
	if config.AppConfig.UseWebhooks {
		mode = "webhook"
	}
	app.Locales = Locales
	app.Run(mode)
}
