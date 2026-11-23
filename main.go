package main

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/divkix/Alita_Robot/alita/app"
	"github.com/divkix/Alita_Robot/alita/config"
	"github.com/divkix/Alita_Robot/alita/utils/constants"
)

//go:embed locales
var Locales embed.FS

func main() {
	// Health check mode for Docker healthcheck (distroless images have no curl/wget)
	if len(os.Args) > 1 && (os.Args[1] == "--health" || os.Args[1] == "-health") {
		healthPort := healthCheckPort()
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", healthPort))
		if err != nil {
			os.Exit(1)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
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

func healthCheckPort() int {
	for _, name := range []string{"HTTP_PORT", "PORT"} {
		value := os.Getenv(name)
		if value == "" {
			continue
		}
		port, err := strconv.Atoi(value)
		if err == nil && port > 0 && port <= 65535 {
			return port
		}
		break
	}
	if config.AppConfig != nil && config.AppConfig.HTTPPort > 0 {
		return config.AppConfig.HTTPPort
	}
	return constants.DefaultHTTPPort
}
