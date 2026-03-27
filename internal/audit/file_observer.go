package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

 	"github.com/hashicorp/go-retryablehttp"

)
type HTTPObserver struct {
	url    string
	client *retryablehttp.Client
}
func NewHTTPObserver(url string) *HTTPObserver {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.HTTPClient.Timeout = 10 * time.Second

	return &HTTPObserver{
		url:    url,
		client: retryClient,
	}
}

func (h *HTTPObserver) Update(event *AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}

	req, err := retryablehttp.NewRequest("POST", h.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// FileObserver записывает события аудита в файл в формате JSON-строк.
type FileObserver struct {
	filePath string
	mu       sync.Mutex
	file     *os.File
}

// NewFileObserver создаёт FileObserver, который пишет в указанный файл.
// Файл открывается в режиме добавления, создаётся при необходимости.
func NewFileObserver(filePath string) (*FileObserver, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open audit file %s: %w", filePath, err)
	}
	return &FileObserver{
		filePath: filePath,
		file:     file,
	}, nil
}

// Update записывает событие аудита в файл.
func (f *FileObserver) Update(event *AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	data = append(data, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return nil // уже закрыт
	}
	_, err = f.file.Write(data)
	return err
}

// Close закрывает файл.
func (f *FileObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}