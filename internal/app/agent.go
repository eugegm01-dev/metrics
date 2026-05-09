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
	cryptoPkg "github.com/eugegm01-dev/metrics/internal/crypto"
	"github.com/eugegm01-dev/metrics/internal/model"
)

// RunAgent – точка входа для агента.
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

	logger.Info("Agent started",
		zap.String("server", cfg.ServerAddr),
		zap.Duration("poll", cfg.PollInterval),
		zap.Duration("report", cfg.ReportInterval),
		zap.Int("rate_limit", cfg.RateLimit),
	)

	// Инициализация gRPC клиента
	var grpcClient *agent.GRPCClient
	if cfg.GRPCServerAddr != "" {
		grpcClient, err = agent.NewGRPCClient(cfg.GRPCServerAddr)
		if err != nil {
			return fmt.Errorf("grpc client: %w", err)
		}
		defer grpcClient.Close()
		logger.Info("gRPC client connected", zap.String("addr", cfg.GRPCServerAddr))
	}

	// Инициализация HTTP-отправителя
	var httpSender *agent.Sender
	if cfg.CryptoKey != "" {
		pubKey, err := cryptoPkg.LoadPublicKey(cfg.CryptoKey)
		if err != nil {
			return fmt.Errorf("failed to load public key: %w", err)
		}
		httpSender = agent.NewSender(cfg.ServerAddr, cfg.Key, pubKey)
	} else {
		httpSender = agent.NewSender(cfg.ServerAddr, cfg.Key, nil)
	}

	agentInstance := agent.NewAgent(cfg.RateLimit)

	metricsChan := make(chan []agent.Metric, 100)

	agentInstance.StartWorkers(func(metrics []agent.Metric) {
		select {
		case metricsChan <- metrics:
		default:
			logger.Warn("Metrics channel full, dropping batch")
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			select {
			case metrics := <-metricsChan:
				sendMetricsBatch(ctx, agentInstance, cfg, metrics, logger, grpcClient, httpSender)
			case <-ctx.Done():
				for {
					select {
					case metrics := <-metricsChan:
						sendMetricsBatch(context.Background(), agentInstance, cfg, metrics, logger, grpcClient, httpSender)
					default:
						return
					}
				}
			}
		}
	}()

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	<-sigChan

	logger.Info("Shutdown signal received, stopping agent...")
	cancel()
	time.Sleep(2 * time.Second)

	agentInstance.StopWorkers()
	close(metricsChan)

	logger.Info("Agent shutdown completed")
	return nil
}

func sendMetricsBatch(ctx context.Context, agentInstance *agent.Agent, cfg *config.AgentConfig,
	metrics []agent.Metric, logger *zap.Logger, grpcClient *agent.GRPCClient, sender *agent.Sender) {
	if len(metrics) == 0 {
		return
	}
	prepared := agentInstance.PrepareMetricsForSend(metrics)
	if len(prepared) == 0 {
		return
	}

	if grpcClient != nil {
		if err := grpcClient.SendBatch(ctx, prepared); err != nil {
			logger.Error("gRPC send failed", zap.Error(err))
		} else {
			logger.Debug("gRPC batch sent", zap.Int("count", len(prepared)))
		}
	} else {
		modelMetrics := make([]model.Metrics, len(prepared))
		for i, m := range prepared {
			modelMetrics[i] = model.Metrics{
				ID:    m.ID,
				MType: m.MType,
				Delta: &m.Delta,
				Value: &m.Value,
			}
		}
		if err := sender.SendBatch(ctx, modelMetrics); err != nil {
			logger.Error("HTTP batch send failed", zap.Error(err))
		} else {
			logger.Debug("HTTP batch sent", zap.Int("count", len(prepared)))
		}
	}
}
