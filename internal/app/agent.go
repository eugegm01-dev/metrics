package app

import (
	"time"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
)

func RunAgent() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return err
	}
	defer logger.Sync()

	cfg, err := config.ParseAgentConfig()
	if err != nil {
		return err
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

	for {
		select {
		case <-pollTicker.C:
			logger.Debug("Poll tick → collecting metrics")
			agentInstance.CollectMetrics()

		case <-reportTicker.C:
			logger.Debug("Report tick → sending metrics")
			metrics := agentInstance.GetCurrentMetrics()
			if len(metrics) > 0 {
				preparedMetrics := agentInstance.PrepareMetricsForSend(metrics)
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
			}
		}
	}
}
