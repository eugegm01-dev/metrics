package main

import (
	"fmt"
	"log"
	"time"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
	models "github.com/eugegm01-dev/metrics/internal/model"
)

func main() {
	cfg, err := config.ParseAgentConfig()
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}

	fmt.Printf("Starting agent with config:\n")
	fmt.Printf("  Server address: %s\n", cfg.ServerAddr)
	fmt.Printf("  Poll interval: %v\n", cfg.PollInterval)
	fmt.Printf("  Report interval: %v\n\n", cfg.ReportInterval)

	pollTicker := time.NewTicker(cfg.PollInterval)
	reportTicker := time.NewTicker(cfg.ReportInterval)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	var metrics []models.Metrics

	for {
		select {
		case <-pollTicker.C:
			fmt.Println("[DEBUG] Poll tick - collecting metrics")
			metrics = agent.CollectMetrics()

		case <-reportTicker.C:
			fmt.Println("[DEBUG] Report tick - sending metrics")
			if len(metrics) > 0 {
				if err := agent.SendMetrics(cfg.ServerAddr, metrics); err != nil {
					fmt.Printf("  Failed to send metrics: %v\n", err)
				} else {
					fmt.Printf("  Successfully sent %d metrics\n", len(metrics))
				}
			}
		}
	}
}
