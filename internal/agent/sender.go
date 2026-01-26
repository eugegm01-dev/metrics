package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// retryClient - клиент с автоматическими повторными попытками
var retryClient *retryablehttp.Client

func init() {
	retryClient = retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.HTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}
}

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

// SendMetricsBatch отправляет метрики батчами с использованием go-retryablehttp
func SendMetricsBatch(serverAddr string, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

	// Используем url.JoinPath для безопасной конкатенации URL
	fullURL, err := url.JoinPath("http://"+serverAddr, "/updates")
	if err != nil {
		return fmt.Errorf("failed to build URL: %w", err)
	}

	req, err := retryablehttp.NewRequest("POST", fullURL, bytes.NewBuffer(gzData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	// Используем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request after retries: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	return nil
}

// SendMetrics отправляет одиночные метрики (для обратной совместимости)
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

		fullURL, err := url.JoinPath("http://"+serverAddr, "/update")
		if err != nil {
			return fmt.Errorf("failed to build URL: %w", err)
		}

		req, err := retryablehttp.NewRequest("POST", fullURL, bytes.NewBuffer(gzData))
		if err != nil {
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		req = req.WithContext(ctx)

		resp, err := retryClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send metric %s after retries: %w", metric.ID, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status %d for metric %s", resp.StatusCode, metric.ID)
		}
	}
	return nil
}
