package app

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/model" // Добавьте этот импорт
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
		zap.String("key", cfg.Key), // Логируем наличие ключа
	)

	agentInstance := agent.NewAgent()

	metricsChan := make(chan []agent.Metric, 100)
	done := make(chan struct{})

	// Горутина 1: сбор runtime метрик
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				metrics := agentInstance.CollectRuntimeMetrics()
				metricsChan <- metrics
			}
		}
	}()

	// Горутина 2: сбор системных метрик через gopsutil
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				metrics := agentInstance.CollectSystemMetrics()
				if len(metrics) > 0 {
					metricsChan <- metrics
				}
			}
		}
	}()

	// Пул воркеров для отправки метрик
	for i := 0; i < cfg.RateLimit; i++ {
		go func(workerID int) {
			for metrics := range metricsChan {
				prepared := agentInstance.PrepareMetricsForSend(metrics)
				// Преобразуем agent.Metric в model.Metrics
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
				// Передаем ключ в функцию отправки
				if err := agent.SendMetricsBatch(cfg.ServerAddr, modelMetrics, cfg.Key); err != nil {
					logger.Error("Failed to send metrics",
						zap.Int("worker", workerID),
						zap.Error(err),
					)
				}
			}
		}(i)
	}

	// Ожидание сигнала завершения
	<-done
	return nil
}
