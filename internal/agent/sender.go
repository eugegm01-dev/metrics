package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	retryablehttp "github.com/hashicorp/go-retryablehttp"

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

// SendMetrics отправляет метрики по одной, используя retryClient.
// Теперь принимает ctx, чтобы можно было отменять при shutdown.
func SendMetrics(ctx context.Context, serverAddr string, metrics []models.Metrics) error {
	for _, metric := range metrics {
		if err := sendSingleMetric(ctx, serverAddr, metric); err != nil {
			return fmt.Errorf("failed to send metric %s: %w", metric.ID, err)
		}
	}
	return nil
}

// sendSingleMetric отправляет одну метрику через retryClient
func sendSingleMetric(ctx context.Context, serverAddr string, metric models.Metrics) error {
	jsonData, err := json.Marshal(metric)
	if err != nil {
		return fmt.Errorf("failed to marshal metric: %w", err)
	}
	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

	base := serverAddr
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("invalid server address %q: %w", serverAddr, err)
	}
	// корректно объединяем путь, учитывая существующий u.Path
	u.Path = path.Join(u.Path, "update")
	req, err := retryablehttp.NewRequest("POST", u.String(), gzData)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	return nil
}

// SendMetricsBatch отправляет метрики батчем через retryClient.
// Принимает ctx для корректного прерывания.
func SendMetricsBatch(ctx context.Context, serverAddr string, metrics []models.Metrics) error {
	if len(metrics) == 0 {
		return nil
	}
	return sendBatch(ctx, serverAddr, metrics)
}

// sendBatch отправляет батч метрик через retryClient
func sendBatch(ctx context.Context, serverAddr string, metrics []models.Metrics) error {
	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

	base := serverAddr
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("invalid server address %q: %w", serverAddr, err)
	}
	u.Path = path.Join(u.Path, "updates")
	req, err := retryablehttp.NewRequest("POST", u.String(), gzData)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req = req.WithContext(ctx)

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	return nil
}
