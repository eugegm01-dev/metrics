package main

import (
	"log"
	"time"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
	models "github.com/eugegm01-dev/metrics/internal/model"
)

func main() {
	// Инициализируем zap-логгер (как в сервере)
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	cfg, err := config.ParseAgentConfig()
	if err != nil {
		logger.Fatal("Failed to parse config", zap.Error(err))
	}

	// Выводим конфигурацию через логгер
	logger.Info("Agent started with configuration",
		zap.String("server_address", cfg.ServerAddr),
		zap.Duration("poll_interval", cfg.PollInterval),
		zap.Duration("report_interval", cfg.ReportInterval),
	)

	// Создаём один экземпляр агента — здесь живёт pollCount
	agentInstance := agent.NewAgent()

	pollTicker := time.NewTicker(cfg.PollInterval)
	reportTicker := time.NewTicker(cfg.ReportInterval)
	defer pollTicker.Stop()
	defer reportTicker.Stop()

	var currentMetrics []models.Metrics

	for {
		select {
		case <-pollTicker.C:
			logger.Debug("Poll tick → collecting metrics")
			currentMetrics = agentInstance.CollectMetrics() // собираем через экземпляр

		case <-reportTicker.C:
			logger.Debug("Report tick → sending metrics")
			if len(currentMetrics) > 0 {
				preparedMetrics := agentInstance.PrepareMetricsForSend(currentMetrics) // корректируем Delta
				// Используем новую функцию для батчевой отправки с retry
				if err := agent.SendMetricsBatch(cfg.ServerAddr, preparedMetrics); err != nil {
					logger.Error("Failed to send metrics to server after retries",
						zap.Error(err),
						zap.Int("metrics_count", len(preparedMetrics)),
					)
				} else {
					logger.Info("Metrics successfully sent as batch",
						zap.Int("metrics_count", len(preparedMetrics)),
					)
				}
				// PollCount накапливается, метрики не сбрасываем
			}
		}
	}
}
