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
	Key             string
	AuditFile       string // путь к файлу аудита
	AuditURL        string // URL для отправки аудита
	MigrationsDir   string // путь к миграциям

}

func ParseServerConfig() (*ServerConfig, error) {
	var flagRunAddr string
	var flagStoreInterval int
	var flagFileStoragePath string
	var flagRestore bool
	var flagDSN string
	var flagKey string
	var flagAuditFile string
	var flagAuditURL string
	var flagMigrationsDir string

	flag.StringVar(&flagDSN, "d", "", "PostgreSQL DSN")
	flag.StringVar(&flagKey, "k", "", "ключ для проверки подписи")
	flag.StringVar(&flagAuditFile, "audit-file", "", "путь к файлу аудита")
	flag.StringVar(&flagAuditURL, "audit-url", "", "URL для отправки логов аудита")
	flag.StringVar(&flagMigrationsDir, "migrations-dir", "migrations", "путь к директории миграций")

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
		AuditFile:       flagAuditFile,
		AuditURL:        flagAuditURL,
		MigrationsDir:   flagMigrationsDir,
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
	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
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
