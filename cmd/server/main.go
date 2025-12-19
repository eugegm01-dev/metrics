package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/eugegm01-dev/metrics/internal/config"
	"go.uber.org/zap"
)

func main() {
	// создаём логгер
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	sugar := logger.Sugar()
	defer sugar.Sync()

	cfg, err := config.ParseAgentConfig()
	if err != nil {
		sugar.Fatalw("Failed to parse config", "error", err)
	}

	sugar.Infow("Starting agent",
		"server_address", cfg.ServerAddr,
		"poll_interval", cfg.PollInterval,
		"report_interval", cfg.ReportInterval,
	)

	fmt.Printf("Starting agent with config:\n")
	fmt.Printf("  Server address: %s\n", cfg.ServerAddr)
	fmt.Printf("  Poll interval: %v\n", cfg.PollInterval)
	fmt.Printf("  Report interval: %v\n\n", cfg.ReportInterval)

	pollTicker := time.NewTicker(cfg.PollInterval)
	reportTicker := time.NewTicker(cfg.ReportInterval)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	for {
		select {
		case <-pollTicker.C:
			sugar.Debug("Poll tick - collecting metrics")

		case <-reportTicker.C:
			sugar.Info("Report tick - sending metrics")
			testConnection(cfg.ServerAddr, sugar)
		}
	}
}

func testConnection(serverAddr string, sugar *zap.SugaredLogger) {
	url := "http://" + serverAddr + "/"

	start := time.Now()
	resp, err := http.Get(url)
	duration := time.Since(start)

	if err != nil {
		sugar.Errorw("Connection error",
			"url", url,
			"error", err,
			"duration", duration,
		)
		return
	}
	defer resp.Body.Close()

	sugar.Infow("Server responded",
		"url", url,
		"status", resp.StatusCode,
		"duration", duration,
	)
}
