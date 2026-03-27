package audit

import (
	"encoding/json"
	"os"
	"sync"
)

// FileObserver записывает события аудита в файл
type FileObserver struct {
	filePath string
	mu       sync.Mutex
	file     *os.File
}

// NewFileObserver создаёт наблюдателя для записи в файл
func NewFileObserver(filePath string) (*FileObserver, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &FileObserver{
		filePath: filePath,
		file:     file,
	}, nil
}

// Update записывает событие в файл
func (f *FileObserver) Update(event *AuditEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Добавляем перевод строки
	data = append(data, '\n')

	_, err = f.file.Write(data)
	return err
}

// Close закрывает файл
func (f *FileObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file != nil {
		return f.file.Close()
	}
	return nil
}
