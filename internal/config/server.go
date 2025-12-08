package config

import (
	"flag"
	"log"
	"os"
)

type ServerConfig struct {
	Addr string
}

func ParseServerConfig() (*ServerConfig, error) {
	var flagRunAddr string

	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "адрес и порт для запуска сервера")

	flag.Parse()

	cfg := &ServerConfig{
		Addr: flagRunAddr,
	}

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		log.Printf("Using server address from environment: %s", envAddr)
		cfg.Addr = envAddr
	}

	return cfg, nil
}
