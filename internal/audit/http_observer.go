package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPObserver отправляет события аудита на удалённый сервер
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт наблюдателя для отправки по HTTP
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Update отправляет событие на сервер
func (h *HTTPObserver) Update(event *AuditEvent) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", h.url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("audit endpoint returned status: %d", resp.StatusCode)
	}

	return nil
}

// Close закрывает клиент (не требуется для http.Client)
func (h *HTTPObserver) Close() error {
	return nil
}
