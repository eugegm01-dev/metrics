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
	var flagServerAddr string
	var flagReportInt int
	var flagPollInt int

	flag.StringVar(&flagServerAddr, "a", "localhost:8080", "адрес и порт HTTP-сервера")
	flag.IntVar(&flagReportInt, "r", 10, "интервал отправки метрик на сервер (секунды)")
	flag.IntVar(&flagPollInt, "p", 2, "интервал опроса метрик из runtime (секунды)")

	flag.Parse()

	cfg := &AgentConfig{
		ServerAddr:     flagServerAddr,
		ReportInterval: time.Duration(flagReportInt) * time.Second,
		PollInterval:   time.Duration(flagPollInt) * time.Second,
	}

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.ServerAddr = envAddr
	}
	// Пример для REPORT_INTERVAL (для POLL_INTERVAL сделайте аналогично):
	if envReport := os.Getenv("REPORT_INTERVAL"); envReport != "" {
		val, err := strconv.Atoi(envReport)
		if err != nil {
			// ЛОГИРУЕМ ошибку, но не падаем. Используем значение по умолчанию.
			log.Printf("CONFIG WARNING: invalid REPORT_INTERVAL value '%s', using default (%ds). Error: %v", envReport, flagReportInt, err)
		} else {
			cfg.ReportInterval = time.Duration(val) * time.Second
		}
	}
	if envPoll := os.Getenv("POLL_INTERVAL"); envPoll != "" {
		val, err := strconv.Atoi(envPoll)
		if err != nil {
			log.Printf("CONFIG WARNING: invalid POLL_INTERVAL value '%s', using default (%ds). Error: %v",
				envPoll, flagPollInt, err) // ← envPoll и flagPollInt
		} else {
			cfg.PollInterval = time.Duration(val) * time.Second // ← PollInterval
		}
	}

	return cfg, nil
}
