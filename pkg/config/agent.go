package config

import (
	"flag"
	"log"
	"os"
	"strconv"
	"time"
)

type AgentConfig struct {
	ServerAddr     string
	ReportInterval time.Duration
	PollInterval   time.Duration
}

func ParseAgentConfig() (*AgentConfig, error) {
	// 1. Объявление флагов с значениями по умолчанию
	var flagServerAddr string
	var flagReportInt int
	var flagPollInt int

	flag.StringVar(&flagServerAddr, "a", "localhost:8080", "адрес и порт HTTP-сервера")
	flag.IntVar(&flagReportInt, "r", 10, "интервал отправки метрик на сервер (секунды)")
	flag.IntVar(&flagPollInt, "p", 2, "интервал опроса метрик из runtime (секунды)")
	flag.Parse()

	// 2. Инициализация конфига значениями по умолчанию (из флагов)
	cfg := &AgentConfig{
		ServerAddr:     flagServerAddr,
		ReportInterval: time.Duration(flagReportInt) * time.Second,
		PollInterval:   time.Duration(flagPollInt) * time.Second,
	}

	// 3. ПРИОРИТЕТ 1: Переменные окружения (переопределяют флаги)
	// Адрес сервера
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.ServerAddr = envAddr
	}

	// Интервал отчёта
	if envReport := os.Getenv("REPORT_INTERVAL"); envReport != "" {
		val, err := strconv.Atoi(envReport)
		if err != nil {
			log.Printf("CONFIG WARNING: invalid REPORT_INTERVAL '%s', using default (%ds). Error: %v",
				envReport, flagReportInt, err)
		} else {
			cfg.ReportInterval = time.Duration(val) * time.Second
		}
	}

	// Интервал опроса (ДОБАВЛЕНА ОБРАБОТКА)
	if envPoll := os.Getenv("POLL_INTERVAL"); envPoll != "" {
		val, err := strconv.Atoi(envPoll)
		if err != nil {
			// Исправлена опечатка: было envReport, должно быть envPoll
			log.Printf("CONFIG WARNING: invalid POLL_INTERVAL '%s', using default (%ds). Error: %v",
				envPoll, flagPollInt, err)
		} else {
			cfg.PollInterval = time.Duration(val) * time.Second
		}
	}

	return cfg, nil
}
