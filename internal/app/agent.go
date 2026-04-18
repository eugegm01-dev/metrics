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

	// Initialize crypto
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

	// Create agent instance
	agentInstance := agent.NewAgent(cfg.RateLimit)

	// Create agent wrapper
	a := &Agent{
		instance:       agentInstance,
		cfg:            cfg,
		logger:         logger,
		shutdownChan:   make(chan struct{}),
		metricsChan:    make(chan []agent.Metric, 100),
		workerDoneChan: make(chan struct{}),
	}

	return a.runWithGracefulShutdown()
}

// runWithGracefulShutdown запускает агент с корректной обработкой сигналов
func (a *Agent) runWithGracefulShutdown() error {
	// Start workers
	a.instance.StartWorkers(func(metrics []agent.Metric) {
		select {
		case a.metricsChan <- metrics:
			// Metrics queued for sending
		default:
			a.logger.Warn("Metrics channel full, dropping batch")
		}
	})

	// Start sender goroutine
	go a.sendMetricsLoop()

	// Start metric collection goroutines
	go a.collectRuntimeMetricsLoop()
	go a.collectSystemMetricsLoop()

	// Listen for shutdown signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	// Block until shutdown signal
	sig := <-shutdown
	a.logger.Info("Shutdown signal received",
		zap.String("signal", sig.String()),
		zap.String("description", getAgentSignalDescription(sig)))

	return a.gracefulShutdown()
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
			a.sendMetricsBatch(metrics)
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
		a.sendMetricsBatch(allMetrics)
	}
}

// sendMetricsBatch sends a batch of metrics with retry
func (a *Agent) sendMetricsBatch(metrics []agent.Metric) {
	if len(metrics) == 0 {
		return
	}

	// Prepare metrics
	prepared := a.instance.PrepareMetricsForSend(metrics)
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

	// Send with retry
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := agent.SendMetricsBatchWithContext(ctx, a.cfg.ServerAddr, modelMetrics, a.cfg.Key); err != nil {
		a.logger.Error("Failed to send metrics batch",
			zap.Int("count", len(modelMetrics)),
			zap.Error(err))
	} else {
		a.logger.Debug("Metrics batch sent successfully", zap.Int("count", len(modelMetrics)))
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

// collectSystemMetricsLoop collects system metrics periodically
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
