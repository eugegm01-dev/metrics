package config

import (
	"flag"
	"os"
)

// ServerConfig хранит конфигурацию сервера
type ServerConfig struct {
	Addr string // Адрес запуска HTTP-сервера (флаг -a)
}

// ParseServerConfig создаёт и заполняет конфиг сервера
func ParseServerConfig() (*ServerConfig, error) {
	// Объявляем переменные для флагов со значениями по умолчанию
	var flagRunAddr string

	// Регистрируем флаги
	flag.StringVar(&flagRunAddr, "a", "localhost:8080", "адрес и порт для запуска сервера")

	// Парсим флаги. При неизвестном флаге программа завершится с ошибкой.
	flag.Parse()

	// Создаём и возвращаем конфиг
	cfg := &ServerConfig{
		Addr: flagRunAddr,
	}

	// Для совместимости с будущими инкрементами можно добавить переменные окружения:
	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		cfg.Addr = envAddr
	}

	return cfg, nil
}
