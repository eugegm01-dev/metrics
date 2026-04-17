package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

// AgentConfig содержит конфигурацию агента.
type AgentConfig struct {
	ServerAddr     string
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      int
	Key            string // поле для ключа
	CryptoKey      string // путь к публичному ключу

}

// ParseAgentConfig читает конфигурацию агента из флагов командной строки и переменных окружения.
// Приоритет: переменные окружения переопределяют флаги.
// AgentConfigFile представляет формат JSON-файла для агента.
type AgentConfigFile struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
	RateLimit      int    `json:"rate_limit"`
	Key            string `json:"key"`
}

// ParseAgentConfig загружает конфигурацию агента с учётом приоритетов.
func ParseAgentConfig() (*AgentConfig, error) {
	var (
		flagServerAddr     string
		flagPollInterval   int
		flagReportInterval int
		flagRateLimit      int
		flagKey            string
		flagCryptoKey      string
		configFile         string
	)

	flag.StringVar(&configFile, "c", "", "config file path")
	flag.StringVar(&configFile, "config", "", "config file path")
	flag.StringVar(&flagServerAddr, "a", "localhost:8080", "адрес и порт сервера")
	flag.IntVar(&flagPollInterval, "p", 2, "интервал опроса метрик в секундах")
	flag.IntVar(&flagReportInterval, "r", 10, "интервал отправки метрик в секундах")
	flag.IntVar(&flagRateLimit, "l", 10, "количество одновременно исходящих запросов")
	flag.StringVar(&flagKey, "k", "", "ключ для подписи запросов")
	flag.StringVar(&flagCryptoKey, "crypto-key", "", "путь к файлу публичного ключа")
	flag.Parse()

	// Базовые значения
	cfg := &AgentConfig{
		ServerAddr:     "localhost:8080",
		PollInterval:   2 * time.Second,
		ReportInterval: 10 * time.Second,
		RateLimit:      10,
		Key:            "",
		CryptoKey:      "",
	}

	// Загрузка из файла
	if configFile == "" {
		if envConfig := os.Getenv("CONFIG"); envConfig != "" {
			configFile = envConfig
		}
	}
	if configFile != "" {
		if err := loadAgentConfigFromFile(configFile, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Переменные окружения
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.ServerAddr = envAddr
	}
	if envPollInterval := os.Getenv("POLL_INTERVAL"); envPollInterval != "" {
		if val, err := strconv.Atoi(envPollInterval); err == nil {
			cfg.PollInterval = time.Duration(val) * time.Second
		}
	}
	if envReportInterval := os.Getenv("REPORT_INTERVAL"); envReportInterval != "" {
		if val, err := strconv.Atoi(envReportInterval); err == nil {
			cfg.ReportInterval = time.Duration(val) * time.Second
		}
	}
	if envRateLimit := os.Getenv("RATE_LIMIT"); envRateLimit != "" {
		if val, err := strconv.Atoi(envRateLimit); err == nil {
			cfg.RateLimit = val
		}
	}
	if envKey := os.Getenv("KEY"); envKey != "" {
		cfg.Key = envKey
	}
	if envCryptoKey := os.Getenv("CRYPTO_KEY"); envCryptoKey != "" {
		cfg.CryptoKey = envCryptoKey
	}

	// Флаги (высший приоритет)
	cfg.ServerAddr = flagServerAddr
	cfg.PollInterval = time.Duration(flagPollInterval) * time.Second
	cfg.ReportInterval = time.Duration(flagReportInterval) * time.Second
	cfg.RateLimit = flagRateLimit
	cfg.Key = flagKey
	cfg.CryptoKey = flagCryptoKey

	return cfg, nil
}

func loadAgentConfigFromFile(path string, cfg *AgentConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	var fileCfg AgentConfigFile
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse config JSON: %w", err)
	}
	if fileCfg.Address != "" {
		cfg.ServerAddr = fileCfg.Address
	}
	if fileCfg.ReportInterval != "" {
		if d, err := time.ParseDuration(fileCfg.ReportInterval); err == nil {
			cfg.ReportInterval = d
		}
	}
	if fileCfg.PollInterval != "" {
		if d, err := time.ParseDuration(fileCfg.PollInterval); err == nil {
			cfg.PollInterval = d
		}
	}
	if fileCfg.CryptoKey != "" {
		cfg.CryptoKey = fileCfg.CryptoKey
	}
	if fileCfg.RateLimit != 0 {
		cfg.RateLimit = fileCfg.RateLimit
	}
	if fileCfg.Key != "" {
		cfg.Key = fileCfg.Key
	}
	return nil
}
