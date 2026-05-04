// Package app содержит основную логику запуска и graceful shutdown агента и сервера.
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

// RunAgent – точка входа для агента. Загружает конфигурацию, инициализирует
// шифрование (если задан ключ), запускает сбор метрик и их отправку на сервер.
// При получении сигнала SIGINT/SIGTERM/SIGQUIT агент корректно завершает работу,
// отправляя оставшиеся в канале метрики.
func RunAgent() error {
	logger, err := zap.NewDevelopment(zap.AddStacktrace(zap.FatalLevel))
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	cfg, err := config.ParseAgentConfig()
	// Инициализация gRPC клиента
	var grpcClient *agent.GRPCClient
	if cfg.GRPCServerAddr != "" {
		var err error
		grpcClient, err = agent.NewGRPCClient(cfg.GRPCServerAddr)
		if err != nil {
			return fmt.Errorf("grpc client: %w", err)
		}
		defer grpcClient.Close()
		logger.Info("gRPC client connected", zap.String("addr", cfg.GRPCServerAddr))
	}
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

	// Канал для передачи метрик от сборщиков к отправителю.
	// Буферизация позволяет избежать блокировок при пиковых нагрузках.
	metricsChan := make(chan []agent.Metric, 100)

	// Запускаем воркеры агента. Они будут асинхронно отправлять метрики.
	// Внутренняя реализация agent.StartWorkers запускает rateLimit горутин,
	// которые читают из внутреннего канала агента и вызывают переданную функцию.
	agentInstance.StartWorkers(func(metrics []agent.Metric) {
		select {
		case metricsChan <- metrics:
		default:
			logger.Warn("Metrics channel full, dropping batch")
		}
	})

	// Горутина для отправки метрик из канала.
	// Использует контекст для graceful shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case metrics := <-metricsChan:
				sendMetricsBatch(ctx, agentInstance, cfg, metrics, logger, grpcClient)
			case <-ctx.Done():
				// При получении сигнала завершения дренируем канал,
				// чтобы не потерять уже собранные метрики.
				for {
					select {
					case metrics := <-metricsChan:
						sendMetricsBatch(context.Background(), agentInstance, cfg, metrics, logger, grpcClient)
					default:
						return
					}
				}
			}
		}
	}()

	// Горутина сбора runtime метрик (память, GC, горутины и т.д.)
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

	// Горутина сбора системных метрик (CPU, память) через gopsutil
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

	// Даём время на отправку оставшихся метрик
	time.Sleep(2 * time.Second)

	agentInstance.StopWorkers()
	close(metricsChan)

	logger.Info("Agent shutdown completed")
	return nil
}

// sendMetricsBatch подготавливает и отправляет пачку метрик на сервер.
// В случае ошибки логирует её, но не прерывает выполнение программы.
func sendMetricsBatch(ctx context.Context, agentInstance *agent.Agent, cfg *config.AgentConfig, metrics []agent.Metric, logger *zap.Logger, grpcClient *agent.GRPCClient) {
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

	if grpcClient != nil {
		if err := grpcClient.SendBatch(ctx, metrics); err != nil {
			logger.Error("gRPC send failed", zap.Error(err))
			// при желании можешь сделать fallback на HTTP
		} else {
			logger.Debug("gRPC batch sent", zap.Int("count", len(metrics)))
			return
		}
	}
}
