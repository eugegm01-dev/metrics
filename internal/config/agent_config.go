package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

type AgentConfig struct {
	ServerAddr     string
	PollInterval   time.Duration
	ReportInterval time.Duration
	RateLimit      int
	Key            string // Добавляем поле для ключа
}

func ParseAgentConfig() (*AgentConfig, error) {
	var flagServerAddr string
	var flagPollInterval int
	var flagReportInterval int
	var flagRateLimit int
	var flagKey string // Добавляем флаг для ключа

	flag.StringVar(&flagServerAddr, "a", "localhost:8080", "адрес и порт сервера")
	flag.IntVar(&flagPollInterval, "p", 2, "интервал опроса метрик в секундах")
	flag.IntVar(&flagReportInterval, "r", 10, "интервал отправки метрик в секундах")
	flag.IntVar(&flagRateLimit, "l", 10, "количество одновременно исходящих запросов")
	flag.StringVar(&flagKey, "k", "", "ключ для подписи запросов") // Добавляем флаг

	flag.Parse()

	cfg := &AgentConfig{
		ServerAddr:     flagServerAddr,
		PollInterval:   time.Duration(flagPollInterval) * time.Second,
		ReportInterval: time.Duration(flagReportInterval) * time.Second,
		RateLimit:      flagRateLimit,
		Key:            flagKey,
	}

	if envAddr, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.ServerAddr = envAddr
	}
	if envPollInterval, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		if val, err := strconv.Atoi(envPollInterval); err == nil {
			cfg.PollInterval = time.Duration(val) * time.Second
		}
	}
	if envReportInterval, ok := os.LookupEnv("REPORT_INTERVAL"); ok {
		if val, err := strconv.Atoi(envReportInterval); err == nil {
			cfg.ReportInterval = time.Duration(val) * time.Second
		}
	}
	if envRateLimit, ok := os.LookupEnv("RATE_LIMIT"); ok {
		if val, err := strconv.Atoi(envRateLimit); err == nil {
			cfg.RateLimit = val
		}
	}
	if envKey, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = envKey // Читаем ключ из переменной окружения
	}

	return cfg, nil
}
