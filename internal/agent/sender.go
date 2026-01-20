package agent

import (
	"bytes"
	"compress/gzip"
	"context"
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

// SendMetrics отправляет метрики с поддержкой retry
func SendMetrics(serverAddr string, metrics []models.Metrics) error {
	ctx := context.Background()
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	for _, metric := range metrics {
		err := Retry(ctx, func() error {
			return sendSingleMetric(serverAddr, metric)
		}, 3, delays...)

		if err != nil {
			return fmt.Errorf("failed to send metric %s: %w", metric.ID, err)
		}
	}
	return nil
}

// sendSingleMetric отправляет одну метрику
func sendSingleMetric(serverAddr string, metric models.Metrics) error {
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

// SendMetricsBatch отправляет метрики батчами с поддержкой retry
func SendMetricsBatch(serverAddr string, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	ctx := context.Background()
	delays := []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}

	return Retry(ctx, func() error {
		return sendBatch(serverAddr, metrics)
	}, 3, delays...)
}

// sendBatch отправляет батч метрик
func sendBatch(serverAddr string, metrics []models.Metrics) error {
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
