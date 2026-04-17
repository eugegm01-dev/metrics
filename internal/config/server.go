// Package config contains the server's and database's cofiguration
package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"
)

// ServerConfig содержит конфигурацию сервера.

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
	CryptoKey       string // путь к приватному ключу

}

// ServerConfigFile представляет формат JSON-файла для сервера.
type ServerConfigFile struct {
	Address       string `json:"address"`
	Restore       *bool  `json:"restore"`
	StoreInterval string `json:"store_interval"`
	StoreFile     string `json:"store_file"`
	DatabaseDSN   string `json:"database_dsn"`
	CryptoKey     string `json:"crypto_key"`
	Key           string `json:"key"`
	AuditFile     string `json:"audit_file"`
	AuditURL      string `json:"audit_url"`
	MigrationsDir string `json:"migrations_dir"`
}

// ParseServerConfig загружает конфигурацию с учётом приоритетов:
// 1. Флаги командной строки (высший приоритет)
// 2. Переменные окружения
// 3. JSON-файл (если задан через -c/-config или CONFIG)
// 4. Значения по умолчанию
func ParseServerConfig() (*ServerConfig, error) {
	// Определяем флаги
	var (
		flagRunAddr         string
		flagStoreInterval   int
		flagFileStoragePath string
		flagRestore         bool
		flagDSN             string
		flagKey             string
		flagAuditFile       string
		flagAuditURL        string
		flagMigrationsDir   string
		flagCryptoKey       string
		configFile          string
	)

	flag.StringVar(&configFile, "c", "", "config file path")
	flag.StringVar(&configFile, "config", "", "config file path")
	flag.StringVar(&flagDSN, "d", "", "PostgreSQL DSN")
	flag.StringVar(&flagKey, "k", "", "ключ для проверки подписи")
	flag.StringVar(&flagAuditFile, "audit-file", "", "путь к файлу аудита")
	flag.StringVar(&flagAuditURL, "audit-url", "", "URL для отправки логов аудита")
	flag.StringVar(&flagMigrationsDir, "migrations-dir", "migrations", "путь к директории миграций")
	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "адрес и порт для запуска сервера")
	flag.IntVar(&flagStoreInterval, "i", 300, "интервал сохранения метрик на диск в секундах (0 - синхронная запись)")
	flag.StringVar(&flagFileStoragePath, "f", "/tmp/metrics-db.json", "путь к файлу для сохранения метрик")
	flag.StringVar(&flagCryptoKey, "crypto-key", "", "путь к файлу приватного ключа")
	flag.BoolVar(&flagRestore, "r", true, "загружать сохранённые метрики при старте")
	flag.Parse()

	// 1. Базовые значения по умолчанию
	cfg := &ServerConfig{
		Addr:            "localhost:8080",
		StoreInterval:   300 * time.Second,
		FileStoragePath: "/tmp/metrics-db.json",
		Restore:         true,
		DatabaseDSN:     "",
		Key:             "",
		AuditFile:       "",
		AuditURL:        "",
		MigrationsDir:   "migrations",
		CryptoKey:       "",
	}

	// 2. Загрузка из JSON-файла (если указан)
	if configFile == "" {
		if envConfig := os.Getenv("CONFIG"); envConfig != "" {
			configFile = envConfig
		}
	}
	if configFile != "" {
		if err := loadServerConfigFromFile(configFile, cfg); err != nil {
			return nil, fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// 3. Переменные окружения (переопределяют файл)
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.Addr = envAddr
	}
	if envDSN := os.Getenv("DATABASE_DSN"); envDSN != "" {
		cfg.DatabaseDSN = envDSN
	}
	if envKey := os.Getenv("KEY"); envKey != "" {
		cfg.Key = envKey
	}
	if envStoreInterval := os.Getenv("STORE_INTERVAL"); envStoreInterval != "" {
		if val, err := strconv.Atoi(envStoreInterval); err == nil {
			cfg.StoreInterval = time.Duration(val) * time.Second
		}
	}
	if envFilePath := os.Getenv("FILE_STORAGE_PATH"); envFilePath != "" {
		cfg.FileStoragePath = envFilePath
	}
	if envAuditFile := os.Getenv("AUDIT_FILE"); envAuditFile != "" {
		cfg.AuditFile = envAuditFile
	}
	if envAuditURL := os.Getenv("AUDIT_URL"); envAuditURL != "" {
		cfg.AuditURL = envAuditURL
	}
	if envRestore := os.Getenv("RESTORE"); envRestore != "" {
		if val, err := strconv.ParseBool(envRestore); err == nil {
			cfg.Restore = val
		}
	}
	if envCryptoKey := os.Getenv("CRYPTO_KEY"); envCryptoKey != "" {
		cfg.CryptoKey = envCryptoKey
	}

	// 4. Флаги командной строки (высший приоритет)
	cfg.Addr = flagRunAddr
	cfg.StoreInterval = time.Duration(flagStoreInterval) * time.Second
	cfg.FileStoragePath = flagFileStoragePath
	cfg.Restore = flagRestore
	cfg.DatabaseDSN = flagDSN
	cfg.Key = flagKey
	cfg.AuditFile = flagAuditFile
	cfg.AuditURL = flagAuditURL
	cfg.MigrationsDir = flagMigrationsDir
	cfg.CryptoKey = flagCryptoKey

	return cfg, nil
}

// loadServerConfigFromFile читает JSON-файл и применяет значения к cfg (не перезаписывая то, что уже установлено в cfg по умолчанию)
func loadServerConfigFromFile(path string, cfg *ServerConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	var fileCfg ServerConfigFile
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parse config JSON: %w", err)
	}
	if fileCfg.Address != "" {
		cfg.Addr = fileCfg.Address
	}
	if fileCfg.Restore != nil {
		cfg.Restore = *fileCfg.Restore
	}
	if fileCfg.StoreInterval != "" {
		if d, err := time.ParseDuration(fileCfg.StoreInterval); err == nil {
			cfg.StoreInterval = d
		}
	}
	if fileCfg.StoreFile != "" {
		cfg.FileStoragePath = fileCfg.StoreFile
	}
	if fileCfg.DatabaseDSN != "" {
		cfg.DatabaseDSN = fileCfg.DatabaseDSN
	}
	if fileCfg.CryptoKey != "" {
		cfg.CryptoKey = fileCfg.CryptoKey
	}
	if fileCfg.Key != "" {
		cfg.Key = fileCfg.Key
	}
	if fileCfg.AuditFile != "" {
		cfg.AuditFile = fileCfg.AuditFile
	}
	if fileCfg.AuditURL != "" {
		cfg.AuditURL = fileCfg.AuditURL
	}
	if fileCfg.MigrationsDir != "" {
		cfg.MigrationsDir = fileCfg.MigrationsDir
	}
	return nil
}
