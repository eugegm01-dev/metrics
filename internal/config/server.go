package config

import (
	"flag"
	"log"
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
}

func ParseServerConfig() (*ServerConfig, error) {
	var flagRunAddr string
	var flagStoreInterval int
	var flagFileStoragePath string
	var flagRestore bool

	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "адрес и порт для запуска сервера")
	flag.IntVar(&flagStoreInterval, "i", 300, "интервал сохранения метрик на диск в секундах (0 - синхронная запись)")
	flag.StringVar(&flagFileStoragePath, "f", "/tmp/metrics-db.json", "путь к файлу для сохранения метрик")
	flag.BoolVar(&flagRestore, "r", true, "загружать сохраненные метрики при старте")

	flag.Parse()

	cfg := &ServerConfig{
		Addr:            flagRunAddr,
		StoreInterval:   time.Duration(flagStoreInterval) * time.Second,
		FileStoragePath: flagFileStoragePath,
		Restore:         flagRestore,
	}

	// Приоритет: переменные окружения > флаги > значения по умолчанию
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		log.Printf("Using server address from environment: %s", envAddr)
		cfg.Addr = envAddr
	}

	if envStoreInterval := os.Getenv("STORE_INTERVAL"); envStoreInterval != "" {
		val, err := strconv.Atoi(envStoreInterval)
		if err != nil {
			log.Printf("CONFIG WARNING: invalid STORE_INTERVAL '%s', using default. Error: %v",
				envStoreInterval, err)
		} else {
			cfg.StoreInterval = time.Duration(val) * time.Second
		}
	}

	if envFilePath := os.Getenv("FILE_STORAGE_PATH"); envFilePath != "" {
		cfg.FileStoragePath = envFilePath
	}

	if envRestore := os.Getenv("RESTORE"); envRestore != "" {
		// Приводим к нижнему регистру для надежности
		envRestoreLower := strings.ToLower(envRestore)
		cfg.Restore = (envRestoreLower == "true" || envRestoreLower == "1" || envRestoreLower == "yes")
	}

	return cfg, nil
}
