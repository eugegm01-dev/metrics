package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPObserver отправляет события аудита на удалённый HTTP-эндпоинт.
type HTTPObserver struct {
	url    string
	client *http.Client
}

// NewHTTPObserver создаёт HTTPObserver, отправляющий события на указанный URL.
func NewHTTPObserver(url string) *HTTPObserver {
	return &HTTPObserver{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Update отправляет событие аудита через POST-запрос с JSON-телом.
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

// Close – заглушка для HTTPObserver.
func (h *HTTPObserver) Close() error {
	return nil
}
