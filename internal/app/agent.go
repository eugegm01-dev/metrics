package app

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/model"
)

func RunAgent() error {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Sync()

	cfg, err := config.ParseAgentConfig()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	logger.Info("Agent started",
		zap.String("server", cfg.ServerAddr),
		zap.Duration("poll", cfg.PollInterval),
		zap.Duration("report", cfg.ReportInterval),
		zap.Int("rate_limit", cfg.RateLimit),
		zap.String("key", cfg.Key),
	)

	agentInstance := agent.NewAgent(cfg.RateLimit)

	// Запускаем воркеры
	agentInstance.StartWorkers(func(metrics []agent.Metric) {
		prepared := agentInstance.PrepareMetricsForSend(metrics)
		var modelMetrics []model.Metrics
		for _, m := range prepared {
			metric := model.Metrics{
				ID:    m.ID,
				MType: m.MType,
			}
			switch m.MType {
			case model.Gauge:
				value := m.Value
				metric.Value = &value
			case model.Counter:
				delta := m.Delta
				metric.Delta = &delta
			}
			modelMetrics = append(modelMetrics, metric)
		}

		if err := agent.SendMetricsBatch(cfg.ServerAddr, modelMetrics, cfg.Key); err != nil {
			logger.Error("Failed to send metrics",
				zap.Error(err),
			)
		}
	})
	defer agentInstance.StopWorkers()

	// Горутина 1: сбор runtime метрик
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		for range ticker.C {
			metrics := agentInstance.CollectRuntimeMetrics()
			agentInstance.SendMetrics(metrics)
		}
	}()

	// Горутина 2: сбор системных метрик через gopsutil
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		for range ticker.C {
			metrics := agentInstance.CollectSystemMetrics()
			if len(metrics) > 0 {
				agentInstance.SendMetrics(metrics)
			}
		}
	}()

	// Ожидание завершения (можно добавить обработку сигналов)
	select {}
}
