package config

import (
	"flag"
	"os"
	"strconv"
	"time"
)

// AgentConfig хранит конфигурацию агента
type AgentConfig struct {
	ServerAddr     string        // Адрес сервера (флаг -a)
	ReportInterval time.Duration // Интервал отправки (флаг -r)
	PollInterval   time.Duration // Интервал сбора (флаг -p)
}

// ParseAgentConfig создаёт и заполняет конфиг агента
func ParseAgentConfig() (*AgentConfig, error) {
	// Объявляем переменные для флагов
	var flagServerAddr string
	var flagReportInt int // в секундах
	var flagPollInt int   // в секунды

	// Значения по умолчанию из задания
	flag.StringVar(&flagServerAddr, "a", "localhost:8080", "адрес и порт HTTP-сервера")
	flag.IntVar(&flagReportInt, "r", 10, "интервал отправки метрик на сервер (секунды)")
	flag.IntVar(&flagPollInt, "p", 2, "интервал опроса метрик из runtime (секунды)")

	// Парсим флаги. При неизвестном флаге программа завершится.
	flag.Parse()

	// Создаём конфиг, преобразуя секунды в time.Duration
	cfg := &AgentConfig{
		ServerAddr:     flagServerAddr,
		ReportInterval: time.Duration(flagReportInt) * time.Second,
		PollInterval:   time.Duration(flagPollInt) * time.Second,
	}

	// Переменные окружения для будущих инкрементов
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.ServerAddr = envAddr
	}
	if envReport := os.Getenv("REPORT_INTERVAL"); envReport != "" {
		if val, err := strconv.Atoi(envReport); err == nil {
			cfg.ReportInterval = time.Duration(val) * time.Second
		}
	}
	if envPoll := os.Getenv("POLL_INTERVAL"); envPoll != "" {
		if val, err := strconv.Atoi(envPoll); err == nil {
			cfg.PollInterval = time.Duration(val) * time.Second
		}
	}

	return cfg, nil
}
