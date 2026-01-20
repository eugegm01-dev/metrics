// internal/agent/sender.go
package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
)

func gzipData(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(data); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func SendMetrics(serverAddr string, metrics []models.Metrics) error {
	for _, metric := range metrics {
		jsonData, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		gzData, err := gzipData(jsonData)
		if err != nil {
			return fmt.Errorf("failed to gzip data: %w", err)
		}

		url := "http://" + serverAddr + "/update"
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(gzData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip") // клиент поддерживает gzip

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

// SendMetricsBatch отправляет метрики батчами
func SendMetricsBatch(serverAddr string, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil // не отправляем пустые батчи
	}

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

	url := "http://" + serverAddr + "/updates"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(gzData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	return nil
}
