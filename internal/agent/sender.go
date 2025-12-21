package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

// SendMetrics отправляет метрики на сервер
func SendMetrics(serverAddr string, metrics []models.Metrics) error {
	for _, metric := range metrics {
		jsonData, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		url := "http://" + serverAddr + "/update"
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %d", resp.StatusCode)
		}
	}

	return nil
}
