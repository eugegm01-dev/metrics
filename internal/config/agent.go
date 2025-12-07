package config

import (
	"flag"
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
