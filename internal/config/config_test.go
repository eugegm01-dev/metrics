package config

import (
	"flag"
	"os"
	"testing"
	"time"
)

func TestParseServerConfig(t *testing.T) {
	// Сохраняем оригинальные аргументы
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Тест 1: Значение по умолчанию
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) // Сбрасываем флаги
	cfg, err := ParseServerConfig()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Addr != "localhost:8080" {
		t.Errorf("Expected default address 'localhost:8080', got '%s'", cfg.Addr)
	}

	// Тест 2: Пользовательское значение
	os.Args = []string{"cmd", "-a", "localhost:9090"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg, err = ParseServerConfig()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.Addr != "localhost:9090" {
		t.Errorf("Expected address 'localhost:9090', got '%s'", cfg.Addr)
	}
}

func TestParseAgentConfig(t *testing.T) {
	// Сохраняем оригинальные аргументы
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Тест 1: Значения по умолчанию
	os.Args = []string{"cmd"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg, err := ParseAgentConfig()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.ServerAddr != "localhost:8080" {
		t.Errorf("Expected default server address 'localhost:8080', got '%s'", cfg.ServerAddr)
	}
	if cfg.ReportInterval != 10*time.Second {
		t.Errorf("Expected default report interval 10s, got %v", cfg.ReportInterval)
	}
	if cfg.PollInterval != 2*time.Second {
		t.Errorf("Expected default poll interval 2s, got %v", cfg.PollInterval)
	}

	// Тест 2: Пользовательские значения
	os.Args = []string{"cmd", "-a", "server:8080", "-r", "5", "-p", "1"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	cfg, err = ParseAgentConfig()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cfg.ServerAddr != "server:8080" {
		t.Errorf("Expected server address 'server:8080', got '%s'", cfg.ServerAddr)
	}
	if cfg.ReportInterval != 5*time.Second {
		t.Errorf("Expected report interval 5s, got %v", cfg.ReportInterval)
	}
	if cfg.PollInterval != 1*time.Second {
		t.Errorf("Expected poll interval 1s, got %v", cfg.PollInterval)
	}
}
