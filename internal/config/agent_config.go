// Package config содержит структуры конфигурации и функции для их загрузки
// из различных источников (файл JSON, переменные окружения, флаги командной строки).
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

// AgentConfig содержит все параметры конфигурации агента.
type AgentConfig struct {
	// ServerAddr – адрес HTTP-сервера, на который агент отправляет метрики.
	ServerAddr string

	// PollInterval – интервал сбора метрик (runtime и системных).
	PollInterval time.Duration

	// ReportInterval – интервал отправки собранных метрик на сервер.
	ReportInterval time.Duration

	// RateLimit – максимальное количество одновременно исходящих запросов.
	RateLimit int

	// Key – секретный ключ для подписи запросов с помощью HMAC-SHA256.
	Key string

	// CryptoKey – путь к файлу с публичным RSA-ключом для асимметричного шифрования.
	CryptoKey string
}

// AgentConfigFile представляет структуру JSON-файла конфигурации агента.
// Поля соответствуют ключам в JSON, что позволяет гибко настраивать агента
// без передачи большого количества флагов.
type AgentConfigFile struct {
	Address        string `json:"address"`
	ReportInterval string `json:"report_interval"`
	PollInterval   string `json:"poll_interval"`
	CryptoKey      string `json:"crypto_key"`
	RateLimit      int    `json:"rate_limit"`
	Key            string `json:"key"`
}

// ParseAgentConfig загружает конфигурацию агента, соблюдая следующий приоритет
// (от низшего к высшему):
//  1. Значения по умолчанию
//  2. JSON-файл, указанный через флаг -c/-config или переменную окружения CONFIG
//  3. Переменные окружения (ADDRESS, POLL_INTERVAL, REPORT_INTERVAL, RATE_LIMIT,
//     KEY, CRYPTO_KEY)
//  4. Флаги командной строки, если они были явно заданы (используется flag.Visit)
//
// Такой порядок гарантирует, что наиболее специфичные настройки (флаги)
// переопределяют общие (файл, окружение).
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

	// Базовые значения (низший приоритет)
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

	// Флаги (высший приоритет) – применяются только если были явно заданы.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "a":
			cfg.ServerAddr = flagServerAddr
		case "p":
			cfg.PollInterval = time.Duration(flagPollInterval) * time.Second
		case "r":
			cfg.ReportInterval = time.Duration(flagReportInterval) * time.Second
		case "l":
			cfg.RateLimit = flagRateLimit
		case "k":
			cfg.Key = flagKey
		case "crypto-key":
			cfg.CryptoKey = flagCryptoKey
		}
	})

	return cfg, nil
}

// loadAgentConfigFromFile читает JSON-файл и заполняет поля конфигурации,
// которые ещё не были установлены (т.е. равны значениям по умолчанию).
// Это позволяет комбинировать настройки из файла с параметрами окружения и флагами.
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
