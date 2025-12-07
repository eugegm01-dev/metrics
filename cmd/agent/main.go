package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/eugegm01-dev/metrics/internal/config"
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

	for {
		select {
		case <-pollTicker.C:
			fmt.Println("[DEBUG] Poll tick - collecting metrics")

		case <-reportTicker.C:
			fmt.Println("[DEBUG] Report tick - sending metrics")
			testConnection(cfg.ServerAddr)
		}
	}
}

func testConnection(serverAddr string) {
	url := "http://" + serverAddr + "/"
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("  Connection error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("  Server responded with status: %d\n", resp.StatusCode)
}
