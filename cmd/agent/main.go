package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
	models "github.com/eugegm01-dev/metrics/internal/model"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	cfg, err := config.ParseAgentConfig()
	if err != nil {
		logger.Fatal("Failed to parse config", zap.Error(err))
	}

	logger.Info("Agent started with configuration",
		zap.String("server_address", cfg.ServerAddr),
		zap.Duration("poll_interval", cfg.PollInterval),
		zap.Duration("report_interval", cfg.ReportInterval),
	)

	agentInstance := agent.NewAgent()

	pollTicker := time.NewTicker(cfg.PollInterval)
	reportTicker := time.NewTicker(cfg.ReportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	var (
		currentMetrics []models.Metrics
		mu             sync.RWMutex
	)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutdown signal received, performing final send if needed")
			// финальная отправка: копируем метрики под локом и отправляем без блокировки
			mu.RLock()
			finalCopy := make([]models.Metrics, len(currentMetrics))
			copy(finalCopy, currentMetrics)
			mu.RUnlock()

			if len(finalCopy) > 0 {
				prepared := agentInstance.PrepareMetricsForSend(finalCopy)
				if err := agent.SendMetricsBatch(context.Background(), cfg.ServerAddr, prepared); err != nil {
					logger.Error("Failed to send final metrics", zap.Error(err))
				} else {
					logger.Info("Final metrics sent", zap.Int("metrics_count", len(prepared)))
				}
			}
			return

		case <-pollTicker.C:
			logger.Debug("Poll tick → collecting metrics")
			collected := agentInstance.CollectMetrics()
			mu.Lock()
			currentMetrics = collected
			mu.Unlock()

		case <-reportTicker.C:
			logger.Debug("Report tick → sending metrics")
			// копируем под RLock, чтобы не держать лок во время сетевых операций
			mu.RLock()
			if len(currentMetrics) == 0 {
				mu.RUnlock()
				continue
			}
			toSend := make([]models.Metrics, len(currentMetrics))
			copy(toSend, currentMetrics)
			mu.RUnlock()

			preparedMetrics := agentInstance.PrepareMetricsForSend(toSend)
			if err := agent.SendMetricsBatch(ctx, cfg.ServerAddr, preparedMetrics); err != nil {
				logger.Error("Failed to send metrics to server after retries", zap.Error(err))
			} else {
				logger.Info("Metrics successfully sent as batch",
					zap.Int("metrics_count", len(preparedMetrics)),
				)
			}
		}
	}
}
