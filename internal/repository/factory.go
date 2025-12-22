package repository

import "time"

// NewStorage создает хранилище в зависимости от параметров
func NewStorage(filePath string, storeInterval time.Duration, restore bool) (Storage, error) {
	if filePath == "" {
		// Если путь к файлу не указан, используем память
		return NewMemStorage(), nil
	}

	// Иначе создаем файловое хранилище
	return NewFileStorage(filePath, storeInterval, restore)
}
