package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ServerConfig struct {
	Addr            string
	StoreInterval   time.Duration
	FileStoragePath string
	Restore         bool
	DatabaseDSN     string
	Key             string // Добавляем поле для ключа
}

func ParseServerConfig() (*ServerConfig, error) {
	var flagRunAddr string
	var flagStoreInterval int
	var flagFileStoragePath string
	var flagRestore bool
	var flagDSN string
	var flagKey string // Добавляем флаг для ключа

	flag.StringVar(&flagDSN, "d", "", "PostgreSQL DSN")
	flag.StringVar(&flagKey, "k", "", "ключ для проверки подписи") // Добавляем флаг

	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "адрес и порт для запуска сервера")
	flag.IntVar(&flagStoreInterval, "i", 300, "интервал сохранения метрик на диск в секундах (0 - синхронная запись)")
	flag.StringVar(&flagFileStoragePath, "f", "/tmp/metrics-db.json", "путь к файлу для сохранения метрик")
	flag.BoolVar(&flagRestore, "r", true, "загружать сохранённые метрики при старте")

	flag.Parse()

	cfg := &ServerConfig{
		Addr:            flagRunAddr,
		StoreInterval:   time.Duration(flagStoreInterval) * time.Second,
		FileStoragePath: flagFileStoragePath,
		Restore:         flagRestore,
		DatabaseDSN:     flagDSN,
		Key:             flagKey,
	}

	// Приоритет: переменные окружения > флаги > дефолт

	// ADDRESS
	if envAddr, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.Addr = envAddr
	}
	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.DatabaseDSN = envDSN
	}

	// KEY
	if envKey, ok := os.LookupEnv("KEY"); ok {
		cfg.Key = envKey
	}

	// STORE_INTERVAL
	if envStoreInterval, ok := os.LookupEnv("STORE_INTERVAL"); ok {
		val, err := strconv.Atoi(envStoreInterval)
		if err != nil {
			return nil, fmt.Errorf("invalid STORE_INTERVAL value: %q (must be integer), error: %v", envStoreInterval, err)
		}
		cfg.StoreInterval = time.Duration(val) * time.Second
	}

	// FILE_STORAGE_PATH
	if envFilePath, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.FileStoragePath = envFilePath
	}

	// RESTORE
	if envRestore, ok := os.LookupEnv("RESTORE"); ok {
		val, err := strconv.ParseBool(envRestore)
		if err != nil {
			lower := strings.ToLower(envRestore)
			switch lower {
			case "true", "1", "yes":
				cfg.Restore = true
			case "false", "0", "no", "":
				cfg.Restore = false
			default:
				return nil, fmt.Errorf("invalid RESTORE value: %q (must be boolean)", envRestore)
			}
		} else {
			cfg.Restore = val
		}
	}

	return cfg, nil
}
