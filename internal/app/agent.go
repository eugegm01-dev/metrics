// Package app contains the main application logic for the agent and server.
package app

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
	"github.com/eugegm01-dev/metrics/internal/model"
)

// Agent encapsulates the metrics agent with graceful shutdown
type Agent struct {
	instance       *agent.Agent
	cfg            *config.AgentConfig
	logger         *zap.Logger
	shutdownChan   chan struct{}
	metricsChan    chan []agent.Metric
	workerDoneChan chan struct{}
}

// RunAgent запускает агент с конфигурацией из флагов и переменных окружения.
// Периодически собирает и отправляет метрики на сервер.
func RunAgent() error {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.FatalLevel))
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.ParseAgentConfig()
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.CryptoKey != "" {
		if err := agent.InitAgentCrypto(cfg.CryptoKey); err != nil {
			return fmt.Errorf("failed to init agent crypto: %w", err)
		}
		logger.Info("Agent crypto initialized", zap.String("key_path", cfg.CryptoKey))
	}

	logger.Info("Agent started",
		zap.String("server", cfg.ServerAddr),
		zap.Duration("poll", cfg.PollInterval),
		zap.Duration("report", cfg.ReportInterval),
		zap.Int("rate_limit", cfg.RateLimit),
	)

	agentInstance := agent.NewAgent(cfg.RateLimit)

	// Канал для метрик
	metricsChan := make(chan []agent.Metric, 100)

	// Запускаем воркеры агента (они будут обрабатывать метрики и отправлять)
	agentInstance.StartWorkers(func(metrics []agent.Metric) {
		select {
		case metricsChan <- metrics:
		default:
			logger.Warn("Metrics channel full, dropping batch")
		}
	})

	// Горутина для отправки метрик из канала
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case metrics := <-metricsChan:
				sendMetricsBatch(ctx, agentInstance, cfg, metrics, logger)
			case <-ctx.Done():
				// Дренируем канал перед выходом
				for {
					select {
					case metrics := <-metricsChan:
						sendMetricsBatch(context.Background(), agentInstance, cfg, metrics, logger)
					default:
						return
					}
				}
			}
		}
	}()

	// Горутина сбора runtime метрик
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				metrics := agentInstance.CollectRuntimeMetrics()
				agentInstance.SendMetrics(metrics)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Горутина сбора системных метрик
	go func() {
		ticker := time.NewTicker(cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				metrics := agentInstance.CollectSystemMetrics()
				if len(metrics) > 0 {
					agentInstance.SendMetrics(metrics)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Ожидание сигнала завершения
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigChan

	logger.Info("Shutdown signal received, stopping agent...")
	cancel() // сигнализируем горутинам остановиться

	// Даем время на отправку оставшихся метрик
	time.Sleep(2 * time.Second)

	agentInstance.StopWorkers()
	close(metricsChan)

	logger.Info("Agent shutdown completed")
	return nil
}

// runWithGracefulShutdown запускает агент с корректной обработкой сигналов
func (a *Agent) runWithGracefulShutdown() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	a.shutdownChan = make(chan struct{})
	a.workerDoneChan = make(chan struct{})

	// Start workers with proper processing function
	a.instance.StartWorkers(func(metrics []agent.Metric) {
		select {
		case a.metricsChan <- metrics:
		default:
			a.logger.Warn("Metrics channel full, dropping batch")
		}
	})

	go a.collectRuntimeMetricsLoop()
	go a.collectSystemMetricsLoop()

	<-ctx.Done()
	a.logger.Info("Shutdown signal received, draining metrics...")

	close(a.shutdownChan)

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer drainCancel()

	select {
	case <-a.workerDoneChan:
	case <-drainCtx.Done():
		a.logger.Warn("Drain timeout, forcing stop")
	}

	a.instance.StopWorkers()
	close(a.metricsChan)

	a.logger.Info("Agent shutdown completed")
	return nil
}

// sendMetricsLoop continuously sends metrics from the channel
func (a *Agent) sendMetricsLoop() {
	defer close(a.workerDoneChan)

	for {
		select {
		case metrics, ok := <-a.metricsChan:
			if !ok {
				return
			}
			sendMetricsBatch(context.Background(), a.instance, a.cfg, metrics, a.logger)
		case <-a.shutdownChan:
			// Drain channel before shutdown
			a.drainAndSendMetrics()
			return
		}
	}
}

// drainAndSendMetrics sends all remaining metrics in the channel
func (a *Agent) drainAndSendMetrics() {
	a.logger.Info("Draining metrics channel before shutdown")

	// Collect all pending metrics
	var allMetrics []agent.Metric

	// Get metrics from channel with timeout
	drainTimeout := time.After(5 * time.Second)

	for {
		select {
		case metrics, ok := <-a.metricsChan:
			if !ok {
				goto send
			}
			allMetrics = append(allMetrics, metrics...)
		case <-drainTimeout:
			a.logger.Warn("Drain timeout reached, sending what we have")
			goto send
		default:
			// No more metrics available
			goto send
		}
	}

send:
	if len(allMetrics) > 0 {
		a.logger.Info("Sending final metrics batch", zap.Int("count", len(allMetrics)))
		sendMetricsBatch(context.Background(), a.instance, a.cfg, allMetrics, a.logger)
	}
}

// sendMetricsBatch sends a batch of metrics with retry
func sendMetricsBatch(ctx context.Context, agentInstance *agent.Agent, cfg *config.AgentConfig, metrics []agent.Metric, logger *zap.Logger) {
	if len(metrics) == 0 {
		return
	}
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

	if err := agent.SendMetricsBatchWithContext(ctx, cfg.ServerAddr, modelMetrics, cfg.Key); err != nil {
		logger.Error("Failed to send metrics batch",
			zap.Int("count", len(modelMetrics)),
			zap.Error(err))
	} else {
		logger.Debug("Metrics batch sent successfully", zap.Int("count", len(modelMetrics)))
	}
}

// collectRuntimeMetricsLoop collects runtime metrics periodically
func (a *Agent) collectRuntimeMetricsLoop() {
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			metrics := a.instance.CollectRuntimeMetrics()
			a.instance.SendMetrics(metrics)
		case <-a.shutdownChan:
			return
		}
	}
}

func (a *Agent) collectSystemMetricsLoop() {
	ticker := time.NewTicker(a.cfg.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			metrics := a.instance.CollectSystemMetrics()
			if len(metrics) > 0 {
				a.instance.SendMetrics(metrics)
			}
		case <-a.shutdownChan:
			return
		}
	}
}

// gracefulShutdown performs graceful shutdown of the agent
func (a *Agent) gracefulShutdown() error {
	a.logger.Info("Starting agent graceful shutdown")

	// Step 1: Signal shutdown to all goroutines
	close(a.shutdownChan)

	// Step 2: Wait for worker goroutines to finish (with timeout)
	a.logger.Info("Waiting for workers to finish")
	select {
	case <-a.workerDoneChan:
		a.logger.Info("All workers stopped gracefully")
	case <-time.After(10 * time.Second):
		a.logger.Warn("Worker shutdown timeout, forcing stop")
	}

	// Step 3: Stop agent workers
	a.instance.StopWorkers()

	// Step 4: Close metrics channel
	close(a.metricsChan)

	a.logger.Info("Agent shutdown completed successfully")
	return nil
}

// getAgentSignalDescription returns human-readable signal description
func getAgentSignalDescription(sig os.Signal) string {
	switch sig {
	case syscall.SIGTERM:
		return "Termination signal (SIGTERM)"
	case syscall.SIGINT:
		return "Interrupt signal (SIGINT) - typically from Ctrl+C"
	case syscall.SIGQUIT:
		return "Quit signal (SIGQUIT) - typically from Ctrl+\\"
	default:
		return sig.String()
	}
}
