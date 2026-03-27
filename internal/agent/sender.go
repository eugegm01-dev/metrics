package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	models "github.com/eugegm01-dev/metrics/internal/model"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// retryClient - клиент с автоматическими повторными попытками
var retryClient *retryablehttp.Client

var gzipBufPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 1024))
	},
}

func gzipData(data []byte) ([]byte, error) {
	buf := gzipBufPool.Get().(*bytes.Buffer)
	defer func() {
		buf.Reset()
		gzipBufPool.Put(buf)
	}()
	gw := gzip.NewWriter(buf)
	if _, err := gw.Write(data); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func init() {
	retryClient = retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.HTTPClient = &http.Client{
		Timeout: 10 * time.Second,
	}
}

// computeHash вычисляет HMAC-SHA256 хеш от данных с ключом
func computeHash(data []byte, key string) string {
	if key == "" {
		return ""
	}
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// SendMetricsBatch отправляет несколько метрик одним POST-запросом на эндпоинт /updates.
// Сжимает тело запроса gzip и при необходимости добавляет заголовок хеша.
func SendMetricsBatch(serverAddr string, metrics []models.Metrics, key string) error {
	if len(metrics) == 0 {
		return nil
	}

	jsonData, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	hash := computeHash(jsonData, key)

	gzData, err := gzipData(jsonData)
	if err != nil {
		return fmt.Errorf("failed to gzip data: %w", err)
	}

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

	if hash != "" {
		req.Header.Set("HashSHA256", hash)
	}

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

// SendMetrics отправляет каждую метрику по отдельности через эндпоинт /update.
// Оставлен для обратной совместимости; рекомендуется использовать SendMetricsBatch.
func SendMetrics(serverAddr string, metrics []models.Metrics, key string) error {
	for _, metric := range metrics {
		jsonData, err := json.Marshal(metric)
		if err != nil {
			return fmt.Errorf("failed to marshal metric: %w", err)
		}

		hash := computeHash(jsonData, key)

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

		if hash != "" {
			req.Header.Set("HashSHA256", hash)
		}

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
