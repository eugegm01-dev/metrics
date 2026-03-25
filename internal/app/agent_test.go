package app

import (
	"testing"

	"github.com/eugegm01-dev/metrics/internal/agent"
	"github.com/eugegm01-dev/metrics/internal/config"
)

func TestAgentCreation(t *testing.T) {
	cfg := &config.AgentConfig{RateLimit: 2}
	agentInstance := agent.NewAgent(cfg.RateLimit)
	if agentInstance == nil {
		t.Fatal("agent is nil")
	}
	// запускаем воркеры и останавливаем
	agentInstance.StartWorkers(func(metrics []agent.Metric) {})
	agentInstance.StopWorkers()
}
